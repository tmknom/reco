package reco

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
)

func repositoryDeclaration(name string, desired bool) configuration.RepositoryDeclaration {
	owner, repo, _ := strings.Cut(name, "/")
	return configuration.RepositoryDeclaration{
		Reference: repository.Reference{Owner: owner, Repo: repo},
		General:   configuration.General{AllowAutoMerge: &desired},
	}
}

//trace:checks FR-002
func TestSelectTarget(t *testing.T) {
	first := repositoryDeclaration("owner/repo", true)
	second := repositoryDeclaration("owner/second", false)

	t.Run("no selection and one declaration selects it", func(t *testing.T) {
		d := configuration.Declaration{Repositories: []configuration.RepositoryDeclaration{first}}
		got, err := selectTarget(d, nil)
		if err != nil {
			t.Fatalf("selectTarget() error = %v", err)
		}
		if got.Reference != first.Reference {
			t.Errorf("selectTarget() = %v, want %v", got.Reference, first.Reference)
		}
	})

	t.Run("no selection and two declarations is an error", func(t *testing.T) {
		d := configuration.Declaration{Repositories: []configuration.RepositoryDeclaration{first, second}}
		_, err := selectTarget(d, nil)
		want := "2 repositories are declared; select one repository (only one can be processed at a time)"
		if err == nil || err.Error() != want {
			t.Errorf("selectTarget() error = %v, want %q", err, want)
		}
	})

	t.Run("selection differing only in letter case returns the declared spelling", func(t *testing.T) {
		d := configuration.Declaration{Repositories: []configuration.RepositoryDeclaration{first}}
		sel := repository.Reference{Owner: "Owner", Repo: "REPO"}
		got, err := selectTarget(d, &sel)
		if err != nil {
			t.Fatalf("selectTarget() error = %v", err)
		}
		if got.Reference != first.Reference {
			t.Errorf("selectTarget() = %v, want the declared %v", got.Reference, first.Reference)
		}
	})

	t.Run("selection matching the second declaration selects it", func(t *testing.T) {
		d := configuration.Declaration{Repositories: []configuration.RepositoryDeclaration{first, second}}
		sel := second.Reference
		got, err := selectTarget(d, &sel)
		if err != nil {
			t.Fatalf("selectTarget() error = %v", err)
		}
		if got.Reference != second.Reference {
			t.Errorf("selectTarget() = %v, want %v", got.Reference, second.Reference)
		}
	})

	t.Run("undeclared selection is an error naming it", func(t *testing.T) {
		d := configuration.Declaration{Repositories: []configuration.RepositoryDeclaration{first, second}}
		sel := repository.Reference{Owner: "owner", Repo: "other"}
		_, err := selectTarget(d, &sel)
		want := "repository owner/other is not declared"
		if err == nil || err.Error() != want {
			t.Errorf("selectTarget() error = %v, want %q", err, want)
		}
	})
}

//trace:checks FR-006
func TestIsApplicable(t *testing.T) {
	cases := []struct {
		name string
		in   reconciliationResult
		want bool
	}{
		{name: "mismatch is applicable", in: mismatch, want: true},
		{name: "match is not judged", in: match, want: false},
		{name: "not comparable is not applicable", in: notComparable, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isApplicable(tc.in); got != tc.want {
				t.Errorf("isApplicable(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestChangePlan_count(t *testing.T) {
	p := changePlan{settings: []settingPlan{{result: mismatch}, {result: match}, {result: mismatch}, {result: notComparable}}}
	if got := p.count(mismatch); got != 2 {
		t.Errorf("count(mismatch) = %d, want 2", got)
	}
}

func TestPlanChanges(t *testing.T) {
	d := configuration.Declaration{Repositories: []configuration.RepositoryDeclaration{
		repositoryDeclaration("owner/first", true),
		repositoryDeclaration("owner/second", true),
	}}

	t.Run("selected declaration is observed and reconciled", func(t *testing.T) {
		var gotPath string
		baseURL := newGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			_, _ = w.Write([]byte(`{"id": 7, "allow_auto_merge": false, "delete_branch_on_merge": false}`))
		})
		client := github.New("test-token", github.WithBaseURL(baseURL))
		sel := repository.Reference{Owner: "Owner", Repo: "Second"}

		got, err := planChanges(context.Background(), client, d, &sel)
		if err != nil {
			t.Fatalf("planChanges() error = %v", err)
		}
		if !strings.Contains(gotPath, "owner/second") {
			t.Errorf("got request path %q, want it to contain the selected declaration", gotPath)
		}
		if got.target != d.Repositories[1].Reference {
			t.Errorf("target = %v, want %v", got.target, d.Repositories[1].Reference)
		}
		if got.repositoryID != 7 {
			t.Errorf("repositoryID = %d, want 7", got.repositoryID)
		}
		want := settingPlan{setting: "general.allow_auto_merge", current: false, desired: true, result: mismatch, applicable: true}
		if len(got.settings) != 1 || got.settings[0] != want {
			t.Errorf("settings = %+v, want [%+v]", got.settings, want)
		}
	})

	t.Run("observation failure is returned unwrapped by this stage", func(t *testing.T) {
		baseURL := newGitHubServer(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		})
		client := github.New("test-token", github.WithBaseURL(baseURL))
		sel := repository.Reference{Owner: "owner", Repo: "first"}

		_, err := planChanges(context.Background(), client, d, &sel)
		if !errors.Is(err, github.ErrUnauthorized) {
			t.Errorf("planChanges() error = %v, want %v", err, github.ErrUnauthorized)
		}
		if err == nil || !strings.HasPrefix(err.Error(), "fetch repository") {
			t.Errorf("planChanges() error = %v, want it to start with %q", err, "fetch repository")
		}
	})
}
