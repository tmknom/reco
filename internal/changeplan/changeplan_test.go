package changeplan

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

func TestRepositoryPlan_Count(t *testing.T) {
	r := RepositoryPlan{Settings: []SettingPlan{{Result: Mismatch}, {Result: Match}, {Result: Mismatch}, {Result: NotComparable}}}
	if got := r.Count(Mismatch); got != 2 {
		t.Errorf("Count(Mismatch) = %d, want 2", got)
	}
}

//trace:checks FR-007
func TestSettingPlan_EnablesActions(t *testing.T) {
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
			if got := tc.plan.EnablesActions(); got != tc.want {
				t.Errorf("EnablesActions() = %t, want %t", got, tc.want)
			}
		})
	}
}

const twoRepositoriesSource = `repositories:
  - name: owner/first
    general:
      allow_auto_merge: true
  - name: owner/second
    general:
      allow_auto_merge: true
      delete_branch_on_merge: true
`

//trace:checks FR-001
//trace:checks FR-002
//trace:checks FR-005
func TestGenerate(t *testing.T) {
	file := func(files fstest.MapFS) ConfigurationFile {
		return ConfigurationFile{Dir: files, Name: "reco.yaml", Path: "./reco.yaml"}
	}
	declared := fstest.MapFS{"reco.yaml": {Data: []byte(twoRepositoriesSource)}}

	t.Run("no selection observes all declarations in the declared order", func(t *testing.T) {
		var paths []string
		baseURL := newGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
			paths = append(paths, r.Method+" "+r.URL.Path)
			id := "1"
			if r.URL.Path == "/repos/owner/second" {
				id = "2"
			}
			_, _ = w.Write([]byte(`{"id": ` + id + `, "allow_auto_merge": true, "delete_branch_on_merge": true}`))
		})
		client := github.New("test-token", github.WithBaseURL(baseURL))

		got, err := Generate(context.Background(), client, file(declared), nil)
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		if want := []string{"GET /repos/owner/first", "GET /repos/owner/second"}; !slices.Equal(paths, want) {
			t.Errorf("requests = %v, want %v", paths, want)
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
		var paths []string
		baseURL := newGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
			paths = append(paths, r.Method+" "+r.URL.Path)
			_, _ = w.Write([]byte(`{"id": 7, "allow_auto_merge": false, "delete_branch_on_merge": false}`))
		})
		client := github.New("test-token", github.WithBaseURL(baseURL))
		sel := mustReference(t, "Owner/Second")

		got, err := Generate(context.Background(), client, file(declared), &sel)
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		if want := []string{"GET /repos/owner/second"}; !slices.Equal(paths, want) {
			t.Errorf("requests = %v, want %v", paths, want)
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

	t.Run("missing configuration file is reported with the user path before any request", func(t *testing.T) {
		requested := false
		baseURL := newGitHubServer(t, func(http.ResponseWriter, *http.Request) { requested = true })
		client := github.New("test-token", github.WithBaseURL(baseURL))

		_, err := Generate(context.Background(), client, file(fstest.MapFS{}), nil)
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Generate() error = %v, want %v", err, fs.ErrNotExist)
		}
		if err == nil || !strings.Contains(err.Error(), "./reco.yaml") {
			t.Errorf("Generate() error = %v, want it to contain the configuration path", err)
		}
		if requested {
			t.Error("GitHub received a request, want none")
		}
	})

	t.Run("invalid declaration of an unselected repository is rejected before any request", func(t *testing.T) {
		requested := false
		baseURL := newGitHubServer(t, func(http.ResponseWriter, *http.Request) { requested = true })
		client := github.New("test-token", github.WithBaseURL(baseURL))
		const src = "repositories:\n  - name: owner/first\n    general:\n      allow_auto_merge: true\n  - name: owner/second\n    general:\n      allow_auto_merge: yes\n"
		files := fstest.MapFS{"reco.yaml": {Data: []byte(src)}}

		_, err := Generate(context.Background(), client, file(files), new(mustReference(t, "owner/first")))
		if err == nil || !strings.Contains(err.Error(), "./reco.yaml") {
			t.Errorf("Generate() error = %v, want a configuration error naming the configuration path", err)
		}
		if requested {
			t.Error("GitHub received a request, want none")
		}
	})

	t.Run("target selection failure is returned before any request", func(t *testing.T) {
		requested := false
		baseURL := newGitHubServer(t, func(http.ResponseWriter, *http.Request) { requested = true })
		client := github.New("test-token", github.WithBaseURL(baseURL))
		sel := mustReference(t, "owner/other")

		if _, err := Generate(context.Background(), client, file(declared), &sel); err == nil {
			t.Fatal("Generate() error = nil, want an error")
		}
		if requested {
			t.Error("GitHub received a request, want none")
		}
	})

	//trace:checks FR-008
	t.Run("observation failure of the second repository stops before the third and returns no plan", func(t *testing.T) {
		var paths []string
		baseURL := newGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
			paths = append(paths, r.Method+" "+r.URL.Path)
			if r.URL.Path == "/repos/owner/second" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			_, _ = w.Write([]byte(`{"id": 1, "allow_auto_merge": false, "delete_branch_on_merge": false}`))
		})
		client := github.New("test-token", github.WithBaseURL(baseURL))
		const src = "repositories:\n" +
			"  - name: owner/first\n    general:\n      allow_auto_merge: true\n" +
			"  - name: owner/second\n    general:\n      allow_auto_merge: true\n" +
			"  - name: owner/third\n    general:\n      allow_auto_merge: true\n"
		files := fstest.MapFS{"reco.yaml": {Data: []byte(src)}}

		got, err := Generate(context.Background(), client, file(files), nil)
		if !errors.Is(err, github.ErrForbidden) {
			t.Errorf("Generate() error = %v, want %v", err, github.ErrForbidden)
		}
		if err == nil || !strings.HasPrefix(err.Error(), "fetch repository owner/second") {
			t.Errorf("Generate() error = %v, want it to start with %q", err, "fetch repository owner/second")
		}
		if !reflect.DeepEqual(got, Plan{}) {
			t.Errorf("Generate() = %+v, want the zero plan", got)
		}
		if want := []string{"GET /repos/owner/first", "GET /repos/owner/second"}; !slices.Equal(paths, want) {
			t.Errorf("requests = %v, want %v", paths, want)
		}
	})
}

