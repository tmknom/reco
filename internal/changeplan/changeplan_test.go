package changeplan

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/githubtest"
	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

const twoRepositoriesSource = `repositories:
  - name: owner/first
    general:
      allow_auto_merge: true
  - name: owner/second
    general:
      allow_auto_merge: true
      delete_branch_on_merge: true
`

//trace:checks FR-006 FR-007 FR-008
func TestPlan_NotApplicableError(t *testing.T) {
	zeta, alpha := mustReference(t, "owner/zeta"), mustReference(t, "owner/alpha")
	match := SettingPlan{Key: setting.AllowAutoMerge, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(true), Result: Match}
	applicable := SettingPlan{Key: setting.AllowAutoMerge, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicable}
	notApplicable := SettingPlan{
		Key: setting.SHAPinningRequired, Observed: github.Unavailable[setting.Value](github.UnavailableActionsDisabled), Desired: setting.BoolValue(true),
		Result: NotComparable, Applicability: NotApplicable, NotApplicableReason: NotApplicableActionsRemainDisabled,
	}
	cases := []struct {
		name         string
		repositories []RepositoryPlan
		want         string
	}{
		{
			name:         "plan without not-applicable changes has no error",
			repositories: []RepositoryPlan{{Target: zeta, Settings: []SettingPlan{match, applicable}}},
		},
		{
			name: "not-applicable settings in two repositories are all named in the change plan order",
			repositories: []RepositoryPlan{
				{Target: zeta, Settings: []SettingPlan{applicable, notApplicable}},
				{Target: alpha, Settings: []SettingPlan{notApplicable}},
			},
			want: "plan cannot be applied: owner/zeta: actions.permissions.sha_pinning_required is not applicable; owner/alpha: actions.permissions.sha_pinning_required is not applicable",
		},
		{
			name: "not-applicable environment is named with its environment name",
			repositories: []RepositoryPlan{{Target: zeta, Settings: []SettingPlan{{
				Key: setting.EnvironmentResource, Environment: mustEnvironmentName(t, "cafe"), Result: NotComparable,
				Applicability: NotApplicable, NotApplicableReason: NotApplicableEnvironmentNameUndetermined,
			}}}},
			want: "plan cannot be applied: owner/zeta: environments.cafe is not applicable",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Plan{Repositories: tc.repositories}.NotApplicableError()
			if tc.want == "" {
				if err != nil {
					t.Errorf("NotApplicableError() = %v, want nil", err)
				}
				return
			}
			if err == nil || err.Error() != tc.want {
				t.Errorf("NotApplicableError() = %v, want %q", err, tc.want)
			}
		})
	}
}

