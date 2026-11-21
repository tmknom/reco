package github

import (
	"errors"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/tmknom/reco/internal/githubtest"
	"github.com/tmknom/reco/internal/setting"
)

const environmentsPath = "/repos/owner/repo/environments"

// environmentRoutes は、パスごとに登録した応答を返す。パスは r.URL.EscapedPath() で比べ、%2F を区切りと取り違えない。
// 検索条件を含めた要求の列を、届いた順に requests へ記録する。登録のない要求は 500 を返し、テストの失敗として報告する。
type environmentRoutes struct {
	t        *testing.T
	routes   map[string]githubtest.Response
	mu       sync.Mutex
	requests []string
}

func (e *environmentRoutes) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	e.mu.Lock()
	e.requests = append(e.requests, r.URL.EscapedPath()+"?"+r.URL.RawQuery)
	e.mu.Unlock()
	response, ok := e.routes[r.URL.EscapedPath()]
	if !ok {
		e.t.Errorf("got request %s, want one of the registered routes", r.URL)
		response = githubtest.Response{Status: http.StatusInternalServerError}
	}
	response.ServeHTTP(w, r)
}

func (e *environmentRoutes) Requests() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.requests...)
}

const firstPage = "?per_page=100&page=1"

//trace:checks FR-004 QR-002
func TestObserveRepositoryEnvironments(t *testing.T) {
	branch, tag := string(setting.DeploymentPatternBranch), string(setting.DeploymentPatternTag)
	selected := &githubtest.DeploymentBranchPolicyBody{CustomBranchPolicies: true}
	protected := &githubtest.DeploymentBranchPolicyBody{ProtectedBranches: true}
	wait := 15
	reviewers := &githubtest.RequiredReviewersBody{PreventSelfReview: true, Reviewers: []githubtest.ReviewerBody{{Type: "User", ID: 7}}}

	t.Run("an observation without the environments area sends no list request and has no Environments", func(t *testing.T) {
		routes := &environmentRoutes{t: t, routes: map[string]githubtest.Response{
			repositoryPath:          githubtest.OK(repositoryBody),
			workflowPermissionsPath: githubtest.OK(workflowPermissionsBody),
		}}
		c := newTestClient(t, routes.ServeHTTP)

		observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaWorkflowPermissions)
		if err != nil {
			t.Fatalf("ObserveRepository() error = %v", err)
		}
		if observed.Environments != nil {
			t.Errorf("ObserveRepository().Environments = %+v, want nil", observed.Environments)
		}
		want := []string{repositoryPath + "?", workflowPermissionsPath + "?"}
		if got := routes.Requests(); !reflect.DeepEqual(got, want) {
			t.Errorf("requests = %q, want %q", got, want)
		}
	})

	t.Run("environments are read after the actions access", func(t *testing.T) {
		routes := &environmentRoutes{t: t, routes: map[string]githubtest.Response{
			repositoryPath:          githubtest.OK(privateRepositoryBody),
			actionsPermissionsPath:  githubtest.OK(actionsPermissionsBody("all")),
			workflowPermissionsPath: githubtest.OK(workflowPermissionsBody),
			actionsAccessPath:       githubtest.OK(actionsAccessBody),
			environmentsPath:        githubtest.OK(githubtest.EnvironmentsBody{}.JSON()),
		}}
		c := newTestClient(t, routes.ServeHTTP)

		observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"),
			setting.AreaEnvironments, setting.AreaActionsAccess, setting.AreaWorkflowPermissions, setting.AreaActionsPermissions)
		if err != nil {
			t.Fatalf("ObserveRepository() error = %v", err)
		}
		want := []string{
			repositoryPath + "?",
			actionsPermissionsPath + "?",
			workflowPermissionsPath + "?",
			actionsAccessPath + "?",
			environmentsPath + firstPage,
		}
		if got := routes.Requests(); !reflect.DeepEqual(got, want) {
			t.Errorf("requests = %q, want %q", got, want)
		}
		if !reflect.DeepEqual(observed.Environments, &Environments{Existing: []Environment{}}) {
			t.Errorf("ObserveRepository().Environments = %+v, want a non-nil empty list", observed.Environments)
		}
	})

	t.Run("deployment patterns are read only for selected environments in the list order", func(t *testing.T) {
		list := githubtest.EnvironmentsBody{TotalCount: 4, Environments: []githubtest.EnvironmentBody{
			{ID: 1, Name: "open"},
			{ID: 2, Name: "main-only", DeploymentBranchPolicy: selected, WaitTimer: &wait, RequiredReviewers: reviewers, CanAdminsBypass: true},
			{ID: 3, Name: "guarded", DeploymentBranchPolicy: protected},
			{ID: 4, Name: "Prod/Eu 100%", DeploymentBranchPolicy: selected},
		}}
		routes := &environmentRoutes{t: t, routes: map[string]githubtest.Response{
			repositoryPath:   githubtest.OK(repositoryBody),
			environmentsPath: githubtest.OK(list.JSON()),
			environmentsPath + "/main-only/deployment-branch-policies": githubtest.OK(githubtest.DeploymentBranchPoliciesBody{TotalCount: 3, Patterns: []githubtest.DeploymentPatternBody{
				{ID: 10, Name: "main", Type: branch},
				{ID: 11, Name: "main", Type: tag},
				{ID: 12, Name: "v*", Type: tag},
			}}.JSON()),
			environmentsPath + "/Prod%2FEu%20100%25/deployment-branch-policies": githubtest.OK(githubtest.DeploymentBranchPoliciesBody{}.JSON()),
		}}
		c := newTestClient(t, routes.ServeHTTP)

		observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaEnvironments)
		if err != nil {
			t.Fatalf("ObserveRepository() error = %v", err)
		}
		wantRequests := []string{
			repositoryPath + "?",
			environmentsPath + firstPage,
			environmentsPath + "/main-only/deployment-branch-policies" + firstPage,
			environmentsPath + "/Prod%2FEu%20100%25/deployment-branch-policies" + firstPage,
		}
		if got := routes.Requests(); !reflect.DeepEqual(got, wantRequests) {
			t.Errorf("requests = %q, want %q", got, wantRequests)
		}

		mainBranch := setting.DeploymentPattern{Type: setting.DeploymentPatternBranch, Name: "main"}
		mainTag := setting.DeploymentPattern{Type: setting.DeploymentPatternTag, Name: "main"}
		versionTag := setting.DeploymentPattern{Type: setting.DeploymentPatternTag, Name: "v*"}
		want := &Environments{Existing: []Environment{
			{ID: 1, Name: "open", DeploymentBranchPolicy: setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyAll}},
			{
				ID:   2,
				Name: "main-only",
				DeploymentBranchPolicy: setting.DeploymentBranchPolicy{
					Mode:     setting.DeploymentBranchPolicySelected,
					Patterns: []setting.DeploymentPattern{mainBranch, mainTag, versionTag},
				},
				DeploymentPatternIDs: map[setting.DeploymentPattern]DeploymentPatternID{mainBranch: 10, mainTag: 11, versionTag: 12},
				Protection: setting.EnvironmentProtection{
					WaitTimer:         &wait,
					RequiredReviewers: &setting.RequiredReviewers{PreventSelfReview: true, Reviewers: []setting.Reviewer{{Type: setting.ReviewerUser, ID: 7}}},
					CanAdminsBypass:   true,
				},
			},
			{ID: 3, Name: "guarded", DeploymentBranchPolicy: setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyProtectedBranches}},
			{
				ID:   4,
				Name: "Prod/Eu 100%",
				DeploymentBranchPolicy: setting.DeploymentBranchPolicy{
					Mode:     setting.DeploymentBranchPolicySelected,
					Patterns: []setting.DeploymentPattern{},
				},
				DeploymentPatternIDs: map[setting.DeploymentPattern]DeploymentPatternID{},
			},
		}}
		if !reflect.DeepEqual(observed.Environments, want) {
			t.Errorf("ObserveRepository().Environments = %+v, want %+v", observed.Environments, want)
		}
	})
}

