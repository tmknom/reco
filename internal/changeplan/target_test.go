package changeplan

import (
	"slices"
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
		Desired:   configuration.DesiredState{General: configuration.General{AllowAutoMerge: &desired}},
	}
}

func referencesOf(declarations []configuration.RepositoryDeclaration) []repository.Reference {
	var references []repository.Reference
	for _, d := range declarations {
		references = append(references, d.Reference)
	}
	return references
}

//trace:checks FR-002
func TestSelectTargets(t *testing.T) {
	// 宣言は名前順と逆に並べ、並べ替える実装を失敗させる。
	first := repositoryDeclaration(t, "owner/second", true)
	second := repositoryDeclaration(t, "owner/repo", false)

	t.Run("no selection returns all declarations in the declared order", func(t *testing.T) {
		d := configuration.Declaration{Repositories: []configuration.RepositoryDeclaration{first, second}}
		got, err := selectTargets(d, nil)
		if err != nil {
			t.Fatalf("selectTargets() error = %v", err)
		}
		if want := []repository.Reference{first.Reference, second.Reference}; !slices.Equal(referencesOf(got), want) {
			t.Errorf("selectTargets() = %v, want %v", referencesOf(got), want)
		}
	})

	t.Run("selection differing only in letter case returns the declared spelling", func(t *testing.T) {
		d := configuration.Declaration{Repositories: []configuration.RepositoryDeclaration{first}}
		sel := mustReference(t, "Owner/SECOND")
		got, err := selectTargets(d, &sel)
		if err != nil {
			t.Fatalf("selectTargets() error = %v", err)
		}
		if want := []repository.Reference{first.Reference}; !slices.Equal(referencesOf(got), want) {
			t.Errorf("selectTargets() = %v, want the declared %v", referencesOf(got), want)
		}
	})

	t.Run("selection matching the middle declaration returns only it", func(t *testing.T) {
		third := repositoryDeclaration(t, "owner/third", true)
		d := configuration.Declaration{Repositories: []configuration.RepositoryDeclaration{first, second, third}}
		sel := second.Reference
		got, err := selectTargets(d, &sel)
		if err != nil {
			t.Fatalf("selectTargets() error = %v", err)
		}
		if want := []repository.Reference{second.Reference}; !slices.Equal(referencesOf(got), want) {
			t.Errorf("selectTargets() = %v, want %v", referencesOf(got), want)
		}
	})

	t.Run("undeclared selection is an error naming it", func(t *testing.T) {
		d := configuration.Declaration{Repositories: []configuration.RepositoryDeclaration{first, second}}
		sel := mustReference(t, "owner/other")
		_, err := selectTargets(d, &sel)
		want := "repository owner/other is not declared"
		if err == nil || err.Error() != want {
			t.Errorf("selectTargets() error = %v, want %q", err, want)
		}
	})
}