//trace:checks FR-007
func TestSettingPlan_IsCreation(t *testing.T) {
	production := mustEnvironmentName(t, "production")
	cases := []struct {
		name string
		plan SettingPlan
		want bool
	}{
		{name: "mismatch of an environment is a creation", plan: SettingPlan{Key: setting.EnvironmentResource, Environment: production, Result: Mismatch}, want: true},
		{name: "matched environment is not a creation", plan: SettingPlan{Key: setting.EnvironmentResource, Environment: production, Result: Match}},
		{name: "environment whose existence cannot be determined is not a creation", plan: SettingPlan{Key: setting.EnvironmentResource, Environment: production, Result: NotComparable}},
		{name: "mismatch of a setting in an environment is not a creation", plan: SettingPlan{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production, Result: Mismatch}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.plan.IsCreation(); got != tc.want {
				t.Errorf("IsCreation() = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestSettingPlan_UnavailableReason(t *testing.T) {
	t.Run("environment existence reads the reason of the environment observation", func(t *testing.T) {
		s := SettingPlan{Key: setting.EnvironmentResource, ObservedEnvironment: lookup(t, "production", github.Environment{Name: "prodüction"}), Result: NotComparable}
		if got := s.UnavailableReason(); got != github.UnavailableEnvironmentNameUndetermined {
			t.Errorf("UnavailableReason() = %v, want %v", got, github.UnavailableEnvironmentNameUndetermined)
		}
	})
	t.Run("other settings read the reason of the setting observation", func(t *testing.T) {
		s := SettingPlan{Key: setting.EnvironmentDeploymentBranchPolicy, Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent), Result: NotComparable}
		if got := s.UnavailableReason(); got != github.UnavailableEnvironmentAbsent {
			t.Errorf("UnavailableReason() = %v, want %v", got, github.UnavailableEnvironmentAbsent)
		}
	})
}

func TestRepositoryPlan_Count(t *testing.T) {
	r := RepositoryPlan{Settings: []SettingPlan{{Result: Mismatch}, {Result: Match}, {Result: Mismatch}, {Result: NotComparable}}}
	if got := r.Count(Mismatch); got != 2 {
		t.Errorf("Count(Mismatch) = %d, want 2", got)
	}
}

// TestSettingPlan_IsPlannedChange は、変更予定を不一致と比較不能の管理設定とする定義を確かめる。
//
//trace:checks FR-008
func TestSettingPlan_IsPlannedChange(t *testing.T) {
	cases := []struct {
		name   string
		result ReconciliationResult
		want   bool
	}{
		{name: "match is not a planned change", result: Match, want: false},
		{name: "mismatch is a planned change", result: Mismatch, want: true},
		{name: "not comparable is a planned change", result: NotComparable, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := (SettingPlan{Result: tc.result}).IsPlannedChange(); got != tc.want {
				t.Errorf("IsPlannedChange() = %v, want %v", got, tc.want)
			}
		})
	}
}

//trace:checks FR-008
func TestRepositoryPlan_HasPlannedChanges(t *testing.T) {
	cases := []struct {
		name     string
		settings []SettingPlan
		want     bool
	}{
		{name: "no setting has no planned change", want: false},
		{name: "only matches have no planned change", settings: []SettingPlan{{Result: Match}, {Result: Match}}, want: false},
		{name: "a planned change after a match is found", settings: []SettingPlan{{Result: Match}, {Result: Mismatch}}, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := (RepositoryPlan{Settings: tc.settings}).HasPlannedChanges(); got != tc.want {
				t.Errorf("HasPlannedChanges() = %v, want %v", got, tc.want)
			}
		})
	}
}

//trace:checks FR-006 FR-007
func TestRepositoryPlan_NotApplicableChanges(t *testing.T) {
	notApplicable := SettingPlan{
		Key: setting.SHAPinningRequired, Observed: github.Unavailable[setting.Value](github.UnavailableActionsDisabled), Desired: setting.BoolValue(true),
		Result: NotComparable, Applicability: NotApplicable, NotApplicableReason: NotApplicableActionsRemainDisabled,
	}
	applicable := SettingPlan{Key: setting.AllowAutoMerge, Result: Mismatch, Applicability: Applicable}
	match := SettingPlan{Key: setting.DeleteBranchOnMerge, Result: Match}
	cases := []struct {
		name     string
		settings []SettingPlan
		want     []SettingPlan
	}{
		{name: "matches and applicable changes have no not-applicable change", settings: []SettingPlan{match, applicable}, want: nil},
		{name: "only the not-applicable change is returned among the other plans", settings: []SettingPlan{applicable, notApplicable, match}, want: []SettingPlan{notApplicable}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := (RepositoryPlan{Settings: tc.settings}).NotApplicableChanges(); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("NotApplicableChanges() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestSettingPlan_EnablesActionsByPolicy(t *testing.T) {
	cases := []struct {
		name string
		plan SettingPlan
		want bool
	}{
		{
			name: "allowed_actions unavailable because actions is disabled enables actions",
			plan: SettingPlan{Key: setting.AllowedActions, Observed: github.Unavailable[setting.Value](github.UnavailableActionsDisabled), Result: NotComparable},
			want: true,
		},
		{
			name: "allowed_actions with an observed policy does not enable actions",
			plan: SettingPlan{Key: setting.AllowedActions, Observed: github.Observed(setting.AllowedActionsPolicyValue(setting.AllowedActionsAll)), Result: Mismatch},
			want: false,
		},
		{
			name: "allowed_actions unavailable for another reason does not enable actions",
			plan: SettingPlan{Key: setting.AllowedActions, Observed: github.Unavailable[setting.Value](github.UnavailableNotSelected), Result: NotComparable},
			want: false,
		},
		{
			name: "selected_actions unavailable because actions is disabled does not enable actions",
			plan: SettingPlan{Key: setting.SelectedActions, Observed: github.Unavailable[setting.Value](github.UnavailableActionsDisabled), Result: NotComparable},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.plan.EnablesActionsByPolicy(); got != tc.want {
				t.Errorf("EnablesActionsByPolicy() = %t, want %t", got, tc.want)
			}
		})
	}
}

//trace:checks FR-001 FR-002 FR-005
func TestGenerate(t *testing.T) {
	file := func(files fstest.MapFS) configuration.File {
		return configuration.File{Dir: files, Name: "reco.yaml", Path: "./reco.yaml"}
	}
	declared := fstest.MapFS{"reco.yaml": {Data: []byte(twoRepositoriesSource)}}

	t.Run("no selection observes all declarations in the declared order", func(t *testing.T) {
		server := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
			"GET /repos/owner/first":  githubtest.OK(githubtest.RepositoryBody{ID: 1, AllowAutoMerge: true, DeleteBranchOnMerge: true}.JSON()),
			"GET /repos/owner/second": githubtest.OK(githubtest.RepositoryBody{ID: 2, AllowAutoMerge: true, DeleteBranchOnMerge: true}.JSON()),
		}))
		client := github.New("test-token", github.WithBaseURL(server.URL()))

		got, err := Generate(context.Background(), client, file(declared), nil)
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		if want := []string{"GET /repos/owner/first", "GET /repos/owner/second"}; !slices.Equal(server.RequestLines(), want) {
			t.Errorf("requests = %v, want %v", server.RequestLines(), want)
		}
		var targets []string
		var ids []repository.ID
		for _, r := range got.Repositories {
			targets = append(targets, r.Target.String())
			ids = append(ids, r.RepositoryID)
		}
		if want := []string{"owner/first", "owner/second"}; !slices.Equal(targets, want) {
			t.Errorf("targets = %v, want %v", targets, want)
		}
		if want := []repository.ID{1, 2}; !slices.Equal(ids, want) {
			t.Errorf("repository IDs = %v, want %v", ids, want)
		}
	})

	t.Run("selected declaration is the only one observed and reconciled", func(t *testing.T) {
		server := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
			"GET /repos/owner/second": githubtest.OK(githubtest.RepositoryBody{ID: 7}.JSON()),
		}))
		client := github.New("test-token", github.WithBaseURL(server.URL()))
		sel := mustReference(t, "Owner/Second")

		got, err := Generate(context.Background(), client, file(declared), &sel)
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		if want := []string{"GET /repos/owner/second"}; !slices.Equal(server.RequestLines(), want) {
			t.Errorf("requests = %v, want %v", server.RequestLines(), want)
		}
		want := []RepositoryPlan{{
			Target:       mustReference(t, "owner/second"),
			RepositoryID: 7,
			Settings: []SettingPlan{
				{Key: setting.DeleteBranchOnMerge, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicable},
				{Key: setting.AllowAutoMerge, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicable},
			},
		}}
		if !reflect.DeepEqual(got.Repositories, want) {
			t.Errorf("Repositories = %+v, want %+v", got.Repositories, want)
		}
	})

	t.Run("missing configuration file is reported before any request", func(t *testing.T) {
		server := githubtest.NewServer(t, githubtest.Routes(t, nil))
		client := github.New("test-token", github.WithBaseURL(server.URL()))

		_, err := Generate(context.Background(), client, file(fstest.MapFS{}), nil)
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Generate() error = %v, want %v", err, fs.ErrNotExist)
		}
		if len(server.Requests()) != 0 {
			t.Errorf("got requests %v, want none", server.RequestLines())
		}
	})

	t.Run("invalid declaration of an unselected repository is rejected before any request", func(t *testing.T) {
		server := githubtest.NewServer(t, githubtest.Routes(t, nil))
		client := github.New("test-token", github.WithBaseURL(server.URL()))
		const src = "repositories:\n  - name: owner/first\n    general:\n      allow_auto_merge: true\n  - name: owner/second\n    general:\n      allow_auto_merge: yes\n"
		files := fstest.MapFS{"reco.yaml": {Data: []byte(src)}}

		_, err := Generate(context.Background(), client, file(files), new(mustReference(t, "owner/first")))
		if err == nil {
			t.Error("Generate() error = nil, want a configuration error")
		}
		if len(server.Requests()) != 0 {
			t.Errorf("got requests %v, want none", server.RequestLines())
		}
	})

	t.Run("target selection failure is returned before any request", func(t *testing.T) {
		server := githubtest.NewServer(t, githubtest.Routes(t, nil))
		client := github.New("test-token", github.WithBaseURL(server.URL()))
		sel := mustReference(t, "owner/other")

		if _, err := Generate(context.Background(), client, file(declared), &sel); err == nil {
			t.Fatal("Generate() error = nil, want an error")
		}
		if len(server.Requests()) != 0 {
			t.Errorf("got requests %v, want none", server.RequestLines())
		}
	})
}

