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

	"github.com/tmknom/reco/internal/setting"
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
		{name: "204 without an error is succeeded", status: 204, err: nil, want: UpdateSucceeded},
		{name: "204 with an error is unknown", status: 204, err: errFailed, want: UpdateUnknown},
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
			// send は GetBody を設定しないため、本文付きの 307 には CheckRedirect がなくても net/http が追従しない。追従しないことの検出は 302 が担う
			name: "302 is redirect and is not followed",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, location, http.StatusFound)
			},
			want: ErrRedirect,
		},
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

//trace:checks FR-013
func TestDescribeUpdateOutcome(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		err         error
		want        UpdateOutcome
		wantUnknown bool
	}{
		{name: "403 is rejected without the unknown outcome", status: 403, err: ErrForbidden, want: UpdateRejected},
		{name: "500 is unknown and says so", status: 500, err: ErrUnexpectedResponse, want: UpdateUnknown, wantUnknown: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outcome, err := describeUpdateOutcome("update workflow permissions", mustReference(t, "owner/repo"), tc.status, tc.err)
			if outcome != tc.want || !errors.Is(err, tc.err) {
				t.Fatalf("describeUpdateOutcome() = %v, %v, want %v with %v", outcome, err, tc.want, tc.err)
			}
			if !strings.HasPrefix(err.Error(), "update workflow permissions owner/repo: ") {
				t.Errorf("error = %q, want it to start with the operation and the repository", err)
			}
			if got := strings.Contains(err.Error(), "update outcome unknown"); got != tc.wantUnknown {
				t.Errorf("error = %q, states unknown outcome = %v, want %v", err, got, tc.wantUnknown)
			}
		})
	}

	t.Run("success returns no error", func(t *testing.T) {
		outcome, err := describeUpdateOutcome("update workflow permissions", mustReference(t, "owner/repo"), 204, nil)
		if outcome != UpdateSucceeded || err != nil {
			t.Errorf("describeUpdateOutcome() = %v, %v, want %v without error", outcome, err, UpdateSucceeded)
		}
	})
}

