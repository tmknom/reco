package apply

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/githubtest"
	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

func mustReference(t *testing.T, name string) repository.Reference {
	t.Helper()
	reference, err := repository.ParseReference(name)
	if err != nil {
		t.Fatalf("ParseReference(%q) error = %v", name, err)
	}
	return reference
}

var errWrite = errors.New("write failed")

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }

// failAfterWriter は、n 回の書き込みに成功したあとの書き込みを失敗させる。
type failAfterWriter struct {
	n   int
	buf bytes.Buffer
}

func (w *failAfterWriter) Write(p []byte) (int, error) {
	if w.n == 0 {
		return 0, errWrite
	}
	w.n--
	return w.buf.Write(p)
}

// response は stub が順に返す応答。disconnect が真なら本文を返さずに接続を切る。
type response struct {
	status     int
	body       string
	location   string
	disconnect bool
}

// newSequenceServer は、要求ごとに responses を順に返す。足りなければ 500 を返す。
func newSequenceServer(t *testing.T, responses ...response) (*url.URL, *githubtest.Server) {
	t.Helper()
	var mu sync.Mutex
	next := 0
	server := githubtest.NewServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		resp := response{status: http.StatusInternalServerError}
		if next < len(responses) {
			resp = responses[next]
			next++
		}
		mu.Unlock()

		if resp.disconnect {
			conn, _, err := http.NewResponseController(w).Hijack()
			if err != nil {
				t.Errorf("hijack: %v", err)
				return
			}
			_ = conn.Close()
			return
		}
		if resp.location != "" {
			w.Header().Set("Location", resp.location)
		}
		w.WriteHeader(resp.status)
		_, _ = w.Write([]byte(resp.body))
	}))
	return server.URL(), server
}

func okRepository(id int64, allowAutoMerge, deleteBranchOnMerge bool) response {
	return response{status: http.StatusOK, body: githubtest.RepositoryBody{ID: id, AllowAutoMerge: allowAutoMerge, DeleteBranchOnMerge: deleteBranchOnMerge}.JSON()}
}

func okWorkflowPermissions(defaultWorkflowPermissions string, canApprovePullRequestReviews bool) response {
	return response{status: http.StatusOK, body: githubtest.WorkflowPermissionsBody{
		DefaultWorkflowPermissions: defaultWorkflowPermissions, CanApprovePullRequestReviews: canApprovePullRequestReviews}.JSON()}
}

// okAllowedActions は、Actions が有効で、Actions 許可ポリシーが policy の Actions permissions の取得の正常応答。
func okAllowedActions(policy string, shaPinningRequired bool) response {
	return response{status: http.StatusOK, body: githubtest.ActionsPermissionsBody{AllowedActions: policy, SHAPinningRequired: shaPinningRequired}.JSON()}
}

// okActionsDisabled は、Actions が無効のリポジトリの Actions permissions の取得の正常応答。
// GitHub は無効のときも sha_pinning_required を返すが、保存された値を示さない。
func okActionsDisabled() response {
	return response{status: http.StatusOK, body: githubtest.ActionsDisabledBody}
}

// okSelectedActions は、Selected actions の取得の正常応答。
func okSelectedActions(githubOwnedAllowed, verifiedAllowed bool, patternsAllowed ...string) response {
	return response{status: http.StatusOK, body: githubtest.SelectedActionsBody{
		GitHubOwnedAllowed: githubOwnedAllowed, VerifiedAllowed: verifiedAllowed, PatternsAllowed: patternsAllowed}.JSON()}
}

// actionsPermissionsConfigSource は、allowed_actions だけを policy で宣言する。
func actionsPermissionsConfigSource(policy string) string {
	return "repositories:\n  - name: owner/repo\n    actions:\n      permissions:\n        allowed_actions: " + policy + "\n"
}

func newClient(baseURL *url.URL) *github.Client {
	return github.New("test-token", github.WithBaseURL(baseURL))
}

// delayRecorder は、待つよう求められた時間を記録し、実時間を待たずに経過を知らせる。
type delayRecorder struct {
	delays []time.Duration
}

func (r *delayRecorder) after(d time.Duration) <-chan time.Time {
	r.delays = append(r.delays, d)
	elapsed := make(chan time.Time, 1)
	elapsed <- time.Time{}
	return elapsed
}

// elapsedAtOnce は、待つ時間を確かめないテストで、実時間を待たずに経過を知らせる。
func elapsedAtOnce(d time.Duration) <-chan time.Time {
	return (&delayRecorder{}).after(d)
}

// relevantTo は、key の設定だけを関係する現在値の設定とし、承認時に予定した値を観測値 observed とする変更前提の基準。
func relevantTo(key setting.Key, observed setting.Value) []changeplan.RelevantSetting {
	return []changeplan.RelevantSetting{{Key: key, AtApproval: github.Observed(observed)}}
}

// singleUpdate は、key を desired にする更新操作一つからなる論理的な設定変更。
func singleUpdate(key setting.Key, desired setting.Value, relevant []changeplan.RelevantSetting) changeplan.LogicalChange {
	return changeplan.LogicalChange{Operations: []changeplan.UpdateOperation{{Key: key, Desired: desired, RelevantSettings: relevant}}}
}

// allowAutoMergeChange は、allow_auto_merge を false から true にする論理的な設定変更。
func allowAutoMergeChange() changeplan.LogicalChange {
	return singleUpdate(setting.AllowAutoMerge, setting.BoolValue(true), relevantTo(setting.AllowAutoMerge, setting.BoolValue(false)))
}

// allowAutoMergeMismatch は、allowAutoMergeChange を導く適用可能な変更予定。
func allowAutoMergeMismatch() changeplan.SettingPlan {
	return changeplan.SettingPlan{
		Key:           setting.AllowAutoMerge,
		Observed:      github.Observed(setting.BoolValue(false)),
		Desired:       setting.BoolValue(true),
		Result:        changeplan.Mismatch,
		Applicability: changeplan.Applicable,
	}
}

// deleteBranchOnMergeChange は、delete_branch_on_merge を false から true にする論理的な設定変更。
func deleteBranchOnMergeChange() changeplan.LogicalChange {
	return singleUpdate(setting.DeleteBranchOnMerge, setting.BoolValue(true), relevantTo(setting.DeleteBranchOnMerge, setting.BoolValue(false)))
}

// deleteBranchOnMergeMismatch は、deleteBranchOnMergeChange を導く適用可能な変更予定。
func deleteBranchOnMergeMismatch() changeplan.SettingPlan {
	return changeplan.SettingPlan{
		Key:           setting.DeleteBranchOnMerge,
		Observed:      github.Observed(setting.BoolValue(false)),
		Desired:       setting.BoolValue(true),
		Result:        changeplan.Mismatch,
		Applicability: changeplan.Applicable,
	}
}