//trace:checks FR-008
func TestGenerate_SecondObservationFailureStopsPlan(t *testing.T) {
	server := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
		"GET /repos/owner/first":  githubtest.OK(githubtest.RepositoryBody{ID: 1}.JSON()),
		"GET /repos/owner/second": {Status: http.StatusForbidden},
	}))
	client := github.New("test-token", github.WithBaseURL(server.URL()))
	const src = "repositories:\n" +
		"  - name: owner/first\n    general:\n      allow_auto_merge: true\n" +
		"  - name: owner/second\n    general:\n      allow_auto_merge: true\n" +
		"  - name: owner/third\n    general:\n      allow_auto_merge: true\n"
	file := configuration.File{Dir: fstest.MapFS{"reco.yaml": {Data: []byte(src)}}, Name: "reco.yaml", Path: "./reco.yaml"}

	got, err := Generate(context.Background(), client, file, nil)
	if !errors.Is(err, github.ErrForbidden) {
		t.Errorf("Generate() error = %v, want %v", err, github.ErrForbidden)
	}
	if err == nil || !strings.HasPrefix(err.Error(), "fetch repository owner/second") {
		t.Errorf("Generate() error = %v, want it to start with %q", err, "fetch repository owner/second")
	}
	if !reflect.DeepEqual(got, Plan{}) {
		t.Errorf("Generate() = %+v, want the zero plan", got)
	}
	if want := []string{"GET /repos/owner/first", "GET /repos/owner/second"}; !slices.Equal(server.RequestLines(), want) {
		t.Errorf("requests = %v, want %v", server.RequestLines(), want)
	}
}

