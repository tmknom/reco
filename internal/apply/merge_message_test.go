package apply

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/githubtest"
)

// squashMergeCommitSource は、squash merge のマージメッセージを PR_TITLE と PR_BODY にする設定ファイル。
const squashMergeCommitSource = "repositories:\n  - name: owner/repo\n    general:\n      squash_merge_commit:\n        title: PR_TITLE\n        message: PR_BODY\n"

var squashMergeCommitBody = map[string]any{"squash_merge_commit_title": "PR_TITLE", "squash_merge_commit_message": "PR_BODY"}

//trace:checks FR-010 FR-011 FR-012 INV-001 INV-004 INV-005
func TestRun_MergeMessages(t *testing.T) {
	// current は宣言と違う既定のマージメッセージを持つ応答で、declared は squash merge のマージメッセージだけを宣言した値に変えた応答である。
	current := func(squash, mergeCommit bool) response {
		return okRepositoryBody(githubtest.RepositoryBody{ID: 1, AllowSquashMerge: squash, AllowMergeCommit: mergeCommit})
	}
	declared := func(squash, mergeCommit bool) response {
		return okRepositoryBody(githubtest.RepositoryBody{
			ID: 1, AllowSquashMerge: squash, AllowMergeCommit: mergeCommit, SquashMergeCommitTitle: "PR_TITLE", SquashMergeCommitMessage: "PR_BODY",
		})
	}

	t.Run("changing the squash merge message sends only its title and message and completes", func(t *testing.T) {
		out, _, server, err := runSecurity(t, squashMergeCommitSource, &delayRecorder{},
			[]response{current(true, false)},
			[]response{current(true, false), declared(true, false), declared(true, false)})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if want := []string{getRepository, getRepository, patchRepository, getRepository}; !slices.Equal(server.RequestLines(), want) {
			t.Fatalf("requests = %v, want %v", server.RequestLines(), want)
		}
		if got := server.Bodies()[2]; !reflect.DeepEqual(got, squashMergeCommitBody) {
			t.Errorf("PATCH body = %v, want %v", got, squashMergeCommitBody)
		}
		if !strings.Contains(out.String(), "owner/repo: updated") {
			t.Errorf("Out = %q, want the repository updated", out.String())
		}
	})

	t.Run("enabling squash merge sends the merge methods before the message", func(t *testing.T) {
		out, _, server, err := runSecurity(t, squashMergeCommitSource+"      allow_squash_merge: true\n", &delayRecorder{},
			[]response{current(false, true)},
			[]response{current(false, true), current(true, true), current(true, true)},
			[]response{current(true, true), declared(true, true), declared(true, true)})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		bodies := server.Bodies()
		if got, want := []map[string]any{bodies[2], bodies[5]}, []map[string]any{{"allow_squash_merge": true}, squashMergeCommitBody}; !reflect.DeepEqual(got, want) {
			t.Errorf("PATCH bodies = %v, want %v", got, want)
		}
		if !strings.Contains(out.String(), "owner/repo: updated") {
			t.Errorf("Out = %q, want the repository updated", out.String())
		}
	})

	t.Run("disabling squash merge sends the message before the merge methods", func(t *testing.T) {
		out, _, server, err := runSecurity(t, squashMergeCommitSource+"      allow_squash_merge: false\n", &delayRecorder{},
			[]response{current(true, true)},
			[]response{current(true, true), declared(true, true), declared(true, true)},
			[]response{declared(true, true), declared(false, true), declared(false, true)})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		bodies := server.Bodies()
		if got, want := []map[string]any{bodies[2], bodies[5]}, []map[string]any{squashMergeCommitBody, {"allow_squash_merge": false}}; !reflect.DeepEqual(got, want) {
			t.Errorf("PATCH bodies = %v, want %v", got, want)
		}
		if !strings.Contains(out.String(), "owner/repo: updated") {
			t.Errorf("Out = %q, want the repository updated", out.String())
		}
	})

	t.Run("the merge commit message changed by the squash merge message update is an unexpected outside change", func(t *testing.T) {
		withMergeCommitChanged := okRepositoryBody(githubtest.RepositoryBody{
			ID: 1, AllowSquashMerge: true, SquashMergeCommitTitle: "PR_TITLE", SquashMergeCommitMessage: "PR_BODY", MergeCommitTitle: "PR_TITLE", MergeCommitMessage: "BLANK",
		})
		out, _, _, err := runSecurity(t, squashMergeCommitSource, &delayRecorder{},
			[]response{current(true, false)},
			[]response{current(true, false), withMergeCommitChanged, withMergeCommitChanged})
		if err == nil {
			t.Fatal("Run() error = nil, want the unexpected outside change")
		}
		want := "Other settings: general.merge_commit changed title: MERGE_MESSAGE, message: PR_TITLE -> title: PR_TITLE, message: BLANK"
		if !strings.Contains(out.String(), "owner/repo: failed") || !strings.Contains(out.String(), want) {
			t.Errorf("Out = %q, want the failed change and %q", out.String(), want)
		}
	})

	t.Run("a merge method changed by the squash merge message update is an unexpected outside change", func(t *testing.T) {
		out, _, _, err := runSecurity(t, squashMergeCommitSource, &delayRecorder{},
			[]response{current(true, false)},
			[]response{current(true, false), declared(true, true), declared(true, true)})
		if err == nil {
			t.Fatal("Run() error = nil, want the unexpected outside change")
		}
		if !strings.Contains(out.String(), "owner/repo: failed") || !strings.Contains(out.String(), "Other settings: general.allow_merge_commit changed false -> true") {
			t.Errorf("Out = %q, want the changed allow_merge_commit and the failed change", out.String())
		}
	})
}