const workflowSource = `repositories:
  - name: owner/repo
    general:
      allow_auto_merge: true
    actions:
      workflow:
        default_workflow_permissions: read
`

//trace:checks FR-004
//trace:checks FR-005
func TestGenerate_ObservedAreas(t *testing.T) {
	file := func(src string) ConfigurationFile {
		return ConfigurationFile{Dir: fstest.MapFS{"reco.yaml": {Data: []byte(src)}}, Name: "reco.yaml", Path: "./reco.yaml"}
	}
	const repositoryBody = `{"id": 7, "allow_auto_merge": false, "delete_branch_on_merge": false}`

	t.Run("general only sends only the repository GET", func(t *testing.T) {
		var paths []string
		baseURL := newGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
			paths = append(paths, r.Method+" "+r.URL.Path)
			_, _ = w.Write([]byte(repositoryBody))
		})
		client := github.New("test-token", github.WithBaseURL(baseURL))

		if _, err := Generate(context.Background(), client, file(twoRepositoriesSource), new(mustReference(t, "owner/first"))); err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		if want := []string{"GET /repos/owner/first"}; !slices.Equal(paths, want) {
			t.Errorf("requests = %v, want %v", paths, want)
		}
	})

	t.Run("declared workflow permissions are observed and reconciled", func(t *testing.T) {
		var paths []string
		baseURL := newGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
			paths = append(paths, r.Method+" "+r.URL.Path)
			if strings.HasSuffix(r.URL.Path, "/actions/permissions/workflow") {
				_, _ = w.Write([]byte(`{"default_workflow_permissions": "write", "can_approve_pull_request_reviews": true}`))
				return
			}
			_, _ = w.Write([]byte(repositoryBody))
		})
		client := github.New("test-token", github.WithBaseURL(baseURL))

		got, err := Generate(context.Background(), client, file(workflowSource), nil)
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		if want := []string{"GET /repos/owner/repo", "GET /repos/owner/repo/actions/permissions/workflow"}; !slices.Equal(paths, want) {
			t.Errorf("requests = %v, want %v", paths, want)
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
		var paths []string
		baseURL := newGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
			paths = append(paths, r.Method+" "+r.URL.Path)
			if strings.HasSuffix(r.URL.Path, "/actions/permissions/workflow") {
				_, _ = w.Write([]byte(`{"default_workflow_permissions": "read", "can_approve_pull_request_reviews": false}`))
				return
			}
			_, _ = w.Write([]byte(repositoryBody))
		})
		client := github.New("test-token", github.WithBaseURL(baseURL))
		const src = "repositories:\n" +
			"  - name: owner/first\n    general:\n      allow_auto_merge: true\n" +
			"  - name: owner/second\n    actions:\n      workflow:\n        default_workflow_permissions: read\n" +
			"  - name: owner/third\n    general:\n      allow_auto_merge: true\n"

		if _, err := Generate(context.Background(), client, file(src), nil); err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		want := []string{"GET /repos/owner/first", "GET /repos/owner/second", "GET /repos/owner/second/actions/permissions/workflow", "GET /repos/owner/third"}
		if !slices.Equal(paths, want) {
			t.Errorf("requests = %v, want %v", paths, want)
		}
	})

	t.Run("workflow permissions observation failure is returned without a plan", func(t *testing.T) {
		baseURL := newGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/actions/permissions/workflow") {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			_, _ = w.Write([]byte(repositoryBody))
		})
		client := github.New("test-token", github.WithBaseURL(baseURL))

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

