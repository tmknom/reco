package cmd

import (
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/repository"
)

func TestRepoFlag_Set(t *testing.T) {
	t.Run("valid value is parsed into a repository reference", func(t *testing.T) {
		var f repoFlag
		if err := f.Set("owner/repo"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
		want := repository.Reference{Owner: "owner", Repo: "repo"}
		if f.reference != want {
			t.Errorf("got reference %+v, want %+v", f.reference, want)
		}
		if f.String() != "owner/repo" {
			t.Errorf("String() = %q, want %q", f.String(), "owner/repo")
		}
	})

	t.Run("second value is rejected", func(t *testing.T) {
		var f repoFlag
		if err := f.Set("owner/a"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
		err := f.Set("owner/b")
		if err == nil || !strings.Contains(err.Error(), "only once") {
			t.Errorf("Set() error = %v, want it to contain %q", err, "only once")
		}
	})

	t.Run("malformed value is rejected", func(t *testing.T) {
		var f repoFlag
		if err := f.Set("owner"); err == nil {
			t.Errorf("Set() error = nil, want error")
		}
	})

	t.Run("unset flag shows empty", func(t *testing.T) {
		var f repoFlag
		if f.String() != "" {
			t.Errorf("String() = %q, want empty", f.String())
		}
	})
}

func TestRepoFlag_Selection(t *testing.T) {
	t.Run("unset flag selects nothing", func(t *testing.T) {
		var f repoFlag
		if got := f.selection(); got != nil {
			t.Errorf("selection() = %+v, want nil", got)
		}
	})

	t.Run("set flag selects the parsed reference", func(t *testing.T) {
		var f repoFlag
		if err := f.Set("owner/repo"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
		want := repository.Reference{Owner: "owner", Repo: "repo"}
		if got := f.selection(); got == nil || *got != want {
			t.Errorf("selection() = %+v, want %+v", got, want)
		}
	})
}