// TestObserveRepositoryEnvironmentsFailure は、Environments の読み取りの失敗が、種類を保ったまま
// 取得できた値を含まない観測操作の失敗になることを確かめる。種類の判定規則は TestClassifyFailure が確かめる。
//
//trace:checks FR-003 FR-004 QR-001 QR-002
func TestObserveRepositoryEnvironmentsFailure(t *testing.T) {
	const bodyMarker = "BODY-MARKER"
	selected := &githubtest.DeploymentBranchPolicyBody{CustomBranchPolicies: true}
	policiesPath := func(name string) string { return environmentsPath + "/" + name + "/deployment-branch-policies" }
	list := func(names ...string) githubtest.Response {
		body := githubtest.EnvironmentsBody{TotalCount: len(names)}
		for i, name := range names {
			body.Environments = append(body.Environments, githubtest.EnvironmentBody{ID: int64(i + 1), Name: name, DeploymentBranchPolicy: selected})
		}
		return githubtest.OK(body.JSON())
	}
	onePattern := githubtest.OK(githubtest.DeploymentBranchPoliciesBody{TotalCount: 1, Patterns: []githubtest.DeploymentPatternBody{{ID: 1, Name: "main", Type: "branch"}}}.JSON())

	cases := []struct {
		name         string
		routes       map[string]githubtest.Response
		want         error
		wantContains string
		wantRequests []string
	}{
		{
			name:         "403 on the environments list is forbidden",
			routes:       map[string]githubtest.Response{environmentsPath: {Status: http.StatusForbidden, Body: bodyMarker}},
			want:         ErrForbidden,
			wantRequests: []string{repositoryPath + "?", environmentsPath + firstPage},
		},
		{
			name:         "404 on the deployment branch policies of a selected environment is not found",
			routes:       map[string]githubtest.Response{environmentsPath: list("a"), policiesPath("a"): {Status: http.StatusNotFound, Body: bodyMarker}},
			want:         ErrNotFound,
			wantContains: `deployment branch policies of environment "a"`,
			wantRequests: []string{repositoryPath + "?", environmentsPath + firstPage, policiesPath("a") + firstPage},
		},
		{
			name:         "failure on a later environment's policies discards the earlier ones",
			routes:       map[string]githubtest.Response{environmentsPath: list("a", "b"), policiesPath("a"): onePattern, policiesPath("b"): {Status: http.StatusForbidden, Body: bodyMarker}},
			want:         ErrForbidden,
			wantContains: `deployment branch policies of environment "b"`,
			wantRequests: []string{repositoryPath + "?", environmentsPath + firstPage, policiesPath("a") + firstPage, policiesPath("b") + firstPage},
		},
		{
			name: "invalid element in the list stops before any policies request",
			routes: map[string]githubtest.Response{environmentsPath: githubtest.OK(
				`{"total_count": 2, "environments": [{"id": 1, "name": "a", "can_admins_bypass": false, "protection_rules": [], "deployment_branch_policy": {"protected_branches": false, "custom_branch_policies": true}}, {"id": 2}]}`)},
			want:         ErrUnexpectedResponse,
			wantRequests: []string{repositoryPath + "?", environmentsPath + firstPage},
		},
		{
			name:         "invalid pattern is unexpected response",
			routes:       map[string]githubtest.Response{environmentsPath: list("a"), policiesPath("a"): githubtest.OK(`{"total_count": 1, "branch_policies": [{"id": 1, "name": "x", "type": "commit"}]}`)},
			want:         ErrUnexpectedResponse,
			wantContains: `deployment branch policies of environment "a"`,
			wantRequests: []string{repositoryPath + "?", environmentsPath + firstPage, policiesPath("a") + firstPage},
		},
		{
			name:         "two patterns of the same type and name are unexpected response",
			routes:       map[string]githubtest.Response{environmentsPath: list("a"), policiesPath("a"): githubtest.OK(`{"total_count": 2, "branch_policies": [{"id": 1, "name": "main", "type": "branch"}, {"id": 2, "name": "main", "type": "branch"}]}`)},
			want:         ErrUnexpectedResponse,
			wantContains: "duplicate deployment pattern",
			wantRequests: []string{repositoryPath + "?", environmentsPath + firstPage, policiesPath("a") + firstPage},
		},
		{
			name:         "two environments whose names differ only in letter case are unexpected response before any policies request",
			routes:       map[string]githubtest.Response{environmentsPath: list("Prod", "prod"), policiesPath("Prod"): onePattern, policiesPath("prod"): onePattern},
			want:         ErrUnexpectedResponse,
			wantContains: `duplicate environment name "prod"`,
			wantRequests: []string{repositoryPath + "?", environmentsPath + firstPage},
		},
		{
			name:         "selected environment named dot sends no policies request",
			routes:       map[string]githubtest.Response{environmentsPath: list("."), environmentsPath + "/./deployment-branch-policies": onePattern},
			want:         ErrUnexpectedResponse,
			wantContains: `deployment branch policies of environment "."`,
			wantRequests: []string{repositoryPath + "?", environmentsPath + firstPage},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.routes[repositoryPath] = githubtest.OK(repositoryBody)
			routes := &environmentRoutes{t: t, routes: tc.routes}
			c := newTestClient(t, routes.ServeHTTP)

			observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaEnvironments)
			if !errors.Is(err, tc.want) {
				t.Fatalf("ObserveRepository() error = %v, want %v", err, tc.want)
			}
			if observed != (ObservationResult{}) {
				t.Errorf("ObserveRepository() = %+v, want the zero value", observed)
			}
			msg := err.Error()
			for _, s := range []string{"fetch environments owner/repo", tc.wantContains} {
				if !strings.Contains(msg, s) {
					t.Errorf("error = %q, want it to contain %q", msg, s)
				}
			}
			for _, secret := range []string{testToken, bodyMarker} {
				if strings.Contains(msg, secret) {
					t.Errorf("error = %q, must not contain %q", msg, secret)
				}
			}
			if got := routes.Requests(); !reflect.DeepEqual(got, tc.wantRequests) {
				t.Errorf("requests = %q, want %q", got, tc.wantRequests)
			}
		})
	}
}

