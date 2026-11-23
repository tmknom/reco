package apply

import (
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tmknom/reco/internal/githubtest"
)

// mergeMethodSwapSource は、有効なマージ方式を merge commit から squash merge に入れ替える設定ファイル。
const mergeMethodSwapSource = "repositories:\n  - name: owner/repo\n    general:\n      allow_squash_merge: true\n      allow_merge_commit: false\n"

// mergeMethods は、3つのマージ方式の値だけを指定した、ほかの設定が無効のリポジトリの取得と更新の正常応答。
// githubtest.RepositoryBody のゼロ値は GitHub が取り得ない全無効の組み合わせのため、3方式を必ず指定する。
func mergeMethods(squash, mergeCommit, rebase bool) response {
	return okRepositoryBody(githubtest.RepositoryBody{ID: 1, AllowSquashMerge: squash, AllowMergeCommit: mergeCommit, AllowRebaseMerge: rebase})
}

//trace:checks FR-010 FR-011 FR-012 FR-013 INV-001 INV-004 INV-005
func TestRun_MergeMethods(t *testing.T) {
	t.Run("swapping the enabled merge method sends one PATCH with only the two fields and completes", func(t *testing.T) {
		out, _, server, err := runSecurity(t, mergeMethodSwapSource, &delayRecorder{},
			[]response{mergeMethods(false, true, false)},
			[]response{mergeMethods(false, true, false), mergeMethods(true, false, false), mergeMethods(true, false, false)})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if want := []string{getRepository, getRepository, patchRepository, getRepository}; !slices.Equal(server.RequestLines(), want) {
			t.Fatalf("requests = %v, want %v", server.RequestLines(), want)
		}
		if got, want := server.Bodies()[2], map[string]any{"allow_squash_merge": true, "allow_merge_commit": false}; !reflect.DeepEqual(got, want) {
			t.Errorf("PATCH body = %v, want %v", got, want)
		}
		if !strings.Contains(out.String(), "owner/repo: updated") {
			t.Errorf("Out = %q, want the repository updated", out.String())
		}
	})

	t.Run("only one bundled merge method not matching fails the whole change and names it", func(t *testing.T) {
		bothEnabled := mergeMethods(true, true, false)
		out, _, _, err := runSecurity(t, mergeMethodSwapSource, &delayRecorder{},
			[]response{mergeMethods(false, true, false)},
			[]response{mergeMethods(false, true, false), mergeMethods(true, false, false)},
			slices.Repeat([]response{bothEnabled}, 1+len(reobservationDelays)))
		if err == nil {
			t.Fatal("Run() error = nil, want the mismatch")
		}
		want := "  owner/repo: failed\n    Update request: succeeded\n    Desired state: does not match (general.allow_merge_commit)\n"
		if !strings.Contains(out.String(), want) {
			t.Errorf("Out = %q, want it to contain %q", out.String(), want)
		}
	})

	// 宣言していないマージ方式は変更前提に含めず、GitHub が全無効の要求を 422 で拒否して値を変えないことに依拠する (General の設計の適用可能性と副作用)
	t.Run("undeclared merge method disabled after approval still sends the PATCH and GitHub's 422 fails the change", func(t *testing.T) {
		source := "repositories:\n  - name: owner/repo\n    general:\n      allow_squash_merge: false\n      allow_merge_commit: false\n"
		out, _, server, err := runSecurity(t, source, &delayRecorder{},
			[]response{mergeMethods(true, true, true)},
			[]response{mergeMethods(true, true, false), {status: http.StatusUnprocessableEntity}})
		if err == nil {
			t.Fatal("Run() error = nil, want the rejected update")
		}
		if want := []string{getRepository, getRepository, patchRepository}; !slices.Equal(server.RequestLines(), want) {
			t.Errorf("requests = %v, want %v without a read after the rejection", server.RequestLines(), want)
		}
		want := "  owner/repo: failed\n    Update request: rejected\n    Desired state: not checked\n"
		if !strings.Contains(out.String(), want) {
			t.Errorf("Out = %q, want it to contain %q", out.String(), want)
		}
	})

	t.Run("declared merge method changed after approval sends no PATCH", func(t *testing.T) {
		out, _, server, err := runSecurity(t, mergeMethodSwapSource, &delayRecorder{},
			[]response{mergeMethods(false, true, false)},
			[]response{mergeMethods(false, false, true)})
		if err == nil {
			t.Fatal("Run() error = nil, want the precondition failure")
		}
		if want := []string{getRepository, getRepository}; !slices.Equal(server.RequestLines(), want) {
			t.Errorf("requests = %v, want %v without the PATCH", server.RequestLines(), want)
		}
		if !strings.Contains(out.String(), "Precondition: general.allow_merge_commit changed since approval") {
			t.Errorf("Out = %q, want the changed merge commit", out.String())
		}
	})

	t.Run("merge methods and allow_update_branch are separate changes run in order with the update interval", func(t *testing.T) {
		delays := &delayRecorder{}
		squashOnlyWithUpdateBranch := okRepositoryBody(githubtest.RepositoryBody{ID: 1, AllowSquashMerge: true, AllowUpdateBranch: true})
		out, _, server, err := runSecurity(t, mergeMethodSwapSource+"      allow_update_branch: true\n", delays,
			[]response{mergeMethods(false, true, false)},
			[]response{mergeMethods(false, true, false), mergeMethods(true, false, false), mergeMethods(true, false, false)},
			[]response{mergeMethods(true, false, false), squashOnlyWithUpdateBranch, squashOnlyWithUpdateBranch})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		want := []string{getRepository, getRepository, patchRepository, getRepository, getRepository, patchRepository, getRepository}
		if !slices.Equal(server.RequestLines(), want) {
			t.Fatalf("requests = %v, want %v", server.RequestLines(), want)
		}
		bodies := server.Bodies()
		if got, want := []map[string]any{bodies[2], bodies[5]}, []map[string]any{
			{"allow_squash_merge": true, "allow_merge_commit": false}, {"allow_update_branch": true},
		}; !reflect.DeepEqual(got, want) {
			t.Errorf("PATCH bodies = %v, want %v", got, want)
		}
		if wantDelays := []time.Duration{time.Second}; !slices.Equal(delays.delays, wantDelays) {
			t.Errorf("delays = %v, want %v", delays.delays, wantDelays)
		}
		if !strings.Contains(out.String(), "owner/repo: updated") {
			t.Errorf("Out = %q, want the repository updated", out.String())
		}
	})

	t.Run("allow_auto_merge changed by the bundled update is an unexpected outside change", func(t *testing.T) {
		withAutoMerge := okRepositoryBody(githubtest.RepositoryBody{ID: 1, AllowSquashMerge: true, AllowAutoMerge: true})
		out, _, _, err := runSecurity(t, mergeMethodSwapSource, &delayRecorder{},
			[]response{mergeMethods(false, true, false)},
			[]response{mergeMethods(false, true, false), withAutoMerge, withAutoMerge})
		if err == nil {
			t.Fatal("Run() error = nil, want the unexpected outside change")
		}
		if !strings.Contains(out.String(), "owner/repo: failed") || !strings.Contains(out.String(), "Other settings: general.allow_auto_merge changed false -> true") {
			t.Errorf("Out = %q, want the changed allow_auto_merge and the failed change", out.String())
		}
	})
}
