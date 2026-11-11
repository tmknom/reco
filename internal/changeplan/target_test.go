package changeplan

import (
	"testing"

	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/repository"
)

func mustReference(t *testing.T, name string) repository.Reference {
	t.Helper()
	reference, err := repository.ParseReference(name)
	if err != nil {
		t.Fatalf("ParseReference(%q) error = %v", name, err)
	}
	return reference
}

func repositoryDeclaration(t *testing.T, name string, desired bool) configuration.RepositoryDeclaration {
	t.Helper()
	return configuration.RepositoryDeclaration{
		Reference: mustReference(t, name),
		General:   configuration.General{AllowAutoMerge: &desired},
	}
}

//trace:checks FR-002
func TestSelectTarget(t *testing.T) {
	first := repositoryDeclaration(t, "owner/repo", true)
	second := repositoryDeclaration(t, "owner/second", false)

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
		sel := mustReference(t, "Owner/REPO")
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
		sel := mustReference(t, "owner/other")
		_, err := selectTarget(d, &sel)
		want := "repository owner/other is not declared"
		if err == nil || err.Error() != want {
			t.Errorf("selectTarget() error = %v, want %q", err, want)
		}
	})
}