const workflowSource = `repositories:
  - name: owner/repo
    general:
      allow_auto_merge: true
    actions:
      workflow:
        default_workflow_permissions: read
`

//trace:checks FR-004 FR-005
func TestGenerate_ObservedAreas(t *testing.T) {
	file := func(src string) configuration.File {
		return configuration.File{Dir: fstest.MapFS{"reco.yaml": {Data: []byte(src)}}, Name: "reco.yaml", Path: "./reco.yaml"}
	}
	repositoryBody := githubtest.RepositoryBody{ID: 7}.JSON()

	t.Run("general only sends only the repository GET", func(t *testing.T) {
		server := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
			"GET /repos/owner/first": githubtest.OK(repositoryBody),
		}))
		client := github.New("test-token", github.WithBaseURL(server.URL()))

		if _, err := Generate(context.Background(), client, file(twoRepositoriesSource), new(mustReference(t, "owner/first"))); err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		if want := []string{"GET /repos/owner/first"}; !slices.Equal(server.RequestLines(), want) {
			t.Errorf("requests = %v, want %v", server.RequestLines(), want)
		}
	})

	t.Run("declared workflow permissions are observed and reconciled", func(t *testing.T) {
		server := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
			"GET /repos/owner/repo":                              githubtest.OK(repositoryBody),
			"GET /repos/owner/repo/actions/permissions/workflow": githubtest.OK(githubtest.WorkflowPermissionsBody{DefaultWorkflowPermissions: "write", CanApprovePullRequestReviews: true}.JSON()),
		}))
		client := github.New("test-token", github.WithBaseURL(server.URL()))

		got, err := Generate(context.Background(), client, file(workflowSource), nil)
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		if want := []string{"GET /repos/owner/repo", "GET /repos/owner/repo/actions/permissions/workflow"}; !slices.Equal(server.RequestLines(), want) {
			t.Errorf("requests = %v, want %v", server.RequestLines(), want)
		}
		want := []SettingPlan{
			{Key: setting.AllowAutoMerge, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicable},
			{
				Key: setting.DefaultWorkflowPermissions, Observed: github.Observed(setting.PermissionLevelValue(setting.PermissionWrite)), Desired: setting.PermissionLevelValue(setting.PermissionRead),
				Result: Mismatch, Applicability: Applicable,
			},
		}
		if len(got.Repositories) != 1 {
			t.Fatalf("Repositories = %+v, want one repository plan", got.Repositories)
		}
		if !reflect.DeepEqual(got.Repositories[0].Settings, want) {
			t.Errorf("Settings = %+v, want %+v", got.Repositories[0].Settings, want)
		}
	})

	t.Run("each repository reads only the areas of its own management scope", func(t *testing.T) {
		server := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
			"GET /repos/owner/first":                               githubtest.OK(repositoryBody),
			"GET /repos/owner/second":                              githubtest.OK(repositoryBody),
			"GET /repos/owner/second/actions/permissions/workflow": githubtest.OK(githubtest.WorkflowPermissionsBody{DefaultWorkflowPermissions: "read"}.JSON()),
			"GET /repos/owner/third":                               githubtest.OK(repositoryBody),
		}))
		client := github.New("test-token", github.WithBaseURL(server.URL()))
		const src = "repositories:\n" +
			"  - name: owner/first\n    general:\n      allow_auto_merge: true\n" +
			"  - name: owner/second\n    actions:\n      workflow:\n        default_workflow_permissions: read\n" +
			"  - name: owner/third\n    general:\n      allow_auto_merge: true\n"

		if _, err := Generate(context.Background(), client, file(src), nil); err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		want := []string{"GET /repos/owner/first", "GET /repos/owner/second", "GET /repos/owner/second/actions/permissions/workflow", "GET /repos/owner/third"}
		if got := server.RequestLines(); !slices.Equal(got, want) {
			t.Errorf("requests = %v, want %v", got, want)
		}
	})

	const environmentsSource = "repositories:\n  - name: owner/repo\n    environments:\n      - name: production\n        deployment_branch_policy:\n          mode: protected_branches\n      - name: staging\n"

	t.Run("declared environments are observed and planned with their applicability", func(t *testing.T) {
		server := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
			"GET /repos/owner/repo": githubtest.OK(repositoryBody),
			"GET /repos/owner/repo/environments": githubtest.OK(githubtest.EnvironmentsBody{TotalCount: 1, Environments: []githubtest.EnvironmentBody{
				{ID: 2, Name: "Staging"},
			}}.JSON()),
		}))
		client := github.New("test-token", github.WithBaseURL(server.URL()))

		got, err := Generate(context.Background(), client, file(environmentsSource), nil)
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		if want := []string{"GET /repos/owner/repo", "GET /repos/owner/repo/environments"}; !slices.Equal(server.RequestLines(), want) {
			t.Errorf("requests = %v, want %v", server.RequestLines(), want)
		}
		type planned struct {
			unit          string
			result        ReconciliationResult
			applicability Applicability
		}
		var plans []planned
		for _, s := range got.Repositories[0].Settings {
			plans = append(plans, planned{s.Unit().String(), s.Result, s.Applicability})
		}
		want := []planned{
			{"environments.production", Mismatch, Applicable},
			{"environments.production.deployment_branch_policy", NotComparable, Applicable},
			{"environments.staging", Match, ApplicabilityNotJudged},
		}
		if !reflect.DeepEqual(plans, want) {
			t.Errorf("plans = %+v, want %+v", plans, want)
		}
	})

	t.Run("environments observation failure is returned without planning a creation", func(t *testing.T) {
		server := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
			"GET /repos/owner/repo":              githubtest.OK(repositoryBody),
			"GET /repos/owner/repo/environments": {Status: http.StatusForbidden},
		}))
		client := github.New("test-token", github.WithBaseURL(server.URL()))

		got, err := Generate(context.Background(), client, file(environmentsSource), nil)
		if !errors.Is(err, github.ErrForbidden) {
			t.Errorf("Generate() error = %v, want %v", err, github.ErrForbidden)
		}
		if !reflect.DeepEqual(got, Plan{}) {
			t.Errorf("Generate() = %+v, want the zero plan", got)
		}
	})

	t.Run("workflow permissions observation failure is returned without a plan", func(t *testing.T) {
		server := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
			"GET /repos/owner/repo":                              githubtest.OK(repositoryBody),
			"GET /repos/owner/repo/actions/permissions/workflow": {Status: http.StatusForbidden},
		}))
		client := github.New("test-token", github.WithBaseURL(server.URL()))

		got, err := Generate(context.Background(), client, file(workflowSource), nil)
		if !errors.Is(err, github.ErrForbidden) {
			t.Errorf("Generate() error = %v, want %v", err, github.ErrForbidden)
		}
		if !reflect.DeepEqual(got, Plan{}) {
			t.Errorf("Generate() = %+v, want the zero plan", got)
		}
	})
}

