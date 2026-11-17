package githubtest

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/iotest"
)

// spyTB は、stub が報告した失敗を、実行中のテストを失敗させずに記録する。
// stub の handler はテストと別の goroutine で失敗を報告するため、記録を排他制御する。
type spyTB struct {
	testing.TB
	mu       sync.Mutex
	failures []string
}

func (s *spyTB) Errorf(format string, args ...any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failures = append(s.failures, fmt.Sprintf(format, args...))
}

func (s *spyTB) reported() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.failures)
}

// send は、stub に要求を送り、応答のステータスと本文を返す。Authorization にはパスを入れ、要求ごとに異なる値にする。
func send(t *testing.T, s *Server, method, path, body string) (int, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, s.URL().String()+path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+path)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer func() {
		// 読み取りの済んだ応答の本文で、閉じる失敗は検証の結果に影響しない
		_ = resp.Body.Close()
	}()
	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	return resp.StatusCode, string(got)
}

func TestServer_Requests(t *testing.T) {
	t.Run("requests are recorded with method, path, authorization, and body in the order received", func(t *testing.T) {
		s := NewServer(t, Response{Status: http.StatusNoContent})
		send(t, s, http.MethodGet, "/repos/owner/first", "")
		send(t, s, http.MethodPatch, "/repos/owner/second", `{"allow_auto_merge": true}`)
		send(t, s, http.MethodGet, "/repos/owner/third", "")

		want := []Request{
			{Method: http.MethodGet, Path: "/repos/owner/first", Authorization: "Bearer /repos/owner/first"},
			{Method: http.MethodPatch, Path: "/repos/owner/second", Authorization: "Bearer /repos/owner/second", Body: map[string]any{"allow_auto_merge": true}},
			{Method: http.MethodGet, Path: "/repos/owner/third", Authorization: "Bearer /repos/owner/third"},
		}
		if got := s.Requests(); !reflect.DeepEqual(got, want) {
			t.Errorf("Requests() = %+v, want %+v", got, want)
		}
		if got, want := s.Methods(), []string{"GET", "PATCH", "GET"}; !slices.Equal(got, want) {
			t.Errorf("Methods() = %v, want %v", got, want)
		}
		if got, want := s.RequestLines(), []string{"GET /repos/owner/first", "PATCH /repos/owner/second", "GET /repos/owner/third"}; !slices.Equal(got, want) {
			t.Errorf("RequestLines() = %v, want %v", got, want)
		}
		if got, want := s.Bodies(), []map[string]any{nil, {"allow_auto_merge": true}, nil}; !reflect.DeepEqual(got, want) {
			t.Errorf("Bodies() = %v, want %v", got, want)
		}
	})

	t.Run("request body that is not JSON is reported as a test failure", func(t *testing.T) {
		spy := &spyTB{TB: t}
		s := NewServer(spy, Response{Status: http.StatusNoContent})
		send(t, s, http.MethodPatch, "/repos/owner/repo", `not json`)
		if got := spy.reported(); len(got) != 1 || !strings.HasPrefix(got[0], "decode request body") {
			t.Errorf("reported failures = %q, want one decode failure", got)
		}
	})

	t.Run("request body read failure is reported as a test failure", func(t *testing.T) {
		spy := &spyTB{TB: t}
		s := NewServer(spy, Response{Status: http.StatusNoContent})
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPatch, "/repos/owner/repo", iotest.ErrReader(errors.New("read failed")))
		s.serve(httptest.NewRecorder(), req)
		if got := spy.reported(); len(got) != 1 || !strings.HasPrefix(got[0], "read request body") {
			t.Errorf("reported failures = %q, want one read failure", got)
		}
	})
}

func TestRoutes(t *testing.T) {
	routes := map[string]Response{
		"GET /repos/owner/repo": OK("repository"),
		"PUT /repos/owner/repo": {Status: http.StatusNoContent},
	}

	t.Run("registered methods of the same path get their own responses without a failure", func(t *testing.T) {
		spy := &spyTB{TB: t}
		s := NewServer(spy, Routes(spy, routes))
		if status, body := send(t, s, http.MethodGet, "/repos/owner/repo", ""); status != http.StatusOK || body != "repository" {
			t.Errorf("GET got %d %q, want 200 repository", status, body)
		}
		if status, _ := send(t, s, http.MethodPut, "/repos/owner/repo", ""); status != http.StatusNoContent {
			t.Errorf("PUT got %d, want 204", status)
		}
		if got := spy.reported(); len(got) != 0 {
			t.Errorf("reported failures = %q, want none", got)
		}
	})

	unregistered := []struct {
		name   string
		method string
		path   string
	}{
		{name: "unregistered path returns 500 and is reported as a test failure", method: http.MethodGet, path: "/repos/owner/other"},
		{name: "unregistered method of a registered path returns 500 and is reported as a test failure", method: http.MethodPatch, path: "/repos/owner/repo"},
	}
	for _, tc := range unregistered {
		t.Run(tc.name, func(t *testing.T) {
			spy := &spyTB{TB: t}
			s := NewServer(spy, Routes(spy, routes))
			if status, _ := send(t, s, tc.method, tc.path, ""); status != http.StatusInternalServerError {
				t.Errorf("got %d, want 500", status)
			}
			if got := spy.reported(); len(got) != 1 || !strings.Contains(got[0], tc.method+" "+tc.path) {
				t.Errorf("reported failures = %q, want one naming the request", got)
			}
		})
	}
}

func TestSequence(t *testing.T) {
	t.Run("responses are returned in the order of requests without a failure", func(t *testing.T) {
		spy := &spyTB{TB: t}
		s := NewServer(spy, Sequence(spy, OK("first"), OK("second")))
		for _, want := range []string{"first", "second"} {
			if _, body := send(t, s, http.MethodGet, "/repos/owner/repo", ""); body != want {
				t.Errorf("got %q, want %q", body, want)
			}
		}
		if got := spy.reported(); len(got) != 0 {
			t.Errorf("reported failures = %q, want none", got)
		}
	})

	t.Run("request beyond the responses returns 500 and is reported as a test failure", func(t *testing.T) {
		spy := &spyTB{TB: t}
		s := NewServer(spy, Sequence(spy, OK("first")))
		send(t, s, http.MethodGet, "/repos/owner/repo", "")
		if status, _ := send(t, s, http.MethodGet, "/repos/owner/repo", ""); status != http.StatusInternalServerError {
			t.Errorf("got %d, want 500", status)
		}
		if got := spy.reported(); len(got) != 1 || !strings.Contains(got[0], "GET /repos/owner/repo") {
			t.Errorf("reported failures = %q, want one naming the request", got)
		}
	})
}
