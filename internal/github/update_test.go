package github

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/tmknom/reco/internal/githubtest"
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

// TestClientUpdate は、更新操作が変える設定から、その更新要求の送り先と本文を選ぶ規則を確かめる。
//
//trace:checks INV-001 FR-011
func TestClientUpdate(t *testing.T) {
	cases := []struct {
		name        string
		key         setting.Key
		value       setting.Value
		wantRequest string
		wantBody    map[string]any
	}{
		{
			name: "delete_branch_on_merge sends only delete_branch_on_merge", key: setting.DeleteBranchOnMerge, value: setting.BoolValue(true),
			wantRequest: "PATCH /repos/owner/repo", wantBody: map[string]any{"delete_branch_on_merge": true},
		},
		{
			name: "allow_auto_merge sends only allow_auto_merge", key: setting.AllowAutoMerge, value: setting.BoolValue(true),
			wantRequest: "PATCH /repos/owner/repo", wantBody: map[string]any{"allow_auto_merge": true},
		},
		{
			name: "enabled sends only enabled", key: setting.ActionsEnabled, value: setting.BoolValue(false),
			wantRequest: "PUT /repos/owner/repo/actions/permissions", wantBody: map[string]any{"enabled": false},
		},
		{
			name: "allowed_actions sends enabled and allowed_actions only", key: setting.AllowedActions,
			value:       setting.AllowedActionsPolicyValue(setting.AllowedActionsLocalOnly),
			wantRequest: "PUT /repos/owner/repo/actions/permissions", wantBody: map[string]any{"enabled": true, "allowed_actions": "local_only"},
		},
		{
			name: "sha_pinning_required sends enabled true and sha_pinning_required only", key: setting.SHAPinningRequired,
			value:       setting.BoolValue(true),
			wantRequest: "PUT /repos/owner/repo/actions/permissions", wantBody: map[string]any{"enabled": true, "sha_pinning_required": true},
		},
		{
			name: "selected_actions sends all three items", key: setting.SelectedActions,
			value:       setting.SelectedActionsCriteriaValue(setting.SelectedActionsCriteria{VerifiedAllowed: true, PatternsAllowed: []string{"b/*", "a/*"}}),
			wantRequest: "PUT /repos/owner/repo/actions/permissions/selected-actions",
			wantBody:    map[string]any{"github_owned_allowed": false, "verified_allowed": true, "patterns_allowed": []any{"b/*", "a/*"}},
		},
		{
			name: "default_workflow_permissions sends only default_workflow_permissions", key: setting.DefaultWorkflowPermissions,
			value:       setting.PermissionLevelValue(setting.PermissionRead),
			wantRequest: "PUT /repos/owner/repo/actions/permissions/workflow", wantBody: map[string]any{"default_workflow_permissions": "read"},
		},
		{
			name: "can_approve_pull_request_reviews sends only can_approve_pull_request_reviews", key: setting.CanApprovePullRequestReviews,
			value:       setting.BoolValue(false),
			wantRequest: "PUT /repos/owner/repo/actions/permissions/workflow", wantBody: map[string]any{"can_approve_pull_request_reviews": false},
		},
		{
			name: "access_level sends only access_level", key: setting.ActionsAccessLevel,
			value:       setting.AccessLevelValue(setting.AccessUser),
			wantRequest: "PUT /repos/owner/repo/actions/permissions/access", wantBody: map[string]any{"access_level": "user"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var requests []string
			var bodies []map[string]any
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				requests = append(requests, r.Method+" "+r.URL.Path)
				data, _ := io.ReadAll(r.Body)
				var body map[string]any
				if err := json.Unmarshal(data, &body); err != nil {
					t.Errorf("request body %q is not JSON: %v", data, err)
				}
				bodies = append(bodies, body)
				if r.Method == http.MethodPatch {
					_, _ = w.Write([]byte(repositoryBody))
					return
				}
				w.WriteHeader(http.StatusNoContent)
			})

			if outcome, err := c.Update(t.Context(), mustReference(t, "owner/repo"), tc.key, tc.value); err != nil || outcome != UpdateSucceeded {
				t.Fatalf("Update() = %v, %v, want %v without error", outcome, err, UpdateSucceeded)
			}

			if want := []string{tc.wantRequest}; !slices.Equal(requests, want) {
				t.Errorf("requests = %v, want %v", requests, want)
			}
			if want := []map[string]any{tc.wantBody}; !reflect.DeepEqual(bodies, want) {
				t.Errorf("request bodies = %v, want %v", bodies, want)
			}
		})
	}

	t.Run("undefined setting panics before sending a request", func(t *testing.T) {
		undefined := setting.Key("general.undefined")
		defer func() {
			if recover() == nil {
				t.Errorf("Update(%q) did not panic", undefined)
			}
		}()
		c := newTestClient(t, func(http.ResponseWriter, *http.Request) {
			t.Error("Update() sent a request for an undefined setting")
		})
		_, _ = c.Update(t.Context(), mustReference(t, "owner/repo"), undefined, setting.BoolValue(true))
	})
}