const actionsPermissionsSource = `repositories:
  - name: owner/repo
    actions:
      permissions:
        allowed_actions: all
`

//trace:checks FR-004 FR-005 FR-006 FR-007
func TestGenerate_ActionsPermissions(t *testing.T) {
	file := configuration.File{Dir: fstest.MapFS{"reco.yaml": {Data: []byte(actionsPermissionsSource)}}, Name: "reco.yaml", Path: "./reco.yaml"}
	repositoryBody := githubtest.RepositoryBody{ID: 7}.JSON()

	t.Run("declared policy is observed, judged applicable, and carries the side effect", func(t *testing.T) {
		server := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
			"GET /repos/owner/repo":                                      githubtest.OK(repositoryBody),
			"GET /repos/owner/repo/actions/permissions":                  githubtest.OK(githubtest.ActionsPermissionsBody{AllowedActions: "selected"}.JSON()),
			"GET /repos/owner/repo/actions/permissions/selected-actions": githubtest.OK(githubtest.SelectedActionsBody{GitHubOwnedAllowed: true, PatternsAllowed: []string{"example/*"}}.JSON()),
		}))
		client := github.New("test-token", github.WithBaseURL(server.URL()))

		got, err := Generate(context.Background(), client, file, nil)
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		wantPaths := []string{"GET /repos/owner/repo", "GET /repos/owner/repo/actions/permissions", "GET /repos/owner/repo/actions/permissions/selected-actions"}
		if got := server.RequestLines(); !slices.Equal(got, wantPaths) {
			t.Errorf("requests = %v, want %v", got, wantPaths)
		}
		criteria := setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, PatternsAllowed: []string{"example/*"}}
		want := []SettingPlan{{
			Key: setting.AllowedActions, Observed: github.Observed(setting.AllowedActionsPolicyValue(setting.AllowedActionsSelected)),
			Desired: setting.AllowedActionsPolicyValue(setting.AllowedActionsAll), Result: Mismatch, Applicability: Applicable,
			SideEffects: []SideEffect{{Key: setting.SelectedActions, Observed: github.Observed(setting.SelectedActionsCriteriaValue(criteria))}},
		}}
		if len(got.Repositories) != 1 {
			t.Fatalf("Repositories = %+v, want one repository plan", got.Repositories)
		}
		if !reflect.DeepEqual(got.Repositories[0].Settings, want) {
			t.Errorf("Settings = %+v, want %+v", got.Repositories[0].Settings, want)
		}
	})

	t.Run("declared enabled alone is reconciled and judged applicable without a side effect", func(t *testing.T) {
		server := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
			"GET /repos/owner/repo":                     githubtest.OK(repositoryBody),
			"GET /repos/owner/repo/actions/permissions": githubtest.OK(githubtest.ActionsPermissionsBody{AllowedActions: "all"}.JSON()),
		}))
		client := github.New("test-token", github.WithBaseURL(server.URL()))
		enabledFile := configuration.File{Dir: fstest.MapFS{"reco.yaml": {Data: []byte(strings.Replace(actionsPermissionsSource, "allowed_actions: all", "enabled: false", 1))}}, Name: "reco.yaml", Path: "./reco.yaml"}

		got, err := Generate(context.Background(), client, enabledFile, nil)
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		want := []SettingPlan{{
			Key: setting.ActionsEnabled, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(false), Result: Mismatch, Applicability: Applicable,
		}}
		if len(got.Repositories) != 1 {
			t.Fatalf("Repositories = %+v, want one repository plan", got.Repositories)
		}
		if !reflect.DeepEqual(got.Repositories[0].Settings, want) {
			t.Errorf("Settings = %+v, want %+v", got.Repositories[0].Settings, want)
		}
	})

	t.Run("enabled true with a policy on disabled actions plans only the not-comparable policy", func(t *testing.T) {
		server := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
			"GET /repos/owner/repo":                     githubtest.OK(repositoryBody),
			"GET /repos/owner/repo/actions/permissions": githubtest.OK(githubtest.ActionsDisabledBody),
		}))
		client := github.New("test-token", github.WithBaseURL(server.URL()))
		enabledFile := configuration.File{Dir: fstest.MapFS{"reco.yaml": {Data: []byte(strings.Replace(actionsPermissionsSource, "allowed_actions: all", "enabled: true\n        allowed_actions: all", 1))}}, Name: "reco.yaml", Path: "./reco.yaml"}

		got, err := Generate(context.Background(), client, enabledFile, nil)
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		if len(got.Repositories) != 1 {
			t.Fatalf("Repositories = %+v, want one repository plan", got.Repositories)
		}
		if s := got.Repositories[0].Settings; len(s) != 1 || s[0].Key != setting.AllowedActions || s[0].Result != NotComparable {
			t.Errorf("Settings = %+v, want only the not-comparable %s", s, setting.AllowedActions)
		}
	})

	t.Run("sha_pinning_required alone on disabled actions is judged not applicable with the reason", func(t *testing.T) {
		server := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
			"GET /repos/owner/repo":                     githubtest.OK(repositoryBody),
			"GET /repos/owner/repo/actions/permissions": githubtest.OK(githubtest.ActionsDisabledBody),
		}))
		client := github.New("test-token", github.WithBaseURL(server.URL()))
		shaFile := configuration.File{Dir: fstest.MapFS{"reco.yaml": {Data: []byte(strings.Replace(actionsPermissionsSource, "allowed_actions: all", "sha_pinning_required: true", 1))}}, Name: "reco.yaml", Path: "./reco.yaml"}

		got, err := Generate(context.Background(), client, shaFile, nil)
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		want := []SettingPlan{{
			Key: setting.SHAPinningRequired, Observed: github.Unavailable[setting.Value](github.UnavailableActionsDisabled), Desired: setting.BoolValue(true),
			Result: NotComparable, Applicability: NotApplicable, NotApplicableReason: NotApplicableActionsRemainDisabled,
		}}
		if len(got.Repositories) != 1 {
			t.Fatalf("Repositories = %+v, want one repository plan", got.Repositories)
		}
		if !reflect.DeepEqual(got.Repositories[0].Settings, want) {
			t.Errorf("Settings = %+v, want %+v", got.Repositories[0].Settings, want)
		}
	})

	t.Run("sha_pinning_required with enabled true on disabled actions is judged applicable with the enabling change", func(t *testing.T) {
		server := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
			"GET /repos/owner/repo":                     githubtest.OK(repositoryBody),
			"GET /repos/owner/repo/actions/permissions": githubtest.OK(githubtest.ActionsDisabledBody),
		}))
		client := github.New("test-token", github.WithBaseURL(server.URL()))
		shaFile := configuration.File{Dir: fstest.MapFS{"reco.yaml": {Data: []byte(strings.Replace(actionsPermissionsSource, "allowed_actions: all", "enabled: true\n        sha_pinning_required: true", 1))}}, Name: "reco.yaml", Path: "./reco.yaml"}

		got, err := Generate(context.Background(), client, shaFile, nil)
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		want := []SettingPlan{
			{
				Key: setting.ActionsEnabled, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicable,
				ResumedSettings: []setting.Key{setting.AllowedActions, setting.SelectedActions},
			},
			{Key: setting.SHAPinningRequired, Observed: github.Unavailable[setting.Value](github.UnavailableActionsDisabled), Desired: setting.BoolValue(true), Result: NotComparable, Applicability: Applicable},
		}
		if len(got.Repositories) != 1 {
			t.Fatalf("Repositories = %+v, want one repository plan", got.Repositories)
		}
		if !reflect.DeepEqual(got.Repositories[0].Settings, want) {
			t.Errorf("Settings = %+v, want %+v", got.Repositories[0].Settings, want)
		}
	})

	t.Run("actions permissions observation failure is returned without a plan", func(t *testing.T) {
		server := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
			"GET /repos/owner/repo":                     githubtest.OK(repositoryBody),
			"GET /repos/owner/repo/actions/permissions": {Status: http.StatusForbidden},
		}))
		client := github.New("test-token", github.WithBaseURL(server.URL()))

		got, err := Generate(context.Background(), client, file, nil)
		if !errors.Is(err, github.ErrForbidden) {
			t.Errorf("Generate() error = %v, want %v", err, github.ErrForbidden)
		}
		if !reflect.DeepEqual(got, Plan{}) {
			t.Errorf("Generate() = %+v, want the zero plan", got)
		}
	})
}