// TestUpdateDefaultWorkflowPermissions は、送信する要求と本文が変更対象の項目だけであり、本文のない 204 を成功とすることを確かめる。
//
//trace:checks FR-011
//trace:checks FR-013
//trace:checks INV-001
func TestUpdateDefaultWorkflowPermissions(t *testing.T) {
	cases := []struct {
		name  string
		level setting.PermissionLevel
		want  map[string]any
	}{
		{name: "read sends only default_workflow_permissions read", level: setting.PermissionRead, want: map[string]any{"default_workflow_permissions": "read"}},
		{name: "write sends only default_workflow_permissions write", level: setting.PermissionWrite, want: map[string]any{"default_workflow_permissions": "write"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod, gotPath, gotContentType string
			var gotBody []byte
			var gotContentLength int64
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				gotMethod, gotPath, gotContentType, gotContentLength = r.Method, r.URL.Path, r.Header.Get("Content-Type"), r.ContentLength
				gotBody, _ = io.ReadAll(r.Body)
				w.WriteHeader(http.StatusNoContent)
			})

			outcome, err := c.UpdateDefaultWorkflowPermissions(t.Context(), mustReference(t, "owner/repo"), tc.level)
			if err != nil || outcome != UpdateSucceeded {
				t.Fatalf("UpdateDefaultWorkflowPermissions() = %v, %v, want %v without error", outcome, err, UpdateSucceeded)
			}

			if gotMethod != http.MethodPut || gotPath != workflowPermissionsPath {
				t.Errorf("request = %s %s, want PUT %s", gotMethod, gotPath, workflowPermissionsPath)
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

// TestUpdateCanApprovePullRequestReviews は、本文が変更対象の項目だけであることを確かめる。
// 要求の形と結果の判定は UpdateDefaultWorkflowPermissions と共有するため、本文だけを確かめる。
//
//trace:checks FR-011
//trace:checks INV-001
func TestUpdateCanApprovePullRequestReviews(t *testing.T) {
	cases := []struct {
		name       string
		canApprove bool
		want       map[string]any
	}{
		{name: "true sends only can_approve_pull_request_reviews true", canApprove: true, want: map[string]any{"can_approve_pull_request_reviews": true}},
		{name: "false sends only can_approve_pull_request_reviews false", canApprove: false, want: map[string]any{"can_approve_pull_request_reviews": false}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotBody []byte
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				gotBody, _ = io.ReadAll(r.Body)
				w.WriteHeader(http.StatusNoContent)
			})

			outcome, err := c.UpdateCanApprovePullRequestReviews(t.Context(), mustReference(t, "owner/repo"), tc.canApprove)
			if err != nil || outcome != UpdateSucceeded {
				t.Fatalf("UpdateCanApprovePullRequestReviews() = %v, %v, want %v without error", outcome, err, UpdateSucceeded)
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

// TestUpdateWorkflowPermissionsRejected は、GitHub が拒否した PUT を失敗として返し、再送しないことを確かめる。
//
//trace:checks FR-013
//trace:checks INV-005
func TestUpdateWorkflowPermissionsRejected(t *testing.T) {
	cases := []struct {
		name   string
		status int
		want   error
	}{
		{name: "403 is rejected as forbidden", status: http.StatusForbidden, want: ErrForbidden},
		{name: "404 is rejected as not found", status: http.StatusNotFound, want: ErrNotFound},
		{name: "409 is rejected as conflict", status: http.StatusConflict, want: ErrConflict},
		{name: "422 is rejected as unprocessable", status: http.StatusUnprocessableEntity, want: ErrUnprocessable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var received atomic.Int32
			c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				received.Add(1)
				w.WriteHeader(tc.status)
			})

			outcome, err := c.UpdateDefaultWorkflowPermissions(t.Context(), mustReference(t, "owner/repo"), setting.PermissionRead)
			if outcome != UpdateRejected || !errors.Is(err, tc.want) {
				t.Fatalf("UpdateDefaultWorkflowPermissions() = %v, %v, want %v with %v", outcome, err, UpdateRejected, tc.want)
			}
			if !strings.Contains(err.Error(), "update workflow permissions owner/repo") {
				t.Errorf("error = %q, want it to name the operation and owner/repo", err)
			}
			if n := received.Load(); n != 1 {
				t.Errorf("server received %d requests, want 1", n)
			}
		})
	}
}

// TestUpdateWorkflowPermissionsOutcomeUnknown は、正常応答の 204 以外で成否を確認できない応答を成否不明として返し、再送せず、
// リダイレクトに追従せず、トークンや応答の内容をエラー文言に含めないことを確かめる。
//
//trace:checks FR-013
//trace:checks INV-005
//trace:checks QR-001
func TestUpdateWorkflowPermissionsOutcomeUnknown(t *testing.T) {
	var redirected atomic.Int32
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		redirected.Add(1)
	}))
	t.Cleanup(redirectTarget.Close)
	location := redirectTarget.URL + "/repositories/1296269"
	const bodyMarker = "body-marker"

	cases := []struct {
		name    string
		handler http.HandlerFunc
		want    error
	}{
		{
			name: "200 is unexpected response",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"default_workflow_permissions":"read","note":"` + bodyMarker + `"}`))
			},
			want: ErrUnexpectedResponse,
		},
		{
			name: "202 is unexpected response",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusAccepted)
			},
			want: ErrUnexpectedResponse,
		},
		{
			// send は GetBody を設定しないため、本文付きの 307 には CheckRedirect がなくても net/http が追従しない。追従しないことの検出は 302 が担う
			name: "302 is redirect and is not followed",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, location, http.StatusFound)
			},
			want: ErrRedirect,
		},
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
			name: "500 is unexpected response",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(bodyMarker))
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

			outcome, err := c.UpdateDefaultWorkflowPermissions(t.Context(), mustReference(t, "owner/repo"), setting.PermissionRead)
			if outcome != UpdateUnknown || !errors.Is(err, tc.want) {
				t.Fatalf("UpdateDefaultWorkflowPermissions() = %v, %v, want %v with %v", outcome, err, UpdateUnknown, tc.want)
			}
			for _, secret := range []string{testToken, bodyMarker, location} {
				if strings.Contains(err.Error(), secret) {
					t.Errorf("error = %q, want it not to contain %q", err, secret)
				}
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