// TestUpdateAllowAutoMerge は、送信する要求と本文が変更対象の項目だけであることを確かめる。
//
//trace:checks FR-011 INV-001
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

			outcome, err := c.updateAllowAutoMerge(t.Context(), mustReference(t, "owner/repo"), tc.allow)
			if err != nil || outcome != UpdateSucceeded {
				t.Fatalf("updateAllowAutoMerge() = %v, %v, want %v without error", outcome, err, UpdateSucceeded)
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
// 要求の形と結果の判定は updateAllowAutoMerge と共有するため、本文だけを確かめる。
//
//trace:checks FR-011 INV-001
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

			outcome, err := c.updateDeleteBranchOnMerge(t.Context(), mustReference(t, "owner/repo"), tc.enabled)
			if err != nil || outcome != UpdateSucceeded {
				t.Fatalf("updateDeleteBranchOnMerge() = %v, %v, want %v without error", outcome, err, UpdateSucceeded)
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
//trace:checks FR-013 INV-005
func TestUpdateAllowAutoMergeRejected(t *testing.T) {
	var received atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusUnprocessableEntity)
	})

	outcome, err := c.updateAllowAutoMerge(t.Context(), mustReference(t, "owner/repo"), true)
	if outcome != UpdateRejected || !errors.Is(err, ErrUnprocessable) {
		t.Fatalf("updateAllowAutoMerge() = %v, %v, want %v with %v", outcome, err, UpdateRejected, ErrUnprocessable)
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
//trace:checks FR-013 INV-005
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

			outcome, err := c.updateAllowAutoMerge(t.Context(), mustReference(t, "owner/repo"), true)
			if outcome != UpdateUnknown || !errors.Is(err, tc.want) {
				t.Fatalf("updateAllowAutoMerge() = %v, %v, want %v with %v", outcome, err, UpdateUnknown, tc.want)
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
//trace:checks FR-011 FR-013 INV-001
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

			outcome, err := c.updateDefaultWorkflowPermissions(t.Context(), mustReference(t, "owner/repo"), tc.level)
			if err != nil || outcome != UpdateSucceeded {
				t.Fatalf("updateDefaultWorkflowPermissions() = %v, %v, want %v without error", outcome, err, UpdateSucceeded)
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
// 要求の形と結果の判定は updateDefaultWorkflowPermissions と共有するため、本文だけを確かめる。
//
//trace:checks FR-011 INV-001
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

			outcome, err := c.updateCanApprovePullRequestReviews(t.Context(), mustReference(t, "owner/repo"), tc.canApprove)
			if err != nil || outcome != UpdateSucceeded {
				t.Fatalf("updateCanApprovePullRequestReviews() = %v, %v, want %v without error", outcome, err, UpdateSucceeded)
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
//trace:checks FR-013 INV-005
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

			outcome, err := c.updateDefaultWorkflowPermissions(t.Context(), mustReference(t, "owner/repo"), setting.PermissionRead)
			if outcome != UpdateRejected || !errors.Is(err, tc.want) {
				t.Fatalf("updateDefaultWorkflowPermissions() = %v, %v, want %v with %v", outcome, err, UpdateRejected, tc.want)
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
//trace:checks FR-013 INV-005 QR-001
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

			outcome, err := c.updateDefaultWorkflowPermissions(t.Context(), mustReference(t, "owner/repo"), setting.PermissionRead)
			if outcome != UpdateUnknown || !errors.Is(err, tc.want) {
				t.Fatalf("updateDefaultWorkflowPermissions() = %v, %v, want %v with %v", outcome, err, UpdateUnknown, tc.want)
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

// TestUpdateActionsEnabled は、本文が enabled だけで、allowed_actions と sha_pinning_required を送らないことを確かめる。
// 要求の見出しと送り先は、同じ更新エンドポイントの TestUpdateAllowedActions と TestClientUpdate が確かめる。
//
//trace:checks FR-011 INV-001
func TestUpdateActionsEnabled(t *testing.T) {
	cases := []struct {
		name    string
		enabled bool
		want    map[string]any
	}{
		{name: "true sends only enabled true", enabled: true, want: map[string]any{"enabled": true}},
		{name: "false sends only enabled false", enabled: false, want: map[string]any{"enabled": false}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotBody []byte
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				gotBody, _ = io.ReadAll(r.Body)
				w.WriteHeader(http.StatusNoContent)
			})

			outcome, err := c.updateActionsEnabled(t.Context(), mustReference(t, "owner/repo"), tc.enabled)
			if err != nil || outcome != UpdateSucceeded {
				t.Fatalf("updateActionsEnabled() = %v, %v, want %v without error", outcome, err, UpdateSucceeded)
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

// TestUpdateSHAPinningRequired は、false を送るときも enabled が true のままであることを確かめる。
// true を送る形は TestClientUpdate が確かめる。
//
//trace:checks FR-011 INV-001
func TestUpdateSHAPinningRequired(t *testing.T) {
	var gotBody []byte
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	})

	outcome, err := c.updateSHAPinningRequired(t.Context(), mustReference(t, "owner/repo"), false)
	if err != nil || outcome != UpdateSucceeded {
		t.Fatalf("updateSHAPinningRequired() = %v, %v, want %v without error", outcome, err, UpdateSucceeded)
	}

	var got map[string]any
	if err := json.Unmarshal(gotBody, &got); err != nil {
		t.Fatalf("request body %q is not JSON: %v", gotBody, err)
	}
	if want := map[string]any{"enabled": true, "sha_pinning_required": false}; !reflect.DeepEqual(got, want) {
		t.Errorf("request body = %v, want %v", got, want)
	}
}

// TestUpdateAllowedActions は、要求の形と、本文が enabled: true と allowed_actions だけであることを確かめる。
//
//trace:checks FR-011 FR-013 INV-001
func TestUpdateAllowedActions(t *testing.T) {
	cases := []struct {
		name   string
		policy setting.AllowedActionsPolicy
		want   map[string]any
	}{
		{name: "all sends only enabled true and allowed_actions all", policy: setting.AllowedActionsAll, want: map[string]any{"enabled": true, "allowed_actions": "all"}},
		{name: "local_only sends only enabled true and allowed_actions local_only", policy: setting.AllowedActionsLocalOnly, want: map[string]any{"enabled": true, "allowed_actions": "local_only"}},
		{name: "selected sends only enabled true and allowed_actions selected", policy: setting.AllowedActionsSelected, want: map[string]any{"enabled": true, "allowed_actions": "selected"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod, gotPath string
			var gotHeader http.Header
			var gotBody []byte
			var gotContentLength int64
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				gotMethod, gotPath, gotHeader, gotContentLength = r.Method, r.URL.Path, r.Header, r.ContentLength
				gotBody, _ = io.ReadAll(r.Body)
				w.WriteHeader(http.StatusNoContent)
			})

			outcome, err := c.updateAllowedActions(t.Context(), mustReference(t, "owner/repo"), tc.policy)
			if err != nil || outcome != UpdateSucceeded {
				t.Fatalf("updateAllowedActions() = %v, %v, want %v without error", outcome, err, UpdateSucceeded)
			}

			if gotMethod != http.MethodPut || gotPath != actionsPermissionsPath {
				t.Errorf("request = %s %s, want PUT %s", gotMethod, gotPath, actionsPermissionsPath)
			}
			headers := map[string]string{
				"Authorization":        "Bearer " + testToken,
				"X-GitHub-Api-Version": "2026-03-10",
				"Content-Type":         "application/json",
			}
			for name, value := range headers {
				if v := gotHeader.Get(name); v != value {
					t.Errorf("header %s = %q, want %q", name, v, value)
				}
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

// TestUpdateAllowedActionsRejected は、GitHub が拒否した更新を失敗として返し、error がこの更新操作と対象を示し、再送しないことを確かめる。
// ステータスごとの種類は TestClassifyFailure が、失敗と成否不明の境界は TestUpdateOutcomeOf が、
// リダイレクトと応答の喪失の扱いは TestUpdateWorkflowPermissionsOutcomeUnknown が確かめる。
//
//trace:checks FR-013 INV-005
func TestUpdateAllowedActionsRejected(t *testing.T) {
	var received atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusForbidden)
	})

	outcome, err := c.updateAllowedActions(t.Context(), mustReference(t, "owner/repo"), setting.AllowedActionsAll)
	if outcome != UpdateRejected || !errors.Is(err, ErrForbidden) {
		t.Fatalf("updateAllowedActions() = %v, %v, want %v with %v", outcome, err, UpdateRejected, ErrForbidden)
	}
	if !strings.HasPrefix(err.Error(), "update actions permissions owner/repo: ") {
		t.Errorf("error = %q, want it to start with the operation and owner/repo", err)
	}
	if n := received.Load(); n != 1 {
		t.Errorf("server received %d requests, want 1", n)
	}
}

// TestUpdateAccessLevelRejected は、GitHub が拒否した更新要求を拒否と判定し、操作と対象を error に添えることを確かめる。
//
//trace:checks FR-013
func TestUpdateAccessLevelRejected(t *testing.T) {
	var received atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusUnprocessableEntity)
	})

	outcome, err := c.updateAccessLevel(t.Context(), mustReference(t, "owner/repo"), setting.AccessNone)
	if outcome != UpdateRejected || !errors.Is(err, ErrUnprocessable) {
		t.Fatalf("updateAccessLevel() = %v, %v, want %v with %v", outcome, err, UpdateRejected, ErrUnprocessable)
	}
	if !strings.HasPrefix(err.Error(), "update actions access owner/repo: ") {
		t.Errorf("error = %q, want it to start with the operation and owner/repo", err)
	}
	if n := received.Load(); n != 1 {
		t.Errorf("server received %d requests, want 1", n)
	}
}

// TestEncodeSelectedActions は、Selected actions の本文が3項目をすべて含み、許可パターンを受け取った順のまま JSON の規則で送ることを確かめる。
//
//trace:checks FR-011 INV-001
func TestEncodeSelectedActions(t *testing.T) {
	cases := []struct {
		name     string
		criteria setting.SelectedActionsCriteria
		want     map[string]any
	}{
		{
			name:     "three items are sent with patterns in the given order",
			criteria: setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, VerifiedAllowed: false, PatternsAllowed: []string{"z/*", "a/*"}},
			want:     map[string]any{"github_owned_allowed": true, "verified_allowed": false, "patterns_allowed": []any{"z/*", "a/*"}},
		},
		{
			name:     "false items and empty patterns are not omitted",
			criteria: setting.SelectedActionsCriteria{GitHubOwnedAllowed: false, VerifiedAllowed: false, PatternsAllowed: []string{}},
			want:     map[string]any{"github_owned_allowed": false, "verified_allowed": false, "patterns_allowed": []any{}},
		},
		{
			name:     "nil patterns are sent as an empty array",
			criteria: setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, VerifiedAllowed: true, PatternsAllowed: nil},
			want:     map[string]any{"github_owned_allowed": true, "verified_allowed": true, "patterns_allowed": []any{}},
		},
		{
			// 引用符、バックスラッシュ、ASCII 以外の文字は Go の %q でも JSON として正しく戻る。%q との違いを検出するのは制御文字である
			name:     "patterns are escaped by the JSON rules",
			criteria: setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, VerifiedAllowed: true, PatternsAllowed: []string{"a\"b\\c", "a\x01b", "日本語/*"}},
			want:     map[string]any{"github_owned_allowed": true, "verified_allowed": true, "patterns_allowed": []any{"a\"b\\c", "a\x01b", "日本語/*"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := encodeSelectedActions(tc.criteria)
			var got map[string]any
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("encodeSelectedActions() = %q, not JSON: %v", body, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("encodeSelectedActions() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMustMarshalJSONPanicsOnUnsupportedType(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("mustMarshalJSON() did not panic for a channel")
		}
	}()
	_ = mustMarshalJSON(make(chan int))
}

// TestUpdateSelectedActions は、要求の形と、本文に encodeSelectedActions の結果を送ることを確かめる。本文の規則は TestEncodeSelectedActions が確かめる。
//
//trace:checks FR-011 FR-013
func TestUpdateSelectedActions(t *testing.T) {
	var gotMethod, gotPath string
	var gotHeader http.Header
	var gotBody []byte
	var gotContentLength int64
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotHeader, gotContentLength = r.Method, r.URL.Path, r.Header, r.ContentLength
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	})

	criteria := setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, VerifiedAllowed: false, PatternsAllowed: []string{"z/*", "a/*"}}
	outcome, err := c.updateSelectedActions(t.Context(), mustReference(t, "owner/repo"), criteria)
	if err != nil || outcome != UpdateSucceeded {
		t.Fatalf("updateSelectedActions() = %v, %v, want %v without error", outcome, err, UpdateSucceeded)
	}

	if gotMethod != http.MethodPut || gotPath != selectedActionsPath {
		t.Errorf("request = %s %s, want PUT %s", gotMethod, gotPath, selectedActionsPath)
	}
	headers := map[string]string{
		"Authorization":        "Bearer " + testToken,
		"X-GitHub-Api-Version": "2026-03-10",
		"Content-Type":         "application/json",
	}
	for name, value := range headers {
		if v := gotHeader.Get(name); v != value {
			t.Errorf("header %s = %q, want %q", name, v, value)
		}
	}
	if gotContentLength != int64(len(gotBody)) {
		t.Errorf("Content-Length = %d, want the body length %d", gotContentLength, len(gotBody))
	}
	var got map[string]any
	if err := json.Unmarshal(gotBody, &got); err != nil {
		t.Fatalf("request body %q is not JSON: %v", gotBody, err)
	}
	want := map[string]any{"github_owned_allowed": true, "verified_allowed": false, "patterns_allowed": []any{"z/*", "a/*"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("request body = %v, want %v", got, want)
	}
}

// TestUpdateSelectedActionsRejected は、GitHub が競合として拒否した更新を失敗として返し、error がこの更新操作と対象を示し、再送しないことを確かめる。
//
//trace:checks FR-013 INV-005
func TestUpdateSelectedActionsRejected(t *testing.T) {
	var received atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusConflict)
	})

	outcome, err := c.updateSelectedActions(t.Context(), mustReference(t, "owner/repo"), setting.SelectedActionsCriteria{GitHubOwnedAllowed: true})
	if outcome != UpdateRejected || !errors.Is(err, ErrConflict) {
		t.Fatalf("updateSelectedActions() = %v, %v, want %v with %v", outcome, err, UpdateRejected, ErrConflict)
	}
	if !strings.HasPrefix(err.Error(), "update selected actions owner/repo: ") {
		t.Errorf("error = %q, want it to start with the operation and owner/repo", err)
	}
	if n := received.Load(); n != 1 {
		t.Errorf("server received %d requests, want 1", n)
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

// TestCreateEnvironment は、本文なしの PUT を Environment の名前を1要素にした宛先へ一度だけ送ることを確かめる。
//
//trace:checks INV-001 FR-011
func TestCreateEnvironment(t *testing.T) {
	cases := []struct {
		name            string
		environmentName string
		wantPath        string
	}{
		{name: "plain name", environmentName: "production", wantPath: "/repos/owner/repo/environments/production"},
		{name: "slash stays one path element", environmentName: "a/b", wantPath: "/repos/owner/repo/environments/a%2Fb"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var received atomic.Int32
			var gotMethod, gotPath, gotRawQuery string
			var gotHeader http.Header
			var gotTransferEncoding []string
			var gotContentLength int64
			var gotBody []byte
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				received.Add(1)
				gotMethod, gotPath, gotRawQuery = r.Method, r.URL.EscapedPath(), r.URL.RawQuery
				gotHeader, gotTransferEncoding, gotContentLength = r.Header, r.TransferEncoding, r.ContentLength
				gotBody, _ = io.ReadAll(r.Body)
				_, _ = w.Write([]byte(githubtest.EnvironmentBody{ID: 7, Name: tc.environmentName}.JSON()))
			})

			outcome, err := c.CreateEnvironment(t.Context(), mustReference(t, "owner/repo"), tc.environmentName)
			if err != nil || outcome != UpdateSucceeded {
				t.Fatalf("CreateEnvironment(%q) = %v, %v, want %v without error", tc.environmentName, outcome, err, UpdateSucceeded)
			}

			if n := received.Load(); n != 1 {
				t.Errorf("server received %d requests, want 1", n)
			}
			if gotMethod != http.MethodPut || gotPath != tc.wantPath || gotRawQuery != "" {
				t.Errorf("request = %s %s?%s, want PUT %s without a query", gotMethod, gotPath, gotRawQuery, tc.wantPath)
			}
			if len(gotBody) != 0 {
				t.Errorf("request body = %q, want none", gotBody)
			}
			if v, ok := gotHeader["Content-Type"]; ok {
				t.Errorf("Content-Type = %q, want the header absent", v)
			}
			if gotContentLength != 0 || len(gotTransferEncoding) != 0 {
				t.Errorf("Content-Length = %d, Transfer-Encoding = %v, want 0 and none", gotContentLength, gotTransferEncoding)
			}
		})
	}
}

