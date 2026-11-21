package show

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/githubtest"
	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

var errWrite = errors.New("write failed")

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }

//trace:checks FR-004 QR-002
func TestWriteObservationResult(t *testing.T) {
	target := mustReference(t, "owner/repo")
	allPolicy := &github.ActionsPermissions{
		Enabled:            true,
		AllowedActions:     github.Observed(setting.AllowedActionsAll),
		SHAPinningRequired: github.Observed(true),
		SelectedActions:    github.Unavailable[setting.SelectedActionsCriteria](github.UnavailableNotSelected),
	}
	readOnly := &github.WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionRead, CanApprovePullRequestReviews: false}
	notShared := &github.ActionsAccess{AccessLevel: github.Observed(setting.AccessNone)}
	noEnvironments := &github.Environments{}
	const (
		header           = "repositories:\n  - name: owner/repo\n"
		defaultGeneral   = "    general:\n      delete_branch_on_merge: false\n      allow_auto_merge: false\n"
		allPermissions   = "    actions:\n      permissions:\n        enabled: true\n        allowed_actions: all\n        sha_pinning_required: true\n"
		readOnlyWorkflow = "      workflow:\n        default_workflow_permissions: read\n        can_approve_pull_request_reviews: false\n"
		notSharedAccess  = "      access:\n        access_level: none\n"
		noEnvironment    = "    # environments: none\n"
	)
	cases := []struct {
		name     string
		observed github.ObservationResult
		want     string
	}{
		{
			name: "all areas are shown in the configuration structure and the all policy omits the criteria",
			observed: github.ObservationResult{
				RepositoryID:        1,
				General:             github.General{DeleteBranchOnMerge: true, AllowAutoMerge: false},
				ActionsPermissions:  allPolicy,
				WorkflowPermissions: readOnly,
				ActionsAccess:       notShared,
				Environments:        noEnvironments,
			},
			want: header +
				"    general:\n      delete_branch_on_merge: true\n      allow_auto_merge: false\n" +
				allPermissions + readOnlyWorkflow + notSharedAccess + noEnvironment,
		},
		{
			name: "opposite values are shown as observed",
			observed: github.ObservationResult{
				RepositoryID:        1,
				General:             github.General{DeleteBranchOnMerge: false, AllowAutoMerge: true},
				ActionsPermissions:  allPolicy,
				WorkflowPermissions: &github.WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionWrite, CanApprovePullRequestReviews: true},
				ActionsAccess:       &github.ActionsAccess{AccessLevel: github.Observed(setting.AccessUser)},
				Environments:        noEnvironments,
			},
			want: header +
				"    general:\n      delete_branch_on_merge: false\n      allow_auto_merge: true\n" +
				allPermissions +
				"      workflow:\n        default_workflow_permissions: write\n        can_approve_pull_request_reviews: true\n" +
				"      access:\n        access_level: user\n" +
				noEnvironment,
		},
		{
			name: "selected policy shows the three criteria with one quoted pattern per line",
			observed: github.ObservationResult{
				RepositoryID: 1,
				ActionsPermissions: &github.ActionsPermissions{
					Enabled:            true,
					AllowedActions:     github.Observed(setting.AllowedActionsSelected),
					SHAPinningRequired: github.Observed(false),
					SelectedActions: github.Observed(setting.SelectedActionsCriteria{
						GitHubOwnedAllowed: true,
						VerifiedAllowed:    false,
						PatternsAllowed:    []string{"actions/checkout@v4", "example/*"},
					}),
				},
				WorkflowPermissions: readOnly,
				ActionsAccess:       notShared,
				Environments:        noEnvironments,
			},
			want: header + defaultGeneral +
				"    actions:\n      permissions:\n        enabled: true\n        allowed_actions: selected\n        selected_actions:\n" +
				"          github_owned_allowed: true\n          verified_allowed: false\n          patterns_allowed:\n" +
				"            - \"actions/checkout@v4\"\n            - \"example/*\"\n        sha_pinning_required: false\n" +
				readOnlyWorkflow + notSharedAccess + noEnvironment,
		},
		{
			name: "selected policy without patterns shows the empty sequence",
			observed: github.ObservationResult{
				RepositoryID: 1,
				ActionsPermissions: &github.ActionsPermissions{
					Enabled:            true,
					AllowedActions:     github.Observed(setting.AllowedActionsSelected),
					SHAPinningRequired: github.Observed(false),
					SelectedActions:    github.Observed(setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, VerifiedAllowed: true}),
				},
				WorkflowPermissions: readOnly,
				ActionsAccess:       notShared,
				Environments:        noEnvironments,
			},
			want: header + defaultGeneral +
				"    actions:\n      permissions:\n        enabled: true\n        allowed_actions: selected\n        selected_actions:\n" +
				"          github_owned_allowed: true\n          verified_allowed: true\n          patterns_allowed: []\n        sha_pinning_required: false\n" +
				readOnlyWorkflow + notSharedAccess + noEnvironment,
		},
		{
			name: "disabled actions shows only enabled without the policy, the criteria, and sha_pinning_required",
			observed: github.ObservationResult{
				RepositoryID: 1,
				ActionsPermissions: &github.ActionsPermissions{
					AllowedActions:     github.Unavailable[setting.AllowedActionsPolicy](github.UnavailableActionsDisabled),
					SHAPinningRequired: github.Unavailable[bool](github.UnavailableActionsDisabled),
					SelectedActions:    github.Unavailable[setting.SelectedActionsCriteria](github.UnavailableActionsDisabled),
				},
				WorkflowPermissions: readOnly,
				ActionsAccess:       notShared,
				Environments:        noEnvironments,
			},
			want: header + defaultGeneral +
				"    actions:\n      permissions:\n        enabled: false\n" +
				readOnlyWorkflow + notSharedAccess + noEnvironment,
		},
		{
			name: "public repository shows access and the unavailable access level as comments",
			observed: github.ObservationResult{
				RepositoryID:        1,
				ActionsPermissions:  allPolicy,
				WorkflowPermissions: readOnly,
				ActionsAccess:       &github.ActionsAccess{AccessLevel: github.Unavailable[setting.AccessLevel](github.UnavailablePublicRepository)},
				Environments:        noEnvironments,
			},
			want: header + defaultGeneral + allPermissions + readOnlyWorkflow +
				"      # access:\n      #   access_level: unavailable (the repository is public)\n" +
				noEnvironment,
		},
		{
			name: "organization access level is shown as observed",
			observed: github.ObservationResult{
				RepositoryID:        1,
				ActionsPermissions:  allPolicy,
				WorkflowPermissions: readOnly,
				ActionsAccess:       &github.ActionsAccess{AccessLevel: github.Observed(setting.AccessOrganization)},
				Environments:        noEnvironments,
			},
			want: header + defaultGeneral + allPermissions + readOnlyWorkflow +
				"      access:\n        access_level: organization\n" +
				noEnvironment,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := writeObservationResult(&out, target, tc.observed); err != nil {
				t.Fatalf("writeObservationResult() error = %v", err)
			}
			if out.String() != tc.want {
				t.Errorf("writeObservationResult() wrote %q, want %q", out.String(), tc.want)
			}
		})
	}

	t.Run("write failure is returned", func(t *testing.T) {
		observed := github.ObservationResult{ActionsPermissions: allPolicy, WorkflowPermissions: readOnly, ActionsAccess: notShared, Environments: noEnvironments}
		err := writeObservationResult(failingWriter{}, target, observed)
		if !errors.Is(err, errWrite) {
			t.Errorf("writeObservationResult() error = %v, want %v", err, errWrite)
		}
	})

	t.Run("access level unavailable for a reason other than public panics", func(t *testing.T) {
		observed := github.ObservationResult{
			ActionsPermissions:  allPolicy,
			WorkflowPermissions: readOnly,
			ActionsAccess:       &github.ActionsAccess{AccessLevel: github.Unavailable[setting.AccessLevel](github.UnavailableActionsDisabled)},
		}
		defer func() {
			if recover() == nil {
				t.Error("writeObservationResult() did not panic for an access level unavailable while GitHub Actions is disabled")
			}
		}()
		_ = writeObservationResult(&bytes.Buffer{}, target, observed)
	})
}

