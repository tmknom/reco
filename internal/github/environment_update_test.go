package github

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/tmknom/reco/internal/githubtest"
	"github.com/tmknom/reco/internal/setting"
)

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
		{name: "create with deployment branch policy", update: func() {
			_, _ = c.CreateEnvironmentWithDeploymentBranchPolicy(t.Context(), reference, "..", setting.DeploymentBranchPolicyAll)
		}},
		{name: "deployment branch policy", update: func() {
			_, _ = c.UpdateEnvironmentDeploymentBranchPolicy(t.Context(), reference, "..", setting.DeploymentBranchPolicyAll)
		}},
		{name: "required reviewers", update: func() {
			_, _ = c.UpdateEnvironmentRequiredReviewers(t.Context(), reference, "..", setting.ReviewerSetOf(nil))
		}},
		{name: "add deployment pattern", update: func() {
			_, _ = c.AddDeploymentPattern(t.Context(), reference, "..", setting.DeploymentPattern{Type: setting.DeploymentPatternBranch, Name: "main"})
		}},
		{name: "remove deployment pattern", update: func() {
			_, _ = c.RemoveDeploymentPattern(t.Context(), reference, "..", setting.DeploymentPattern{Type: setting.DeploymentPatternBranch, Name: "main"}, 42)
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
		{name: "selected sends protected_branches false and custom_branch_policies true", mode: setting.DeploymentBranchPolicySelected, want: map[string]any{"deployment_branch_policy": map[string]any{"protected_branches": false, "custom_branch_policies": true}}},
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

// TestUpdateEnvironmentDeploymentBranchPolicyPanicsWithoutBody は、本文を組めない未定義の方式では、
// 要求を送らずに panic することを確かめる。未定義の方式を空の本文で送ると、存在する Environment を何も変えない要求が成功として返るため。
//
//trace:checks INV-001
func TestUpdateEnvironmentDeploymentBranchPolicyPanicsWithoutBody(t *testing.T) {
	for _, mode := range []setting.DeploymentBranchPolicyMode{""} {
		t.Run("undefined mode "+string(mode)+" panics without a request", func(t *testing.T) {
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

// TestCreateEnvironmentWithDeploymentBranchPolicy は、方式を伴う作成が方式を本文に入れて送り、
// 失敗を方式の更新ではなく作成として示すことを確かめる。利用者が承認したのは作成のため。
//
//trace:checks INV-001 FR-011 FR-013
func TestCreateEnvironmentWithDeploymentBranchPolicy(t *testing.T) {
	var gotBody []byte
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusUnprocessableEntity)
	})

	_, err := c.CreateEnvironmentWithDeploymentBranchPolicy(t.Context(), mustReference(t, "example/project-a"), "Prod/Eu", setting.DeploymentBranchPolicySelected)

	var got map[string]any
	if err := json.Unmarshal(gotBody, &got); err != nil {
		t.Fatalf("request body %q is not JSON: %v", gotBody, err)
	}
	want := map[string]any{"deployment_branch_policy": map[string]any{"protected_branches": false, "custom_branch_policies": true}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("request body = %v, want %v", got, want)
	}
	if err == nil || !strings.HasPrefix(err.Error(), `create environment with deployment branch policy "Prod/Eu" example/project-a: `) {
		t.Errorf("error = %v, want it to start with the creation, the quoted name and example/project-a", err)
	}
}

// TestUpdateEnvironmentRequiredReviewers は、本文が reviewers のキーだけで、担当者を種類と ID にして宣言の順に送ることを確かめる。
//
//trace:checks INV-001 FR-011
func TestUpdateEnvironmentRequiredReviewers(t *testing.T) {
	cases := []struct {
		name      string
		reviewers []setting.Reviewer
		wantBody  string
	}{
		{
			name:      "reviewers keep the declared order",
			reviewers: []setting.Reviewer{{Type: setting.ReviewerUser, ID: 1}, {Type: setting.ReviewerUser, ID: 2}},
			wantBody:  `{"reviewers":[{"type":"User","id":1},{"type":"User","id":2}]}`,
		},
		{
			name:      "descending ids are not sorted",
			reviewers: []setting.Reviewer{{Type: setting.ReviewerTeam, ID: 9}, {Type: setting.ReviewerUser, ID: 3}},
			wantBody:  `{"reviewers":[{"type":"Team","id":9},{"type":"User","id":3}]}`,
		},
		{
			name:      "empty set sends an empty array",
			reviewers: nil,
			wantBody:  `{"reviewers":[]}`,
		},
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

			outcome, err := c.UpdateEnvironmentRequiredReviewers(t.Context(), mustReference(t, "owner/repo"), "Prod/Eu", setting.ReviewerSetOf(tc.reviewers))
			if err != nil || outcome != UpdateSucceeded {
				t.Fatalf("UpdateEnvironmentRequiredReviewers() = %v, %v, want %v without error", outcome, err, UpdateSucceeded)
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
			if string(gotBody) != tc.wantBody {
				t.Errorf("request body = %s, want %s", gotBody, tc.wantBody)
			}
		})
	}
}

// TestUpdateEnvironmentRequiredReviewersPanicsOnRuleWithoutReviewers は、担当者のいない保護ルールを要求を送らずに panic することを確かめる。
// 宣言できない値を空の集合として送ると、保護ルールを外す要求になるため。
//
//trace:checks INV-001
func TestUpdateEnvironmentRequiredReviewersPanicsOnRuleWithoutReviewers(t *testing.T) {
	var received atomic.Int32
	c := newTestClient(t, func(http.ResponseWriter, *http.Request) { received.Add(1) })
	ruleWithoutReviewers := setting.ReviewerSetOfRule(&setting.RequiredReviewers{})
	defer func() {
		if recover() == nil {
			t.Error("UpdateEnvironmentRequiredReviewers with a rule without reviewers did not panic")
		}
		if n := received.Load(); n != 0 {
			t.Errorf("server received %d requests, want 0", n)
		}
	}()
	_, _ = c.UpdateEnvironmentRequiredReviewers(t.Context(), mustReference(t, "owner/repo"), "production", ruleWithoutReviewers)
}

// TestUpdateEnvironmentRequiredReviewersErrorNamesTheOperation は、担当者の要求の失敗が、ほかの Environment の要求とは別の操作名と名前を示すことを確かめる。
//
//trace:checks FR-013
func TestUpdateEnvironmentRequiredReviewersErrorNamesTheOperation(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnprocessableEntity) })

	_, err := c.UpdateEnvironmentRequiredReviewers(t.Context(), mustReference(t, "example/project-a"), "Prod/Eu", setting.ReviewerSetOf(nil))
	if err == nil || !strings.HasPrefix(err.Error(), `set required reviewers of environment "Prod/Eu" example/project-a: `) {
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

// TestAddDeploymentPattern は、個別パターン一つを加える POST を、名前と種類を本文にして一度だけ送ることを確かめる。
//
//trace:checks INV-001 FR-011
func TestAddDeploymentPattern(t *testing.T) {
	cases := []struct {
		name    string
		pattern setting.DeploymentPattern
		// wantBody は、名前のエスケープの形まで固定するため、本文の JSON の文字列で持つ。
		wantBody string
	}{
		{name: "branch", pattern: setting.DeploymentPattern{Type: setting.DeploymentPatternBranch, Name: "release/*"}, wantBody: `{"name":"release/*","type":"branch"}`},
		{name: "tag sends the type tag", pattern: setting.DeploymentPattern{Type: setting.DeploymentPatternTag, Name: "v*"}, wantBody: `{"name":"v*","type":"tag"}`},
		{name: "control character is escaped as JSON", pattern: setting.DeploymentPattern{Type: setting.DeploymentPatternBranch, Name: "a\x01b"}, wantBody: `{"name":"a\u0001b","type":"branch"}`},
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
				_, _ = w.Write([]byte(githubtest.DeploymentPatternBody{ID: 5, Name: "created", Type: "branch"}.JSON()))
			})

			outcome, err := c.AddDeploymentPattern(t.Context(), mustReference(t, "owner/repo"), "production", tc.pattern)
			if err != nil || outcome != UpdateSucceeded {
				t.Fatalf("AddDeploymentPattern() = %v, %v, want %v without error", outcome, err, UpdateSucceeded)
			}

			if n := received.Load(); n != 1 {
				t.Errorf("server received %d requests, want 1", n)
			}
			if gotMethod != http.MethodPost || gotPath != "/repos/owner/repo/environments/production/deployment-branch-policies" {
				t.Errorf("request = %s %s, want POST /repos/owner/repo/environments/production/deployment-branch-policies", gotMethod, gotPath)
			}
			if gotContentType != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", gotContentType)
			}
			if string(gotBody) != tc.wantBody {
				t.Errorf("request body = %s, want %s", gotBody, tc.wantBody)
			}
		})
	}
}