const actionsAccessSource = `repositories:
  - name: owner/repo
    actions:
      access:
        access_level: none
`

//trace:checks FR-005 FR-006
func TestGenerate_ActionsAccess(t *testing.T) {
	file := configuration.File{Dir: fstest.MapFS{"reco.yaml": {Data: []byte(actionsAccessSource)}}, Name: "reco.yaml", Path: "./reco.yaml"}

	t.Run("private repository is observed and organization is reconciled as an applicable mismatch", func(t *testing.T) {
		server := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
			"GET /repos/owner/repo":                            githubtest.OK(githubtest.RepositoryBody{ID: 7, Private: true}.JSON()),
			"GET /repos/owner/repo/actions/permissions/access": githubtest.OK(githubtest.ActionsAccessBody{AccessLevel: "organization"}.JSON()),
		}))
		client := github.New("test-token", github.WithBaseURL(server.URL()))

		got, err := Generate(context.Background(), client, file, nil)
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		if want := []string{"GET /repos/owner/repo", "GET /repos/owner/repo/actions/permissions/access"}; !slices.Equal(server.RequestLines(), want) {
			t.Errorf("requests = %v, want %v", server.RequestLines(), want)
		}
		want := []SettingPlan{{
			Key: setting.ActionsAccessLevel, Observed: github.Observed(setting.AccessLevelValue(setting.AccessOrganization)), Desired: setting.AccessLevelValue(setting.AccessNone),
			Result: Mismatch, Applicability: Applicable,
		}}
		if len(got.Repositories) != 1 {
			t.Fatalf("Repositories = %+v, want one repository plan", got.Repositories)
		}
		if !reflect.DeepEqual(got.Repositories[0].Settings, want) {
			t.Errorf("Settings = %+v, want %+v", got.Repositories[0].Settings, want)
		}
	})
}