// defaultWorkflowPermissionsChange は、default_workflow_permissions を write から read にする論理的な設定変更。
func defaultWorkflowPermissionsChange() changeplan.LogicalChange {
	return singleUpdate(
		setting.DefaultWorkflowPermissions,
		setting.PermissionLevelValue(setting.PermissionRead),
		relevantTo(setting.DefaultWorkflowPermissions, setting.PermissionLevelValue(setting.PermissionWrite)),
	)
}

// canApprovePullRequestReviewsChange は、can_approve_pull_request_reviews を true から false にする論理的な設定変更。
func canApprovePullRequestReviewsChange() changeplan.LogicalChange {
	return singleUpdate(setting.CanApprovePullRequestReviews, setting.BoolValue(false), relevantTo(setting.CanApprovePullRequestReviews, setting.BoolValue(true)))
}

// allowedActionsChange は、Actions 許可ポリシーを all から local_only にする、副作用のない論理的な設定変更。
func allowedActionsChange() changeplan.LogicalChange {
	return singleUpdate(
		setting.AllowedActions,
		setting.AllowedActionsPolicyValue(setting.AllowedActionsLocalOnly),
		relevantTo(setting.AllowedActions, setting.AllowedActionsPolicyValue(setting.AllowedActionsAll)),
	)
}

// allowedActionsMismatch は、allowedActionsChange を導く適用可能な変更予定。
func allowedActionsMismatch() changeplan.SettingPlan {
	operation := allowedActionsChange().Operations[0]
	return changeplan.SettingPlan{
		Key: operation.Key, Observed: operation.RelevantSettings[0].AtApproval, Desired: operation.Desired, Result: changeplan.Mismatch, Applicability: changeplan.Applicable,
	}
}

// selectedActionsChange は、Actions 許可ポリシーが selected のまま、選択方式の Actions 許可条件の verified_allowed だけを
// false から true にする論理的な設定変更。許可パターンは2つ持ち、順序だけの違いを作れるようにする。
func selectedActionsChange() changeplan.LogicalChange {
	return singleUpdate(
		setting.SelectedActions,
		setting.SelectedActionsCriteriaValue(setting.SelectedActionsCriteria{VerifiedAllowed: true, PatternsAllowed: []string{"a/*", "b/*"}}),
		relevantTo(setting.SelectedActions, setting.SelectedActionsCriteriaValue(setting.SelectedActionsCriteria{PatternsAllowed: []string{"a/*", "b/*"}})),
	)
}

// lostCriteria は、selectedToAllChange が失わせる選択方式の Actions 許可条件。
var lostCriteria = setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, PatternsAllowed: []string{"a/*", "b/*"}}

// selectedToAllChange は、Actions 許可ポリシーを selected から all にし、lostCriteria を失わせる副作用を伴う論理的な設定変更。
// 失われる値も変更前提の基準に並ぶ。
func selectedToAllChange() changeplan.LogicalChange {
	return singleUpdate(setting.AllowedActions, setting.AllowedActionsPolicyValue(setting.AllowedActionsAll), []changeplan.RelevantSetting{
		{Key: setting.AllowedActions, AtApproval: github.Observed(setting.AllowedActionsPolicyValue(setting.AllowedActionsSelected))},
		{Key: setting.SelectedActions, AtApproval: github.Observed(setting.SelectedActionsCriteriaValue(lostCriteria))},
	})
}

// enableActionsNotComparable は、Actions が無効のリポジトリで Actions 許可ポリシーを local_only と宣言した、
// 比較不能で適用可能な変更予定。選択方式の Actions 許可条件が失われる可能性を副作用として伴う。
func enableActionsNotComparable() changeplan.SettingPlan {
	return changeplan.SettingPlan{
		Key: setting.AllowedActions, Observed: github.Unavailable[setting.Value](github.UnavailableActionsDisabled),
		Desired: setting.AllowedActionsPolicyValue(setting.AllowedActionsLocalOnly), Result: changeplan.NotComparable, Applicability: changeplan.Applicable,
		SideEffects: []changeplan.SideEffect{{Key: setting.SelectedActions, Observed: github.Unavailable[setting.Value](github.UnavailableActionsDisabled)}},
	}
}

// enableActionsChange は、enableActionsNotComparable から導く論理的な設定変更。基準はどちらも Actions が無効による観測不能。
func enableActionsChange() changeplan.LogicalChange {
	return singleUpdate(setting.AllowedActions, setting.AllowedActionsPolicyValue(setting.AllowedActionsLocalOnly), []changeplan.RelevantSetting{
		{Key: setting.AllowedActions, AtApproval: github.Unavailable[setting.Value](github.UnavailableActionsDisabled)},
		{Key: setting.SelectedActions, AtApproval: github.Unavailable[setting.Value](github.UnavailableActionsDisabled)},
	})
}

// declaredCriteria は、toSelectedChange と enableSelectedChange が宣言する選択方式の Actions 許可条件。
// github_owned_allowed を false にし、GitHub がポリシーの更新で戻す値 (github_owned_allowed: true、verified_allowed: false、patterns_allowed: []) と
// 3項目とも異ならせる。許可パターンは2つ持ち、順序だけの違いを作れるようにする。
var declaredCriteria = setting.SelectedActionsCriteria{VerifiedAllowed: true, PatternsAllowed: []string{"a/*", "b/*"}}

// toSelectedChange は、Actions 許可ポリシーを local_only から selected にし、選択方式の Actions 許可条件を declaredCriteria にする、
// 二つの更新操作からなる論理的な設定変更。2つ目の基準は、1つ目による予定どおりの変化を反映したポリシーだけである。
func toSelectedChange() changeplan.LogicalChange {
	return changeplan.LogicalChange{Operations: []changeplan.UpdateOperation{
		{
			Key: setting.AllowedActions, Desired: setting.AllowedActionsPolicyValue(setting.AllowedActionsSelected),
			RelevantSettings: relevantTo(setting.AllowedActions, setting.AllowedActionsPolicyValue(setting.AllowedActionsLocalOnly)),
		},
		{
			Key: setting.SelectedActions, Desired: setting.SelectedActionsCriteriaValue(declaredCriteria),
			RelevantSettings: relevantTo(setting.AllowedActions, setting.AllowedActionsPolicyValue(setting.AllowedActionsSelected)),
		},
	}}
}