// TestAddDeploymentPatternOutcome は、個別パターンを加える応答から成否を判定し、再送しないことを確かめる。
// 303 は、リポジトリの名前の変更の案内ではなく、同じ個別パターンがすでにあるときの応答であることを示し、成否不明のままにする。
//
//trace:checks FR-013 INV-005
func TestAddDeploymentPatternOutcome(t *testing.T) {
	validBody := githubtest.DeploymentPatternBody{ID: 5, Name: "release/*", Type: "branch"}.JSON()
	cases := []struct {
		name        string
		handler     http.HandlerFunc
		wantOutcome UpdateOutcome
		wantErr     error
		wantMessage string
		// unwantedMessage は、error に含まれてはならない文言。
		unwantedMessage string
	}{
		{
			name: "201 is outcome unknown",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(validBody))
			},
			wantOutcome: UpdateUnknown,
		},
		{
			name: "200 without id is outcome unknown",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"name": "release/*", "type": "branch"}`))
			},
			wantOutcome: UpdateUnknown,
			wantErr:     ErrUnexpectedResponse,
		},
		{
			name: "303 is outcome unknown and explains the pattern may already exist",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Location", "https://api.github.com/repositories/1/environments/production/deployment-branch-policies/5")
				w.WriteHeader(http.StatusSeeOther)
			},
			wantOutcome:     UpdateUnknown,
			wantErr:         ErrRedirect,
			wantMessage:     "redirect not followed (HTTP 303): GitHub returns this when the pattern already exists",
			unwantedMessage: "renamed",
		},
		{
			name: "301 keeps the repository rename guidance",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Location", "https://api.github.com/repositories/1")
				w.WriteHeader(http.StatusMovedPermanently)
			},
			wantOutcome: UpdateUnknown,
			wantErr:     ErrRedirect,
			wantMessage: "the repository may have been renamed or transferred",
		},
		{
			name:        "404 is rejected",
			handler:     func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) },
			wantOutcome: UpdateRejected,
			wantErr:     ErrNotFound,
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

			outcome, err := c.AddDeploymentPattern(t.Context(), mustReference(t, "example/project-a"), "production", setting.DeploymentPattern{Type: setting.DeploymentPatternBranch, Name: "release/*"})
			if outcome != tc.wantOutcome || err == nil {
				t.Fatalf("AddDeploymentPattern() = %v, %v, want %v with an error", outcome, err, tc.wantOutcome)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("error = %v, want it to wrap %v", err, tc.wantErr)
			}
			if !strings.HasPrefix(err.Error(), `add deployment pattern branch "release/*" to environment "production" example/project-a: `) {
				t.Errorf("error = %q, want it to start with the operation, the quoted names and example/project-a", err)
			}
			if tc.wantMessage != "" && !strings.Contains(err.Error(), tc.wantMessage) {
				t.Errorf("error = %q, want it to contain %q", err, tc.wantMessage)
			}
			if tc.unwantedMessage != "" && strings.Contains(err.Error(), tc.unwantedMessage) {
				t.Errorf("error = %q, want it not to contain %q", err, tc.unwantedMessage)
			}
			if n := received.Load(); n != 1 {
				t.Errorf("server received %d requests, want 1", n)
			}
		})
	}
}

// TestRemoveDeploymentPattern は、ID の個別パターンを取り除く DELETE を、本文なしで一度だけ送ることを確かめる。
//
//trace:checks INV-001 FR-011
func TestRemoveDeploymentPattern(t *testing.T) {
	var received atomic.Int32
	var gotMethod, gotPath string
	var gotHeader http.Header
	var gotBody []byte
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		gotMethod, gotPath, gotHeader = r.Method, r.URL.EscapedPath(), r.Header
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	})

	outcome, err := c.RemoveDeploymentPattern(t.Context(), mustReference(t, "owner/repo"), "a/b", setting.DeploymentPattern{Type: setting.DeploymentPatternBranch, Name: "feature/*"}, 67527066)
	if err != nil || outcome != UpdateSucceeded {
		t.Fatalf("RemoveDeploymentPattern() = %v, %v, want %v without error", outcome, err, UpdateSucceeded)
	}

	if n := received.Load(); n != 1 {
		t.Errorf("server received %d requests, want 1", n)
	}
	if want := "/repos/owner/repo/environments/a%2Fb/deployment-branch-policies/67527066"; gotMethod != http.MethodDelete || gotPath != want {
		t.Errorf("request = %s %s, want DELETE %s", gotMethod, gotPath, want)
	}
	if len(gotBody) != 0 {
		t.Errorf("request body = %q, want none", gotBody)
	}
	if v, ok := gotHeader["Content-Type"]; ok {
		t.Errorf("Content-Type = %q, want the header absent", v)
	}
}

// TestRemoveDeploymentPatternOutcome は、個別パターンを取り除く応答から成否を判定し、再送しないことを確かめる。
//
//trace:checks FR-013 INV-005
func TestRemoveDeploymentPatternOutcome(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		wantOutcome UpdateOutcome
		wantErr     error
	}{
		{name: "200 is outcome unknown", status: http.StatusOK, wantOutcome: UpdateUnknown},
		{name: "404 is rejected", status: http.StatusNotFound, wantOutcome: UpdateRejected, wantErr: ErrNotFound},
		{name: "502 is outcome unknown", status: http.StatusBadGateway, wantOutcome: UpdateUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var received atomic.Int32
			c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				received.Add(1)
				w.WriteHeader(tc.status)
			})

			outcome, err := c.RemoveDeploymentPattern(t.Context(), mustReference(t, "example/project-a"), "production", setting.DeploymentPattern{Type: setting.DeploymentPatternTag, Name: "v*"}, 67527066)
			if outcome != tc.wantOutcome || err == nil {
				t.Fatalf("RemoveDeploymentPattern() = %v, %v, want %v with an error", outcome, err, tc.wantOutcome)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("error = %v, want it to wrap %v", err, tc.wantErr)
			}
			if !strings.HasPrefix(err.Error(), `remove deployment pattern tag "v*" (id 67527066) from environment "production" example/project-a: `) {
				t.Errorf("error = %q, want it to start with the operation, the ID, the quoted name and example/project-a", err)
			}
			if n := received.Load(); n != 1 {
				t.Errorf("server received %d requests, want 1", n)
			}
		})
	}
}