//trace:checks FR-004
//trace:checks FR-005
//trace:checks FR-006
//trace:checks FR-007
func TestGenerate_ActionsPermissions(t *testing.T) {
	file := ConfigurationFile{Dir: fstest.MapFS{"reco.yaml": {Data: []byte(actionsPermissionsSource)}}, Name: "reco.yaml", Path: "./reco.yaml"}
	const repositoryBody = `{"id": 7, "allow_auto_merge": false, "delete_branch_on_merge": false}`

	t.Run("declared policy is observed, judged applicable, and carries the side effect", func(t *testing.T) {
		var paths []string
		baseURL := newGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
			paths = append(paths, r.Method+" "+r.URL.Path)
			switch {
			case strings.HasSuffix(r.URL.Path, "/actions/permissions/selected-actions"):
				_, _ = w.Write([]byte(`{"github_owned_allowed": true, "verified_allowed": false, "patterns_allowed": ["example/*"]}`))
			case strings.HasSuffix(r.URL.Path, "/actions/permissions"):
				_, _ = w.Write([]byte(`{"enabled": true, "allowed_actions": "selected", "sha_pinning_required": false}`))
			default:
				_, _ = w.Write([]byte(repositoryBody))
			}
		})
		client := github.New("test-token", github.WithBaseURL(baseURL))

		got, err := Generate(context.Background(), client, file, nil)
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		wantPaths := []string{"GET /repos/owner/repo", "GET /repos/owner/repo/actions/permissions", "GET /repos/owner/repo/actions/permissions/selected-actions"}
		if !slices.Equal(paths, wantPaths) {
			t.Errorf("requests = %v, want %v", paths, wantPaths)
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

	t.Run("actions permissions observation failure is returned without a plan", func(t *testing.T) {
		baseURL := newGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/actions/permissions") {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			_, _ = w.Write([]byte(repositoryBody))
		})
		client := github.New("test-token", github.WithBaseURL(baseURL))

		got, err := Generate(context.Background(), client, file, nil)
		if !errors.Is(err, github.ErrForbidden) {
			t.Errorf("Generate() error = %v, want %v", err, github.ErrForbidden)
		}
		if !reflect.DeepEqual(got, Plan{}) {
			t.Errorf("Generate() = %+v, want the zero plan", got)
		}
	})
}

func newGitHubServer(t *testing.T, handler http.HandlerFunc) *url.URL {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	return u
}