// TestCreateEnvironmentPanicsOnUnaddressableName は、URL を組めない名前の更新を、要求を送らずに不変条件の破れとして panic することを確かめる。
//
//trace:checks INV-001 FR-011
func TestCreateEnvironmentPanicsOnUnaddressableName(t *testing.T) {
	var received atomic.Int32
	c := newTestClient(t, func(http.ResponseWriter, *http.Request) { received.Add(1) })
	reference := mustReference(t, "owner/repo")
	cases := []struct {
		name   string
		update func()
	}{
		{name: "create", update: func() { _, _ = c.CreateEnvironment(t.Context(), reference, "..") }},
		{name: "deployment branch policy", update: func() {
			_, _ = c.UpdateEnvironmentDeploymentBranchPolicy(t.Context(), reference, "..", setting.DeploymentBranchPolicyAll)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("update with the name \"..\" did not panic")
				}
			}()
			tc.update()
		})
	}
	if n := received.Load(); n != 0 {
		t.Errorf("server received %d requests, want 0", n)
	}
}

// TestUpdateEnvironmentDeploymentBranchPolicy は、本文が deployment_branch_policy のキーだけで、方式の値を GitHub の形にすることを確かめる。
//
//trace:checks INV-001 FR-011
func TestUpdateEnvironmentDeploymentBranchPolicy(t *testing.T) {
	cases := []struct {
		name string
		mode setting.DeploymentBranchPolicyMode
		want map[string]any
	}{
		{name: "all sends null", mode: setting.DeploymentBranchPolicyAll, want: map[string]any{"deployment_branch_policy": nil}},
		{name: "protected_branches sends protected_branches true and custom_branch_policies false", mode: setting.DeploymentBranchPolicyProtectedBranches, want: map[string]any{"deployment_branch_policy": map[string]any{"protected_branches": true, "custom_branch_policies": false}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var received atomic.Int32
			var gotMethod, gotPath, gotContentType string
			var gotBody []byte
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				received.Add(1)
				gotMethod, gotPath, gotContentType = r.Method, r.URL.EscapedPath(), r.Header.Get("Content-Type")
				gotBody, _ = io.ReadAll(r.Body)
				_, _ = w.Write([]byte(githubtest.EnvironmentBody{ID: 7, Name: "Prod/Eu"}.JSON()))
			})

			outcome, err := c.UpdateEnvironmentDeploymentBranchPolicy(t.Context(), mustReference(t, "owner/repo"), "Prod/Eu", tc.mode)
			if err != nil || outcome != UpdateSucceeded {
				t.Fatalf("UpdateEnvironmentDeploymentBranchPolicy() = %v, %v, want %v without error", outcome, err, UpdateSucceeded)
			}

			if n := received.Load(); n != 1 {
				t.Errorf("server received %d requests, want 1", n)
			}
			if gotMethod != http.MethodPut || gotPath != "/repos/owner/repo/environments/Prod%2FEu" {
				t.Errorf("request = %s %s, want PUT /repos/owner/repo/environments/Prod%%2FEu", gotMethod, gotPath)
			}
			if gotContentType != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", gotContentType)
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

// TestUpdateEnvironmentDeploymentBranchPolicyPanicsWithoutBody は、本文を組めない方式（個別パターンを伴う selected と未定義の方式）では、
// 要求を送らずに panic することを確かめる。未定義の方式を空の本文で送ると、存在する Environment を何も変えない要求が成功として返るため。
//
//trace:checks INV-001
func TestUpdateEnvironmentDeploymentBranchPolicyPanicsWithoutBody(t *testing.T) {
	for _, mode := range []setting.DeploymentBranchPolicyMode{setting.DeploymentBranchPolicySelected, ""} {
		t.Run("mode "+string(mode)+" panics without a request", func(t *testing.T) {
			var received atomic.Int32
			c := newTestClient(t, func(http.ResponseWriter, *http.Request) { received.Add(1) })
			defer func() {
				if recover() == nil {
					t.Errorf("UpdateEnvironmentDeploymentBranchPolicy(%q) did not panic", mode)
				}
				if n := received.Load(); n != 0 {
					t.Errorf("server received %d requests, want 0", n)
				}
			}()
			_, _ = c.UpdateEnvironmentDeploymentBranchPolicy(t.Context(), mustReference(t, "owner/repo"), "production", mode)
		})
	}
}

// TestUpdateEnvironmentDeploymentBranchPolicyErrorNamesTheOperation は、方式の要求の失敗が、作成とは別の操作名と名前を示すことを確かめる。
//
//trace:checks FR-013
func TestUpdateEnvironmentDeploymentBranchPolicyErrorNamesTheOperation(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnprocessableEntity) })

	_, err := c.UpdateEnvironmentDeploymentBranchPolicy(t.Context(), mustReference(t, "example/project-a"), "Prod/Eu", setting.DeploymentBranchPolicyAll)
	if err == nil || !strings.HasPrefix(err.Error(), `set deployment branch policy of environment "Prod/Eu" example/project-a: `) {
		t.Errorf("error = %v, want it to start with the operation, the quoted name and example/project-a", err)
	}
}