// enableSelectedChange は、Actions が無効のリポジトリで Actions を有効にして Actions 許可ポリシーを selected にし、
// 選択方式の Actions 許可条件を declaredCriteria にする、二つの更新操作からなる論理的な設定変更。
func enableSelectedChange() changeplan.LogicalChange {
	change := toSelectedChange()
	change.Operations[0].RelevantSettings = []changeplan.RelevantSetting{
		{Key: setting.AllowedActions, AtApproval: github.Unavailable[setting.Value](github.UnavailableActionsDisabled)},
	}
	return change
}

// enabledStateChange は、Actions の有効状態を to にする論理的な設定変更。基準は有効状態の反対の観測値だけである。
func enabledStateChange(to bool) changeplan.LogicalChange {
	return singleUpdate(setting.ActionsEnabled, setting.BoolValue(to), relevantTo(setting.ActionsEnabled, setting.BoolValue(!to)))
}

func newRepositoryPlan(t *testing.T, settings ...changeplan.SettingPlan) changeplan.RepositoryPlan {
	t.Helper()
	return newRepositoryPlanOf(t, "owner/repo", 1, settings...)
}

// newRepositoryPlanOf は、name の対象リポジトリを ID id と観測した、リポジトリの計画。
func newRepositoryPlanOf(t *testing.T, name string, id repository.ID, settings ...changeplan.SettingPlan) changeplan.RepositoryPlan {
	t.Helper()
	return changeplan.RepositoryPlan{Target: mustReference(t, name), RepositoryID: id, Settings: settings}
}

func newPlan(t *testing.T, settings ...changeplan.SettingPlan) changeplan.Plan {
	t.Helper()
	return changeplan.Plan{Repositories: []changeplan.RepositoryPlan{newRepositoryPlan(t, settings...)}}
}

// unknownKey は、更新要求を持たない設定。
const unknownKey setting.Key = "general.unknown"

// unknownMismatch は、更新要求を持たない設定の適用可能な変更予定。
func unknownMismatch() changeplan.SettingPlan {
	return changeplan.SettingPlan{Key: unknownKey, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: changeplan.Mismatch, Applicability: changeplan.Applicable}
}

// TestExecutableChanges は、承認を求める前に、論理的な設定変更とその更新操作を確定する規則を確かめる。
//
//trace:checks INV-002
//trace:checks FR-009
func TestExecutableChanges(t *testing.T) {
	match := changeplan.SettingPlan{Key: setting.DeleteBranchOnMerge, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(true), Result: changeplan.Match}
	notApplicable := changeplan.SettingPlan{
		Key: setting.SHAPinningRequired, Observed: github.Unavailable[setting.Value](github.UnavailableActionsDisabled), Desired: setting.BoolValue(true),
		Result: changeplan.NotComparable, Applicability: changeplan.NotApplicable, NotApplicableReason: changeplan.NotApplicableActionsRemainDisabled,
	}
	// 宣言の並びを名前順と逆にし、名前順に並べ替える実装を失敗させる
	plans := func(t *testing.T, settings ...[]changeplan.SettingPlan) changeplan.Plan {
		t.Helper()
		names := []string{"owner/zeta", "owner/beta", "owner/alpha"}
		var p changeplan.Plan
		for i, s := range settings {
			p.Repositories = append(p.Repositories, newRepositoryPlanOf(t, names[i], repository.ID(i+1), s...))
		}
		return p
	}

	t.Run("repositories are returned in the declared order with their own plans and logical changes", func(t *testing.T) {
		p := plans(t, []changeplan.SettingPlan{allowedActionsMismatch()}, []changeplan.SettingPlan{allowAutoMergeMismatch()})
		got, err := executableChanges(p)
		if err != nil {
			t.Fatalf("executableChanges() error = %v", err)
		}
		want := []repositoryChanges{
			{plan: p.Repositories[0], changes: []changeplan.LogicalChange{allowedActionsChange()}},
			{plan: p.Repositories[1], changes: []changeplan.LogicalChange{allowAutoMergeChange()}},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("executableChanges() = %+v, want %+v", got, want)
		}
	})

	t.Run("repository without changes between two keeps its position", func(t *testing.T) {
		p := plans(t, []changeplan.SettingPlan{allowAutoMergeMismatch()}, []changeplan.SettingPlan{match}, []changeplan.SettingPlan{allowAutoMergeMismatch()})
		got, err := executableChanges(p)
		if err != nil {
			t.Fatalf("executableChanges() error = %v", err)
		}
		want := []repositoryChanges{
			{plan: p.Repositories[0], changes: []changeplan.LogicalChange{allowAutoMergeChange()}},
			{plan: p.Repositories[1]},
			{plan: p.Repositories[2], changes: []changeplan.LogicalChange{allowAutoMergeChange()}},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("executableChanges() = %+v, want %+v", got, want)
		}
	})

	t.Run("not applicable planned change in the second repository returns an error", func(t *testing.T) {
		got, err := executableChanges(plans(t, []changeplan.SettingPlan{allowAutoMergeMismatch()}, []changeplan.SettingPlan{notApplicable}))
		if err == nil {
			t.Errorf("executableChanges() = %+v, want an error", got)
		}
	})

	t.Run("update operation for a setting without an update request after an executable one panics before approval", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("executableChanges() did not panic")
			}
		}()
		_, _ = executableChanges(plans(t, []changeplan.SettingPlan{allowAutoMergeMismatch(), unknownMismatch()}))
	})

	t.Run("update operation for a setting without an update request in the second repository panics before approval", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("executableChanges() did not panic")
			}
		}()
		_, _ = executableChanges(plans(t, []changeplan.SettingPlan{allowAutoMergeMismatch()}, []changeplan.SettingPlan{unknownMismatch()}))
	})
}

