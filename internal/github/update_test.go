package github

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

//trace:checks FR-013
func TestIsOutcomeUnknown(t *testing.T) {
	cases := []struct {
		name   string
		status int
		want   bool
	}{
		{name: "no response is unknown", status: 0, want: true},
		{name: "204 is unknown", status: 204, want: true},
		{name: "399 is unknown", status: 399, want: true},
		{name: "400 is a rejection", status: 400, want: false},
		{name: "499 is a rejection", status: 499, want: false},
		{name: "500 is unknown", status: 500, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isOutcomeUnknown(tc.status); got != tc.want {
				t.Errorf("isOutcomeUnknown(%d) = %v, want %v", tc.status, got, tc.want)
			}
		})
	}
}

// TestUpdateAllowAutoMerge は、送信する要求と本文が変更対象の項目だけであることを確かめる。
//
//trace:checks FR-011
//trace:checks INV-001
func TestUpdateAllowAutoMerge(t *testing.T) {
	cases := []struct {
		name  string
		allow bool
		want  map[string]any
	}{
		{name: "true sends only allow_auto_merge true", allow: true, want: map[string]any{"allow_auto_merge": true}},
		{name: "false sends only allow_auto_merge false", allow: false, want: map[string]any{"allow_auto_merge": false}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod, gotPath, gotContentType string
			var gotBody []byte
			var gotContentLength int64
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				gotMethod, gotPath, gotContentType, gotContentLength = r.Method, r.URL.Path, r.Header.Get("Content-Type"), r.ContentLength
				gotBody, _ = io.ReadAll(r.Body)
				_, _ = w.Write([]byte(repositoryBody))
			})

			if err := c.UpdateAllowAutoMerge(t.Context(), mustReference(t, "owner/repo"), tc.allow); err != nil {
				t.Fatalf("UpdateAllowAutoMerge() error = %v", err)
			}

			if gotMethod != http.MethodPatch || gotPath != "/repos/owner/repo" {
				t.Errorf("request = %s %s, want PATCH /repos/owner/repo", gotMethod, gotPath)
			}
			if gotContentType != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", gotContentType)
			}
			if gotContentLength != int64(len(gotBody)) {
				t.Errorf("Content-Length = %d, want the body length %d", gotContentLength, len(gotBody))
			}
			var got map[string]any
			if err := json.Unmarshal(gotBody, &got); err != nil {
				t.Fatalf("request body %q is not JSON: %v", gotBody, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("request body = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestUpdateAllowAutoMergeRejected は、GitHub が拒否した更新を成否不明と取り違えず、再送しないことを確かめる。
// ステータスごとの種類は TestClassifyFailure が、拒否と成否不明の境界は TestIsOutcomeUnknown が確かめる。
//
//trace:checks FR-013
//trace:checks INV-005
func TestUpdateAllowAutoMergeRejected(t *testing.T) {
	var received atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusUnprocessableEntity)
	})

	err := c.UpdateAllowAutoMerge(t.Context(), mustReference(t, "owner/repo"), true)
	if !errors.Is(err, ErrUnprocessable) {
		t.Fatalf("UpdateAllowAutoMerge() error = %v, want %v", err, ErrUnprocessable)
	}
	if errors.Is(err, ErrUpdateOutcomeUnknown) {
		t.Errorf("UpdateAllowAutoMerge() error = %v, must not be %v", err, ErrUpdateOutcomeUnknown)
	}
	if n := received.Load(); n != 1 {
		t.Errorf("server received %d requests, want 1", n)
	}
	if !strings.Contains(err.Error(), "owner/repo") {
		t.Errorf("error = %q, want it to name owner/repo", err)
	}
}

// TestUpdateAllowAutoMergeOutcomeUnknown は、成否を確認できない応答を成否不明として返し、再送しないことを確かめる。
//
//trace:checks FR-013
//trace:checks INV-005
func TestUpdateAllowAutoMergeOutcomeUnknown(t *testing.T) {
	var redirected atomic.Int32
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		redirected.Add(1)
	}))
	t.Cleanup(redirectTarget.Close)
	location := redirectTarget.URL + "/repositories/1296269"

	cases := []struct {
		name    string
		handler http.HandlerFunc
		want    error
	}{
		{
			name: "307 is redirect and is not followed",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, location, http.StatusTemporaryRedirect)
			},
			want: ErrRedirect,
		},
		{
			name: "connection closed without a response is communication failure",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				closeConnection(t, w)
			},
			want: ErrCommunicationFailure,
		},
		{
			name: "200 cut off in the body is communication failure",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Length", "100")
				_, _ = w.Write([]byte(`{"id": 1`))
				w.(http.Flusher).Flush()
				closeConnection(t, w)
			},
			want: ErrCommunicationFailure,
		},
		{
			name: "200 without required fields is unexpected response",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{}`))
			},
			want: ErrUnexpectedResponse,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var received atomic.Int32
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				received.Add(1)
				tc.handler(w, r)
			})

			err := c.UpdateAllowAutoMerge(t.Context(), mustReference(t, "owner/repo"), true)
			if !errors.Is(err, ErrUpdateOutcomeUnknown) || !errors.Is(err, tc.want) {
				t.Fatalf("UpdateAllowAutoMerge() error = %v, want %v wrapping %v", err, ErrUpdateOutcomeUnknown, tc.want)
			}
			if n := received.Load(); n != 1 {
				t.Errorf("server received %d requests, want 1", n)
			}
		})
	}
	if n := redirected.Load(); n != 0 {
		t.Errorf("redirect target received %d requests, want 0", n)
	}
}

// closeConnection は応答を書き終えずに接続を閉じ、応答の喪失を作る。
func closeConnection(t *testing.T, w http.ResponseWriter) {
	t.Helper()
	conn, _, err := http.NewResponseController(w).Hijack()
	if err != nil {
		t.Errorf("Hijack() error = %v", err)
		return
	}
	_ = conn.Close()
}