//trace:checks FR-004
func TestEnvironments_Lookup(t *testing.T) {
	declared := mustEnvironmentName(t, "Production")
	undetermined := "café"

	tests := []struct {
		name       string
		existing   []Environment
		wantID     EnvironmentID
		wantName   string
		wantAbsent bool
		wantReason UnavailableReason
	}{
		{
			name:     "same name with different case exists with observed spelling and ID",
			existing: []Environment{{ID: 1, Name: "other"}, {ID: 2, Name: "PRODUCTION"}},
			wantID:   2,
			wantName: "PRODUCTION",
		},
		{
			name:       "empty list is absent",
			existing:   nil,
			wantAbsent: true,
		},
		{
			name:       "only comparable different names is absent",
			existing:   []Environment{{ID: 1, Name: "staging"}, {ID: 2, Name: "prod"}},
			wantAbsent: true,
		},
		{
			name:       "only undetermined name is unavailable",
			existing:   []Environment{{ID: 1, Name: undetermined}},
			wantReason: UnavailableEnvironmentNameUndetermined,
		},
		{
			name:     "undetermined name before the same name still exists",
			existing: []Environment{{ID: 1, Name: undetermined}, {ID: 2, Name: "production"}},
			wantID:   2,
			wantName: "production",
		},
		{
			name:       "comparable different name after an undetermined name is still unavailable",
			existing:   []Environment{{ID: 1, Name: undetermined}, {ID: 2, Name: "staging"}},
			wantReason: UnavailableEnvironmentNameUndetermined,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Environments{Existing: tt.existing}.Lookup(declared)
			switch {
			case tt.wantAbsent:
				if !got.Absent() {
					t.Errorf("Absent() = false, want true")
				}
				if _, ok := got.Existing(); ok {
					t.Errorf("Existing() ok = true, want false")
				}
			case tt.wantReason != 0:
				if got.Absent() {
					t.Errorf("Absent() = true, want false")
				}
				if _, ok := got.Existing(); ok {
					t.Errorf("Existing() ok = true, want false")
				}
				if reason := got.UnavailableReason(); reason != tt.wantReason {
					t.Errorf("UnavailableReason() = %v, want %v", reason, tt.wantReason)
				}
			default:
				env, ok := got.Existing()
				if !ok {
					t.Fatalf("Existing() ok = false, want true")
				}
				if env.ID != tt.wantID || env.Name != tt.wantName {
					t.Errorf("Existing() = (%d, %q), want (%d, %q)", env.ID, env.Name, tt.wantID, tt.wantName)
				}
				if got.Absent() {
					t.Errorf("Absent() = true, want false")
				}
			}
		})
	}
}