//trace:checks INV-002
//trace:checks INV-003
//trace:checks INV-005
//trace:checks FR-008
//trace:checks FR-009
//trace:checks FR-013
func TestApplyPlan(t *testing.T) {
	approve := func(calls *int) func(context.Context) error {
		return func(context.Context) error {
			*calls++
			return nil
		}
	}

	match := changeplan.SettingPlan{Key: setting.DeleteBranchOnMerge, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(true), Result: changeplan.Match}

	t.Run("no change in any repository does not ask for approval or send a request", func(t *testing.T) {
		baseURL, server := newSequenceServer(t)
		var out bytes.Buffer
		calls := 0
		p := changeplan.Plan{Repositories: []changeplan.RepositoryPlan{
			newRepositoryPlanOf(t, "owner/first", 1, match), newRepositoryPlanOf(t, "owner/second", 2, match),
		}}

		err := applyPlan(context.Background(), Config{Out: &out, Approve: approve(&calls), After: elapsedAtOnce}, newClient(baseURL), p)

		if err != nil {
			t.Fatalf("applyPlan() error = %v", err)
		}
		if want := "No changes.\nRepositories checked: 2\n"; out.String() != want {
			t.Errorf("got Out %q, want %q", out.String(), want)
		}
		if calls != 0 || len(server.Methods()) != 0 {
			t.Errorf("approve calls = %d, requests = %v, want none", calls, server.Methods())
		}
	})

	// 先頭を変更不要にし、先頭の対象リポジトリだけで変更の有無を判定する実装を失敗させる
	t.Run("change plan of three repositories is approved once and applied in the declared order", func(t *testing.T) {
		baseURL, server := newSequenceServer(t,
			okRepository(2, false, false), okRepository(2, true, false), okRepository(2, true, false),
			okRepository(3, false, false), okRepository(3, true, false), okRepository(3, true, false))
		var out bytes.Buffer
		var shown string
		calls := 0
		cfg := Config{Configuration: configuration.File{Path: "./reco.yaml"}, Out: &out, Approve: func(context.Context) error {
			calls++
			shown = out.String()
			return nil
		}, After: elapsedAtOnce}
		p := changeplan.Plan{Repositories: []changeplan.RepositoryPlan{
			newRepositoryPlanOf(t, "owner/first", 1, match),
			newRepositoryPlanOf(t, "owner/second", 2, allowAutoMergeMismatch()),
			newRepositoryPlanOf(t, "owner/third", 3, allowAutoMergeMismatch()),
		}}

		err := applyPlan(context.Background(), cfg, newClient(baseURL), p)

		if err != nil {
			t.Fatalf("applyPlan() error = %v", err)
		}
		if calls != 1 {
			t.Errorf("approve calls = %d, want 1", calls)
		}
		wantShown := "Configuration: ./reco.yaml\n\n" +
			"Repository: owner/second\n  general.allow_auto_merge: false -> true\n\n" +
			"Repository: owner/third\n  general.allow_auto_merge: false -> true\n\n" +
			"Plan: 2 repositories to update, 2 changes\nRepositories checked: 3\n\n"
		if shown != wantShown {
			t.Errorf("Out at approval = %q, want %q", shown, wantShown)
		}
		wantRequests := []string{
			"GET /repos/owner/second", "PATCH /repos/owner/second", "GET /repos/owner/second",
			"GET /repos/owner/third", "PATCH /repos/owner/third", "GET /repos/owner/third",
		}
		if !slices.Equal(server.RequestLines(), wantRequests) {
			t.Errorf("requests = %v, want %v", server.RequestLines(), wantRequests)
		}
		wantReport := "Applying changes...\n  owner/first: no change\n  owner/second: updated\n  owner/third: updated\n\nApply complete.\nRepositories updated: 2\n"
		if got := strings.TrimPrefix(out.String(), shown); got != wantReport {
			t.Errorf("Out after approval = %q, want %q", got, wantReport)
		}
	})

	t.Run("failure in the first repository returns its cause and reports the second not attempted", func(t *testing.T) {
		baseURL, server := newSequenceServer(t, okRepository(1, false, false), response{status: http.StatusForbidden, body: `{"message": "Forbidden"}`})
		var out bytes.Buffer
		calls := 0
		p := changeplan.Plan{Repositories: []changeplan.RepositoryPlan{
			newRepositoryPlanOf(t, "owner/first", 1, allowAutoMergeMismatch()),
			newRepositoryPlanOf(t, "owner/second", 2, allowAutoMergeMismatch()),
		}}

		err := applyPlan(context.Background(), Config{Out: &out, Approve: approve(&calls), After: elapsedAtOnce}, newClient(baseURL), p)

		if !errors.Is(err, github.ErrForbidden) {
			t.Errorf("applyPlan() error = %v, want %v", err, github.ErrForbidden)
		}
		if want := []string{"GET /repos/owner/first", "PATCH /repos/owner/first"}; !slices.Equal(server.RequestLines(), want) {
			t.Errorf("requests = %v, want %v", server.RequestLines(), want)
		}
		if !strings.Contains(out.String(), "owner/second: not attempted") || strings.Contains(out.String(), "Apply complete.") {
			t.Errorf("got Out %q, want the second not attempted without Apply complete.", out.String())
		}
	})

	t.Run("change plan without a repository plan panics before asking for approval", func(t *testing.T) {
		calls := 0
		defer func() {
			if recover() == nil {
				t.Error("applyPlan() did not panic")
			}
			if calls != 0 {
				t.Errorf("approve calls = %d, want none", calls)
			}
		}()
		_ = applyPlan(context.Background(), Config{Out: &bytes.Buffer{}, Approve: approve(&calls)}, nil, changeplan.Plan{})
	})

	t.Run("unappliable plan returns an error without approval or output", func(t *testing.T) {
		baseURL, server := newSequenceServer(t)
		var out bytes.Buffer
		calls := 0
		notApplicable := changeplan.SettingPlan{
			Key: setting.SHAPinningRequired, Observed: github.Unavailable[setting.Value](github.UnavailableActionsDisabled), Desired: setting.BoolValue(true),
			Result: changeplan.NotComparable, Applicability: changeplan.NotApplicable, NotApplicableReason: changeplan.NotApplicableActionsRemainDisabled,
		}

		err := applyPlan(context.Background(), Config{Out: &out, Approve: approve(&calls), After: elapsedAtOnce}, newClient(baseURL), newPlan(t, notApplicable))

		if err == nil {
			t.Error("applyPlan() error = nil, want an error")
		}
		if calls != 0 || out.Len() != 0 || len(server.Methods()) != 0 {
			t.Errorf("approve calls = %d, Out = %q, requests = %v, want none", calls, out.String(), server.Methods())
		}
	})

	t.Run("not-comparable policy and its side effect are shown before approval and declining sends no request", func(t *testing.T) {
		baseURL, server := newSequenceServer(t)
		var out bytes.Buffer
		var shown string
		cfg := Config{Out: &out, Approve: func(context.Context) error {
			shown = out.String()
			return errors.New("declined")
		}}

		if err := applyPlan(context.Background(), cfg, newClient(baseURL), newPlan(t, enableActionsNotComparable())); err == nil {
			t.Error("applyPlan() error = nil, want an error")
		}
		for _, want := range []string{"(GitHub Actions disabled) -> ", "Side effect:"} {
			if !strings.Contains(shown, want) {
				t.Errorf("Out at approval = %q, want it to contain %q", shown, want)
			}
		}
		if len(server.Methods()) != 0 {
			t.Errorf("requests = %v, want none", server.Methods())
		}
	})

	t.Run("two changes are shown before approval and both are applied with the interval between update requests", func(t *testing.T) {
		baseURL, server := newSequenceServer(t,
			okRepository(1, false, false), okRepository(1, false, true), okRepository(1, false, true),
			okRepository(1, false, true), okRepository(1, true, true), okRepository(1, true, true))
		var out bytes.Buffer
		var shown string
		delays := &delayRecorder{}
		cfg := Config{Out: &out, Approve: func(context.Context) error {
			shown = out.String()
			return nil
		}, After: delays.after}

		err := applyPlan(context.Background(), cfg, newClient(baseURL), newPlan(t, deleteBranchOnMergeMismatch(), allowAutoMergeMismatch()))

		if err != nil {
			t.Fatalf("applyPlan() error = %v", err)
		}
		for _, want := range []string{"general.delete_branch_on_merge: false -> true", "general.allow_auto_merge: false -> true"} {
			if !strings.Contains(shown, want) {
				t.Errorf("Out at approval = %q, want it to contain %q", shown, want)
			}
		}
		if want := []string{"GET", "PATCH", "GET", "GET", "PATCH", "GET"}; !slices.Equal(server.Methods(), want) {
			t.Errorf("requests = %v, want %v", server.Methods(), want)
		}
		if want := []time.Duration{time.Second}; !slices.Equal(delays.delays, want) {
			t.Errorf("delays = %v, want %v", delays.delays, want)
		}
		if !strings.Contains(out.String(), "owner/repo: updated") {
			t.Errorf("got Out %q, want it to contain %q", out.String(), "owner/repo: updated")
		}
	})

	t.Run("failed second change returns an error without reporting the repository updated", func(t *testing.T) {
		baseURL, _ := newSequenceServer(t,
			okRepository(1, false, false), okRepository(1, false, true), okRepository(1, false, true),
			okRepository(1, false, true), okRepository(1, true, true), response{status: http.StatusInternalServerError})
		var out bytes.Buffer
		calls := 0

		err := applyPlan(context.Background(), Config{Out: &out, Approve: approve(&calls), After: elapsedAtOnce}, newClient(baseURL), newPlan(t, deleteBranchOnMergeMismatch(), allowAutoMergeMismatch()))

		if err == nil {
			t.Error("applyPlan() error = nil, want an error")
		}
		if strings.Contains(out.String(), "owner/repo: updated") {
			t.Errorf("got Out %q, want no repository updated line", out.String())
		}
	})

	t.Run("approval error cancels without a request", func(t *testing.T) {
		baseURL, server := newSequenceServer(t)
		var out bytes.Buffer
		errDeclined := errors.New("declined")
		cfg := Config{Out: &out, Approve: func(context.Context) error { return errDeclined }}

		err := applyPlan(context.Background(), cfg, newClient(baseURL), newPlan(t, allowAutoMergeMismatch()))

		if !errors.Is(err, errDeclined) {
			t.Fatalf("applyPlan() error = %v, want %v", err, errDeclined)
		}
		for _, want := range []string{"apply cancelled", "no changes made"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("applyPlan() error = %q, want it to contain %q", err, want)
			}
		}
		if len(server.Methods()) != 0 {
			t.Errorf("requests = %v, want none", server.Methods())
		}
		if strings.Contains(out.String(), "Applying") {
			t.Errorf("got Out %q, want no Applying line", out.String())
		}
	})

	t.Run("context canceled during approval does not start the change", func(t *testing.T) {
		baseURL, server := newSequenceServer(t)
		var out bytes.Buffer
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		cfg := Config{Out: &out, Approve: func(context.Context) error {
			cancel()
			return nil
		}}

		err := applyPlan(ctx, cfg, newClient(baseURL), newPlan(t, allowAutoMergeMismatch()))

		if !errors.Is(err, context.Canceled) {
			t.Errorf("applyPlan() error = %v, want %v", err, context.Canceled)
		}
		if len(server.Methods()) != 0 {
			t.Errorf("requests = %v, want none", server.Methods())
		}
		if strings.Contains(out.String(), "Applying") {
			t.Errorf("got Out %q, want no Applying line", out.String())
		}
	})

	t.Run("approved change is applied and reported as complete", func(t *testing.T) {
		baseURL, server := newSequenceServer(t, okRepository(1, false, false), okRepository(1, true, false), okRepository(1, true, false))
		var out bytes.Buffer
		calls := 0

		err := applyPlan(context.Background(), Config{Out: &out, Approve: approve(&calls), After: elapsedAtOnce}, newClient(baseURL), newPlan(t, allowAutoMergeMismatch()))

		if err != nil {
			t.Fatalf("applyPlan() error = %v", err)
		}
		if want := []string{"GET", "PATCH", "GET"}; !slices.Equal(server.Methods(), want) {
			t.Errorf("requests = %v, want %v", server.Methods(), want)
		}
		for _, want := range []string{"Applying changes...\n", "owner/repo: updated", "Apply complete."} {
			if !strings.Contains(out.String(), want) {
				t.Errorf("got Out %q, want it to contain %q", out.String(), want)
			}
		}
	})

	t.Run("unknown update outcome fails after a single update request", func(t *testing.T) {
		baseURL, server := newSequenceServer(t,
			okRepository(1, false, false),
			response{status: http.StatusTemporaryRedirect, location: "https://example.com/"},
			okRepository(1, true, false))
		var out bytes.Buffer
		calls := 0

		err := applyPlan(context.Background(), Config{Out: &out, Approve: approve(&calls), After: elapsedAtOnce}, newClient(baseURL), newPlan(t, allowAutoMergeMismatch()))

		if !errors.Is(err, github.ErrRedirect) {
			t.Errorf("applyPlan() error = %v, want %v", err, github.ErrRedirect)
		}
		if want := []string{"GET", "PATCH", "GET"}; !slices.Equal(server.Methods(), want) {
			t.Errorf("requests = %v, want %v", server.Methods(), want)
		}
		if strings.Contains(out.String(), ": updated") {
			t.Errorf("got Out %q, want no updated line", out.String())
		}
	})

	t.Run("changed repository ID is reported as not attempted without an update request", func(t *testing.T) {
		baseURL, server := newSequenceServer(t, okRepository(2, false, false))
		var out bytes.Buffer
		calls := 0

		err := applyPlan(context.Background(), Config{Out: &out, Approve: approve(&calls), After: elapsedAtOnce}, newClient(baseURL), newPlan(t, allowAutoMergeMismatch()))

		if err == nil {
			t.Error("applyPlan() error = nil, want an error")
		}
		if want := []string{"GET"}; !slices.Equal(server.Methods(), want) {
			t.Errorf("requests = %v, want %v", server.Methods(), want)
		}
		if !strings.Contains(out.String(), "not attempted") {
			t.Errorf("got Out %q, want it to contain %q", out.String(), "not attempted")
		}
	})

	t.Run("change plan write failure skips approval", func(t *testing.T) {
		baseURL, server := newSequenceServer(t)
		calls := 0

		err := applyPlan(context.Background(), Config{Out: failingWriter{}, Approve: approve(&calls)}, newClient(baseURL), newPlan(t, allowAutoMergeMismatch()))

		if !errors.Is(err, errWrite) {
			t.Errorf("applyPlan() error = %v, want %v", err, errWrite)
		}
		if calls != 0 || len(server.Methods()) != 0 {
			t.Errorf("approve calls = %d, requests = %v, want none", calls, server.Methods())
		}
	})

	t.Run("applying line write failure skips the update", func(t *testing.T) {
		baseURL, server := newSequenceServer(t)
		calls := 0
		out := &failAfterWriter{n: 1}

		err := applyPlan(context.Background(), Config{Out: out, Approve: approve(&calls), After: elapsedAtOnce}, newClient(baseURL), newPlan(t, allowAutoMergeMismatch()))

		if !errors.Is(err, errWrite) {
			t.Errorf("applyPlan() error = %v, want %v", err, errWrite)
		}
		if len(server.Methods()) != 0 {
			t.Errorf("requests = %v, want none", server.Methods())
		}
	})

	t.Run("change report write failure is returned after the update", func(t *testing.T) {
		baseURL, server := newSequenceServer(t, okRepository(1, false, false), okRepository(1, true, false), okRepository(1, true, false))
		calls := 0
		out := &failAfterWriter{n: 2}

		err := applyPlan(context.Background(), Config{Out: out, Approve: approve(&calls), After: elapsedAtOnce}, newClient(baseURL), newPlan(t, allowAutoMergeMismatch()))

		if !errors.Is(err, errWrite) {
			t.Errorf("applyPlan() error = %v, want %v", err, errWrite)
		}
		if want := []string{"GET", "PATCH", "GET"}; !slices.Equal(server.Methods(), want) {
			t.Errorf("requests = %v, want %v", server.Methods(), want)
		}
	})
}

