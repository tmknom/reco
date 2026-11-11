package changeplan

import (
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/configuration"
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
