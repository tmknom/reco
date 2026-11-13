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
func TestUpdateOutcomeOf(t *testing.T) {
	errFailed := errors.New("failed")
	cases := []struct {
		name   string
		status int
		err    error
		want   UpdateOutcome
	}{
		{name: "200 without an error is succeeded", status: 200, err: nil, want: UpdateSucceeded},
		{name: "200 with an error is unknown", status: 200, err: errFailed, want: UpdateUnknown},
		{name: "no response is unknown", status: 0, err: errFailed, want: UpdateUnknown},
		{name: "204 is unknown", status: 204, err: errFailed, want: UpdateUnknown},
		{name: "399 is unknown", status: 399, err: errFailed, want: UpdateUnknown},
		{name: "400 is rejected", status: 400, err: errFailed, want: UpdateRejected},
		{name: "499 is rejected", status: 499, err: errFailed, want: UpdateRejected},
		{name: "500 is unknown", status: 500, err: errFailed, want: UpdateUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := updateOutcomeOf(tc.status, tc.err); got != tc.want {
				t.Errorf("updateOutcomeOf(%d, %v) = %v, want %v", tc.status, tc.err, got, tc.want)
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

			outcome, err := c.UpdateAllowAutoMerge(t.Context(), mustReference(t, "owner/repo"), tc.allow)
			if err != nil || outcome != UpdateSucceeded {
				t.Fatalf("UpdateAllowAutoMerge() = %v, %v, want %v without error", outcome, err, UpdateSucceeded)
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

// TestUpdateDeleteBranchOnMerge は、本文が変更対象の項目だけであることを確かめる。
// 要求の形と結果の判定は UpdateAllowAutoMerge と共有するため、本文だけを確かめる。
//
//trace:checks FR-011
//trace:checks INV-001
func TestUpdateDeleteBranchOnMerge(t *testing.T) {
	cases := []struct {
		name    string
		enabled bool
		want    map[string]any
	}{
		{name: "true sends only delete_branch_on_merge true", enabled: true, want: map[string]any{"delete_branch_on_merge": true}},
		{name: "false sends only delete_branch_on_merge false", enabled: false, want: map[string]any{"delete_branch_on_merge": false}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotBody []byte
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				gotBody, _ = io.ReadAll(r.Body)
				_, _ = w.Write([]byte(repositoryBody))
			})

			outcome, err := c.UpdateDeleteBranchOnMerge(t.Context(), mustReference(t, "owner/repo"), tc.enabled)
			if err != nil || outcome != UpdateSucceeded {
				t.Fatalf("UpdateDeleteBranchOnMerge() = %v, %v, want %v without error", outcome, err, UpdateSucceeded)
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
// ステータスごとの種類は TestClassifyFailure が、拒否と成否不明の境界は TestUpdateOutcomeOf が確かめる。
//
//trace:checks FR-013
//trace:checks INV-005
func TestUpdateAllowAutoMergeRejected(t *testing.T) {
	var received atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusUnprocessableEntity)
	})

	outcome, err := c.UpdateAllowAutoMerge(t.Context(), mustReference(t, "owner/repo"), true)
	if outcome != UpdateRejected || !errors.Is(err, ErrUnprocessable) {
		t.Fatalf("UpdateAllowAutoMerge() = %v, %v, want %v with %v", outcome, err, UpdateRejected, ErrUnprocessable)
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

			outcome, err := c.UpdateAllowAutoMerge(t.Context(), mustReference(t, "owner/repo"), true)
			if outcome != UpdateUnknown || !errors.Is(err, tc.want) {
				t.Fatalf("UpdateAllowAutoMerge() = %v, %v, want %v with %v", outcome, err, UpdateUnknown, tc.want)
			}
			// 成否不明は利用者に示す事実のため、error の文言に含める
			if !strings.Contains(err.Error(), "update outcome unknown") {
				t.Errorf("error = %q, want it to state that the update outcome is unknown", err)
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