const configSource = `repositories:
  - name: owner/repo
    general:
      allow_auto_merge: true
`

// workflowConfigSource は、allow_auto_merge と default_workflow_permissions を宣言する。
const workflowConfigSource = `repositories:
  - name: owner/repo
    general:
      allow_auto_merge: true
    actions:
      workflow:
        default_workflow_permissions: read
`

//trace:checks INV-002
//trace:checks FR-008
//trace:checks FR-011
func TestRun(t *testing.T) {
	newConfig := func(files fstest.MapFS, out *bytes.Buffer, baseURL *url.URL, calls *int) Config {
		return Config{
			Token:         "test-token",
			Configuration: configuration.File{Dir: files, Name: "reco.yaml", Path: "./reco.yaml"},
			Out:           out,
			Approve: func(context.Context) error {
				*calls++
				return nil
			},
			After:         elapsedAtOnce,
			GitHubOptions: []github.Option{github.WithBaseURL(baseURL)},
		}
	}

	t.Run("general and workflow permissions mismatches are applied after a single approval", func(t *testing.T) {
		baseURL, server := newSequenceServer(t,
			okRepository(1, false, false), okWorkflowPermissions("write", false),
			okRepository(1, false, false), okRepository(1, true, false), okRepository(1, true, false),
			okRepository(1, true, false), okWorkflowPermissions("write", false), response{status: http.StatusNoContent},
			okRepository(1, true, false), okWorkflowPermissions("read", false))
		var out bytes.Buffer
		calls := 0
		files := fstest.MapFS{"reco.yaml": {Data: []byte(workflowConfigSource)}}

		if err := Run(context.Background(), newConfig(files, &out, baseURL, &calls)); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if calls != 1 {
			t.Errorf("approve calls = %d, want 1", calls)
		}
		// actions.permissions を宣言しないため、Actions permissions には観測の要求も送らない
		want := []string{
			"GET /repos/owner/repo", "GET /repos/owner/repo/actions/permissions/workflow",
			"GET /repos/owner/repo", "PATCH /repos/owner/repo", "GET /repos/owner/repo",
			"GET /repos/owner/repo", "GET /repos/owner/repo/actions/permissions/workflow", "PUT /repos/owner/repo/actions/permissions/workflow",
			"GET /repos/owner/repo", "GET /repos/owner/repo/actions/permissions/workflow",
		}
		if !slices.Equal(server.RequestLines(), want) {
			t.Errorf("requests = %v, want %v", server.RequestLines(), want)
		}
		if !strings.Contains(out.String(), "actions.workflow.default_workflow_permissions: write -> read") ||
			!strings.Contains(out.String(), "owner/repo: updated") {
			t.Errorf("got Out %q, want the workflow permissions change and the repository updated", out.String())
		}
	})

	t.Run("matching workflow permissions leave only the general change to apply", func(t *testing.T) {
		baseURL, server := newSequenceServer(t,
			okRepository(1, false, false), okWorkflowPermissions("read", false),
			okRepository(1, false, false), okRepository(1, true, false), okRepository(1, true, false))
		var out bytes.Buffer
		calls := 0
		files := fstest.MapFS{"reco.yaml": {Data: []byte(workflowConfigSource)}}

		if err := Run(context.Background(), newConfig(files, &out, baseURL, &calls)); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if want := []string{"GET", "GET", "GET", "PATCH", "GET"}; !slices.Equal(server.Methods(), want) {
			t.Errorf("requests = %v, want %v", server.Methods(), want)
		}
		if !strings.Contains(out.String(), "owner/repo: updated") {
			t.Errorf("got Out %q, want the repository to be updated", out.String())
		}
	})

	const (
		getRepository         = "GET /repos/owner/repo"
		getActionsPermissions = "GET /repos/owner/repo/actions/permissions"
		getSelectedActions    = "GET /repos/owner/repo/actions/permissions/selected-actions"
		putActionsPermissions = "PUT /repos/owner/repo/actions/permissions"
	)

	// 計画時の観測から導いた変更前提の基準が、更新直前の同じ観測で成立することを確かめる
	t.Run("selected to all is applied after showing the side effect and a single approval", func(t *testing.T) {
		baseURL, server := newSequenceServer(t,
			okRepository(1, false, false), okAllowedActions("selected", false), okSelectedActions(true, false, "a/*"),
			okRepository(1, false, false), okAllowedActions("selected", false), okSelectedActions(true, false, "a/*"),
			response{status: http.StatusNoContent},
			okRepository(1, false, false), okAllowedActions("all", false))
		var out bytes.Buffer
		calls := 0
		files := fstest.MapFS{"reco.yaml": {Data: []byte(actionsPermissionsConfigSource("all"))}}

		if err := Run(context.Background(), newConfig(files, &out, baseURL, &calls)); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		reads := []string{getRepository, getActionsPermissions, getSelectedActions}
		want := slices.Concat(reads, reads, []string{putActionsPermissions, getRepository, getActionsPermissions})
		if !slices.Equal(server.RequestLines(), want) {
			t.Errorf("requests = %v, want %v", server.RequestLines(), want)
		}
		if calls != 1 || !strings.Contains(out.String(), "Side effect:") || !strings.Contains(out.String(), "owner/repo: updated") {
			t.Errorf("approve calls = %d, Out = %q, want one approval, the side effect, and the repository updated", calls, out.String())
		}
	})

	t.Run("policy of disabled actions is not reported as no changes and is applied after a single approval", func(t *testing.T) {
		baseURL, server := newSequenceServer(t,
			okRepository(1, false, false), okActionsDisabled(),
			okRepository(1, false, false), okActionsDisabled(),
			response{status: http.StatusNoContent},
			okRepository(1, false, false), okAllowedActions("local_only", false))
		var out bytes.Buffer
		calls := 0
		files := fstest.MapFS{"reco.yaml": {Data: []byte(actionsPermissionsConfigSource("local_only"))}}

		if err := Run(context.Background(), newConfig(files, &out, baseURL, &calls)); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		want := []string{getRepository, getActionsPermissions, getRepository, getActionsPermissions, putActionsPermissions, getRepository, getActionsPermissions}
		if !slices.Equal(server.RequestLines(), want) {
			t.Errorf("requests = %v, want %v", server.RequestLines(), want)
		}
		if calls != 1 || !strings.Contains(out.String(), "owner/repo: updated") {
			t.Errorf("approve calls = %d, Out = %q, want one approval and the repository updated", calls, out.String())
		}
	})

	t.Run("disabling actions is applied in a single update after a single approval", func(t *testing.T) {
		baseURL, server := newSequenceServer(t,
			okRepository(1, false, false), okAllowedActions("all", true),
			okRepository(1, false, false), okAllowedActions("all", true),
			response{status: http.StatusNoContent},
			okRepository(1, false, false), okActionsDisabled())
		var out bytes.Buffer
		calls := 0
		source := "repositories:\n  - name: owner/repo\n    actions:\n      permissions:\n        enabled: false\n"
		files := fstest.MapFS{"reco.yaml": {Data: []byte(source)}}

		if err := Run(context.Background(), newConfig(files, &out, baseURL, &calls)); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		want := []string{getRepository, getActionsPermissions, getRepository, getActionsPermissions, putActionsPermissions, getRepository, getActionsPermissions}
		if !slices.Equal(server.RequestLines(), want) {
			t.Errorf("requests = %v, want %v", server.RequestLines(), want)
		}
		if calls != 1 || !strings.Contains(out.String(), "owner/repo: updated") {
			t.Errorf("approve calls = %d, Out = %q, want one approval and the repository updated", calls, out.String())
		}
	})

	// 計画時の観測から導いた2つの更新操作の基準が、それぞれの直前の観測で成立することを確かめる
	selectedSource := "repositories:\n  - name: owner/repo\n    actions:\n      permissions:\n        allowed_actions: selected\n" +
		"        selected_actions:\n          github_owned_allowed: false\n          verified_allowed: true\n          patterns_allowed: [\"a/*\"]\n"
	putSelectedActions := "PUT /repos/owner/repo/actions/permissions/selected-actions"
	readSelected := []string{getRepository, getActionsPermissions, getSelectedActions}
	readNotSelected := []string{getRepository, getActionsPermissions}
	// 1つ目のあとに GitHub が戻した3項目と、宣言した3項目
	afterFirst := []response{okRepository(1, false, false), okAllowedActions("selected", false), okSelectedActions(true, false)}
	afterSecond := []response{okRepository(1, false, false), okAllowedActions("selected", false), okSelectedActions(false, true, "a/*")}

	t.Run("policy change to selected is applied in two updates after showing both settings and a single approval", func(t *testing.T) {
		baseURL, server := newSequenceServer(t, slices.Concat(
			[]response{okRepository(1, false, false), okAllowedActions("all", false)},
			[]response{okRepository(1, false, false), okAllowedActions("all", false), {status: http.StatusNoContent}},
			afterFirst, []response{{status: http.StatusNoContent}}, afterSecond)...)
		var out bytes.Buffer
		calls := 0
		files := fstest.MapFS{"reco.yaml": {Data: []byte(selectedSource)}}

		if err := Run(context.Background(), newConfig(files, &out, baseURL, &calls)); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		want := slices.Concat(readNotSelected, readNotSelected, []string{putActionsPermissions}, readSelected, []string{putSelectedActions}, readSelected)
		if !slices.Equal(server.RequestLines(), want) {
			t.Errorf("requests = %v, want %v", server.RequestLines(), want)
		}
		if calls != 1 || !strings.Contains(out.String(), "actions.permissions.allowed_actions: all -> selected\n  actions.permissions.selected_actions:\n") ||
			!strings.Contains(out.String(), "owner/repo: updated") {
			t.Errorf("approve calls = %d, Out = %q, want one approval, both planned settings, and the repository updated", calls, out.String())
		}
	})

	t.Run("policy of disabled actions to selected is applied in two updates after a single approval", func(t *testing.T) {
		baseURL, server := newSequenceServer(t, slices.Concat(
			[]response{okRepository(1, false, false), okActionsDisabled()},
			[]response{okRepository(1, false, false), okActionsDisabled(), {status: http.StatusNoContent}},
			afterFirst, []response{{status: http.StatusNoContent}}, afterSecond)...)
		var out bytes.Buffer
		calls := 0
		files := fstest.MapFS{"reco.yaml": {Data: []byte(selectedSource)}}

		if err := Run(context.Background(), newConfig(files, &out, baseURL, &calls)); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		want := slices.Concat(readNotSelected, readNotSelected, []string{putActionsPermissions}, readSelected, []string{putSelectedActions}, readSelected)
		if !slices.Equal(server.RequestLines(), want) {
			t.Errorf("requests = %v, want %v", server.RequestLines(), want)
		}
		if calls != 1 || strings.Count(out.String(), "(GitHub Actions disabled)") != 2 ||
			!strings.Contains(out.String(), "(enables GitHub Actions)") || !strings.Contains(out.String(), "owner/repo: updated") {
			t.Errorf("approve calls = %d, Out = %q, want one approval, both not-comparable settings, and the repository updated", calls, out.String())
		}
	})

	t.Run("selected criteria change is applied without an actions permissions update", func(t *testing.T) {
		baseURL, server := newSequenceServer(t,
			okRepository(1, false, false), okAllowedActions("selected", false), okSelectedActions(false, false, "a/*"),
			okRepository(1, false, false), okAllowedActions("selected", false), okSelectedActions(false, false, "a/*"),
			response{status: http.StatusNoContent},
			okRepository(1, false, false), okAllowedActions("selected", false), okSelectedActions(false, true, "a/*"))
		var out bytes.Buffer
		calls := 0
		source := "repositories:\n  - name: owner/repo\n    actions:\n      permissions:\n        allowed_actions: selected\n" +
			"        selected_actions:\n          github_owned_allowed: false\n          verified_allowed: true\n          patterns_allowed: [\"a/*\"]\n"
		files := fstest.MapFS{"reco.yaml": {Data: []byte(source)}}

		if err := Run(context.Background(), newConfig(files, &out, baseURL, &calls)); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		reads := []string{"GET /repos/owner/repo", "GET /repos/owner/repo/actions/permissions", "GET /repos/owner/repo/actions/permissions/selected-actions"}
		want := slices.Concat(reads, reads, []string{"PUT /repos/owner/repo/actions/permissions/selected-actions"}, reads)
		if !slices.Equal(server.RequestLines(), want) {
			t.Errorf("requests = %v, want %v", server.RequestLines(), want)
		}
		if calls != 1 || !strings.Contains(out.String(), "owner/repo: updated") {
			t.Errorf("approve calls = %d, Out = %q, want one approval and the repository updated", calls, out.String())
		}
	})

	t.Run("general, actions permissions, and workflow permissions changes are applied in the configuration order", func(t *testing.T) {
		const (
			getRepository         = "GET /repos/owner/repo"
			getActionsPermissions = "GET /repos/owner/repo/actions/permissions"
			getWorkflow           = "GET /repos/owner/repo/actions/permissions/workflow"
		)
		baseURL, server := newSequenceServer(t,
			okRepository(1, false, false), okAllowedActions("all", false), okWorkflowPermissions("write", false),
			okRepository(1, false, false), okRepository(1, true, false), okRepository(1, true, false),
			okRepository(1, true, false), okAllowedActions("all", false), response{status: http.StatusNoContent},
			okRepository(1, true, false), okAllowedActions("local_only", false),
			okRepository(1, true, false), okWorkflowPermissions("write", false), response{status: http.StatusNoContent},
			okRepository(1, true, false), okWorkflowPermissions("read", false))
		var out bytes.Buffer
		calls := 0
		source := configSource + "    actions:\n      permissions:\n        allowed_actions: local_only\n      workflow:\n        default_workflow_permissions: read\n"
		files := fstest.MapFS{"reco.yaml": {Data: []byte(source)}}

		if err := Run(context.Background(), newConfig(files, &out, baseURL, &calls)); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		want := []string{
			getRepository, getActionsPermissions, getWorkflow,
			getRepository, "PATCH /repos/owner/repo", getRepository,
			getRepository, getActionsPermissions, "PUT /repos/owner/repo/actions/permissions", getRepository, getActionsPermissions,
			getRepository, getWorkflow, "PUT /repos/owner/repo/actions/permissions/workflow", getRepository, getWorkflow,
		}
		if !slices.Equal(server.RequestLines(), want) {
			t.Errorf("requests = %v, want %v", server.RequestLines(), want)
		}
		if !strings.Contains(out.String(), "owner/repo: updated") {
			t.Errorf("got Out %q, want the repository updated", out.String())
		}
	})

	t.Run("matching actions permissions leave only the general change to apply", func(t *testing.T) {
		baseURL, server := newSequenceServer(t,
			okRepository(1, false, false), okAllowedActions("all", false),
			okRepository(1, false, false), okRepository(1, true, false), okRepository(1, true, false))
		var out bytes.Buffer
		calls := 0
		source := configSource + "    actions:\n      permissions:\n        allowed_actions: all\n"
		files := fstest.MapFS{"reco.yaml": {Data: []byte(source)}}

		if err := Run(context.Background(), newConfig(files, &out, baseURL, &calls)); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if want := []string{"GET", "GET", "GET", "PATCH", "GET"}; !slices.Equal(server.Methods(), want) {
			t.Errorf("requests = %v, want %v", server.Methods(), want)
		}
		if calls != 1 || !strings.Contains(out.String(), "owner/repo: updated") {
			t.Errorf("approve calls = %d, Out = %q, want one approval and the repository updated", calls, out.String())
		}
	})

	t.Run("missing configuration file stops before approval and requests", func(t *testing.T) {
		baseURL, server := newSequenceServer(t)
		var out bytes.Buffer
		calls := 0

		err := Run(context.Background(), newConfig(fstest.MapFS{}, &out, baseURL, &calls))

		if err == nil {
			t.Error("Run() error = nil, want an error")
		}
		if calls != 0 || len(server.Methods()) != 0 {
			t.Errorf("approve calls = %d, requests = %v, want none", calls, server.Methods())
		}
	})

	t.Run("matching observation reports no changes with the token", func(t *testing.T) {
		baseURL, server := newSequenceServer(t, okRepository(1, true, false))
		var out bytes.Buffer
		calls := 0
		files := fstest.MapFS{"reco.yaml": {Data: []byte(configSource)}}

		err := Run(context.Background(), newConfig(files, &out, baseURL, &calls))

		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if !strings.HasPrefix(out.String(), "No changes.") {
			t.Errorf("got Out %q, want it to start with %q", out.String(), "No changes.")
		}
		authorization := ""
		if requests := server.Requests(); len(requests) > 0 {
			authorization = requests[len(requests)-1].Authorization
		}
		if !strings.Contains(authorization, "test-token") {
			t.Errorf("got Authorization %q, want it to contain the token", authorization)
		}
		if calls != 0 {
			t.Errorf("approve calls = %d, want none", calls)
		}
	})

	t.Run("selection is passed to change planning", func(t *testing.T) {
		baseURL, server := newSequenceServer(t)
		var out bytes.Buffer
		calls := 0
		files := fstest.MapFS{"reco.yaml": {Data: []byte(configSource)}}
		cfg := newConfig(files, &out, baseURL, &calls)
		selection := mustReference(t, "owner/other")
		cfg.Selection = &selection

		if err := Run(context.Background(), cfg); err == nil {
			t.Fatal("Run() error = nil, want an error")
		}
		if len(server.Methods()) != 0 {
			t.Errorf("requests = %v, want none", server.Methods())
		}
	})
}