const (
	repositoryPath          = "/repos/owner/repo"
	actionsPermissionsPath  = "/repos/owner/repo/actions/permissions"
	workflowPermissionsPath = "/repos/owner/repo/actions/permissions/workflow"
	actionsAccessPath       = "/repos/owner/repo/actions/permissions/access"
	environmentsPath        = "/repos/owner/repo/environments"
)

//trace:checks FR-004
func TestRun(t *testing.T) {
	target := mustReference(t, "owner/repo")

	t.Run("all areas are observed with GET only and written to Out", func(t *testing.T) {
		server := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
			"GET " + repositoryPath:          githubtest.OK(githubtest.RepositoryBody{ID: 1, AllowAutoMerge: true, Private: true}.JSON()),
			"GET " + actionsPermissionsPath:  githubtest.OK(githubtest.ActionsPermissionsBody{AllowedActions: "local_only"}.JSON()),
			"GET " + workflowPermissionsPath: githubtest.OK(githubtest.WorkflowPermissionsBody{DefaultWorkflowPermissions: "write", CanApprovePullRequestReviews: true}.JSON()),
			"GET " + actionsAccessPath:       githubtest.OK(githubtest.ActionsAccessBody{AccessLevel: "user"}.JSON()),
			"GET " + environmentsPath: githubtest.OK(githubtest.EnvironmentsBody{TotalCount: 1, Environments: []githubtest.EnvironmentBody{
				{ID: 2, Name: "staging"},
			}}.JSON()),
		}))
		var out bytes.Buffer
		cfg := Config{Token: "test-token", Target: target, Out: &out, GitHubOptions: []github.Option{github.WithBaseURL(server.URL())}}

		if err := Run(context.Background(), cfg); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		want := []string{"GET " + repositoryPath, "GET " + actionsPermissionsPath, "GET " + workflowPermissionsPath, "GET " + actionsAccessPath, "GET " + environmentsPath}
		if got := server.RequestLines(); !slices.Equal(got, want) {
			t.Errorf("got requests %v, want %v", got, want)
		}
		for _, r := range server.Requests() {
			if !strings.Contains(r.Authorization, "test-token") {
				t.Errorf("got Authorization %q, want it to contain the token", r.Authorization)
			}
		}
		for _, observed := range []string{"default_workflow_permissions: write", `- name: "staging"`} {
			if !strings.Contains(out.String(), observed) {
				t.Errorf("got Out %q, want it to contain %q", out.String(), observed)
			}
		}
	})

	failures := []struct {
		name       string
		failedPath string
	}{
		{name: "actions permissions failure is returned as an error instead of omitting the settings", failedPath: actionsPermissionsPath},
		{name: "workflow permissions failure is returned without writing to Out", failedPath: workflowPermissionsPath},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			// 失敗した取得より後の領域は登録しない。失敗のあとに観測を続ければ、stub が登録のない要求として報告する
			routes := map[string]githubtest.Response{
				"GET " + repositoryPath:         githubtest.OK(githubtest.RepositoryBody{ID: 1, AllowAutoMerge: true}.JSON()),
				"GET " + actionsPermissionsPath: githubtest.OK(githubtest.ActionsPermissionsBody{AllowedActions: "all"}.JSON()),
			}
			routes["GET "+tc.failedPath] = githubtest.Response{Status: http.StatusForbidden}
			server := githubtest.NewServer(t, githubtest.Routes(t, routes))
			var out bytes.Buffer
			cfg := Config{Token: "test-token", Target: target, Out: &out, GitHubOptions: []github.Option{github.WithBaseURL(server.URL())}}

			err := Run(context.Background(), cfg)
			if !errors.Is(err, github.ErrForbidden) {
				t.Errorf("Run() error = %v, want %v", err, github.ErrForbidden)
			}
			if out.Len() != 0 {
				t.Errorf("got Out %q, want empty", out.String())
			}
		})
	}
}

func mustReference(t *testing.T, name string) repository.Reference {
	t.Helper()
	reference, err := repository.ParseReference(name)
	if err != nil {
		t.Fatalf("ParseReference(%q) error = %v", name, err)
	}
	return reference
}