// TestUpdateEnvironmentOutcome は、Environment の更新の応答から成否を判定し、再送しないことを確かめる。
// 方式の更新と作成は同じ非公開の更新要求を通るため、作成で確かめる。
//
//trace:checks FR-013 INV-005
func TestUpdateEnvironmentOutcome(t *testing.T) {
	validBody := githubtest.EnvironmentBody{ID: 7, Name: "Prod/Eu"}.JSON()
	cases := []struct {
		name        string
		handler     http.HandlerFunc
		wantOutcome UpdateOutcome
		wantErr     error
	}{
		{
			name:        "422 is rejected",
			handler:     func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnprocessableEntity) },
			wantOutcome: UpdateRejected,
			wantErr:     ErrUnprocessable,
		},
		{
			name: "201 is outcome unknown",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(validBody))
			},
			wantOutcome: UpdateUnknown,
		},
		{
			name: "200 without a required field is outcome unknown",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"id": 7, "name": "Prod/Eu"}`))
			},
			wantOutcome: UpdateUnknown,
			wantErr:     ErrUnexpectedResponse,
		},
		{
			name:        "502 is outcome unknown",
			handler:     func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) },
			wantOutcome: UpdateUnknown,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var received atomic.Int32
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				received.Add(1)
				tc.handler(w, r)
			})

			outcome, err := c.CreateEnvironment(t.Context(), mustReference(t, "example/project-a"), "Prod/Eu")
			if outcome != tc.wantOutcome || err == nil {
				t.Fatalf("CreateEnvironment() = %v, %v, want %v with an error", outcome, err, tc.wantOutcome)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("error = %v, want it to wrap %v", err, tc.wantErr)
			}
			if !strings.HasPrefix(err.Error(), `create environment "Prod/Eu" example/project-a: `) {
				t.Errorf("error = %q, want it to start with the operation, the quoted name and example/project-a", err)
			}
			if n := received.Load(); n != 1 {
				t.Errorf("server received %d requests, want 1", n)
			}
		})
	}
}
