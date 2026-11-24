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
	"testing"
	"testing/fstest"
	"time"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/changeplandisplay"
	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/githubtest"
	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

var errWrite = errors.New("write failed")

type failingWriter struct{}

// lostCriteria は、selectedToAllChange が失わせる選択方式の Actions 許可条件。
var lostCriteria = setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, PatternsAllowed: []string{"a/*", "b/*"}}

// declaredCriteria は、toSelectedChange と enableSelectedChange が宣言する選択方式の Actions 許可条件。
// github_owned_allowed を false にし、GitHub がポリシーの更新で戻す値 (github_owned_allowed: true、verified_allowed: false、patterns_allowed: []) と
// 3項目とも異ならせる。許可パターンは2つ持ち、順序だけの違いを作れるようにする。
var declaredCriteria = setting.SelectedActionsCriteria{VerifiedAllowed: true, PatternsAllowed: []string{"a/*", "b/*"}}

// TestExecutableChanges は、変更計画から導いた論理的な設定変更を、同じ並びのリポジトリの計画と組にすることを確かめる。
// 導出の規則は changeplan が持つため、ここでは組にすることと、導けないときに変更していないことを添えることだけを確かめる。
//
//trace:checks FR-009
func TestExecutableChanges(t *testing.T) {
	match := changeplan.SettingPlan{Unit: setting.Unit{Key: setting.DeleteBranchOnMerge}, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(true), Result: changeplan.Match}
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

	t.Run("each repository plan is paired with its own logical changes in the declared order", func(t *testing.T) {
		p := plans(t, []changeplan.SettingPlan{allowedActionsMismatch()}, []changeplan.SettingPlan{match}, []changeplan.SettingPlan{allowAutoMergeMismatch()})
		got, err := executableChanges(p)
		if err != nil {
			t.Fatalf("executableChanges() error = %v", err)
		}
		want := []repositoryChanges{
			{plan: p.Repositories[0], changes: []changeplan.LogicalChange{allowedActionsChange()}},
			{plan: p.Repositories[1]},
			{plan: p.Repositories[2], changes: []changeplan.LogicalChange{allowAutoMergeChange()}},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("executableChanges() = %+v, want %+v", got, want)
		}
	})

	t.Run("change plan whose logical changes cannot be derived returns an error saying no changes were made", func(t *testing.T) {
		got, err := executableChanges(plans(t, []changeplan.SettingPlan{shaPinningRequiredNotApplicable()}))
		if err == nil || !strings.HasSuffix(err.Error(), "; no changes made") {
			t.Errorf("executableChanges() = %+v, %v, want an error ending with ; no changes made", got, err)
		}
	})
}

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

//trace:checks INV-002 INV-003 INV-005 FR-008 FR-009 FR-013
func TestApplyPlan(t *testing.T) {
	approve := func(calls *int) func(context.Context) error {
		return func(context.Context) error {
			*calls++
			return nil
		}
	}

	match := changeplan.SettingPlan{Unit: setting.Unit{Key: setting.DeleteBranchOnMerge}, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(true), Result: changeplan.Match}

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

	t.Run("not-applicable changes in two repositories are all reported with the change plan without approval or requests", func(t *testing.T) {
		baseURL, server := newSequenceServer(t)
		var out bytes.Buffer
		calls := 0
		cfg := Config{Configuration: configuration.File{Path: "./reco.yaml"}, Out: &out, Approve: approve(&calls), After: elapsedAtOnce}
		p := changeplan.Plan{Repositories: []changeplan.RepositoryPlan{
			newRepositoryPlanOf(t, "owner/first", 1, shaPinningRequiredNotApplicable()),
			newRepositoryPlanOf(t, "owner/second", 2, shaPinningRequiredNotApplicable()),
		}}

		err := applyPlan(context.Background(), cfg, newClient(baseURL), p)

		// 報告の文言と全件の列挙は changeplan と changeplandisplay が持つため、apply は表示を書くことと no changes made を足すことだけを確かめる
		if wantErr := p.NotApplicableError().Error() + "; no changes made"; err == nil || err.Error() != wantErr {
			t.Errorf("applyPlan() error = %v, want %q", err, wantErr)
		}
		if wantOut := changeplandisplay.Text("./reco.yaml", p); out.String() != wantOut {
			t.Errorf("got Out %q, want %q", out.String(), wantOut)
		}
		if calls != 0 || len(server.Methods()) != 0 {
			t.Errorf("approve calls = %d, requests = %v, want none", calls, server.Methods())
		}
	})

	t.Run("applicable first repository is not updated when the second has a not-applicable change", func(t *testing.T) {
		baseURL, server := newSequenceServer(t)
		var out bytes.Buffer
		calls := 0
		p := changeplan.Plan{Repositories: []changeplan.RepositoryPlan{
			newRepositoryPlanOf(t, "owner/first", 1, allowAutoMergeMismatch()),
			newRepositoryPlanOf(t, "owner/second", 2, shaPinningRequiredNotApplicable()),
		}}

		err := applyPlan(context.Background(), Config{Out: &out, Approve: approve(&calls), After: elapsedAtOnce}, newClient(baseURL), p)

		if wantErr := p.NotApplicableError().Error() + "; no changes made"; err == nil || err.Error() != wantErr {
			t.Errorf("applyPlan() error = %v, want %q", err, wantErr)
		}
		if calls != 0 || len(server.Methods()) != 0 {
			t.Errorf("approve calls = %d, requests = %v, want none", calls, server.Methods())
		}
	})

	// 適用可能性の判断がない変更予定は Generate が作らないが、作られたときに承認へ進まないことを確かめる
	t.Run("planned change without an applicability judgment returns an error without output, approval, or requests", func(t *testing.T) {
		baseURL, server := newSequenceServer(t)
		var out bytes.Buffer
		calls := 0
		notJudged := allowAutoMergeMismatch()
		notJudged.Applicability = changeplan.Applicability{}

		err := applyPlan(context.Background(), Config{Out: &out, Approve: approve(&calls), After: elapsedAtOnce}, newClient(baseURL), newPlan(t, notJudged))

		if err == nil || !strings.HasSuffix(err.Error(), "; no changes made") {
			t.Errorf("applyPlan() error = %v, want an error ending with %q", err, "; no changes made")
		}
		if calls != 0 || out.Len() != 0 || len(server.Methods()) != 0 {
			t.Errorf("approve calls = %d, Out = %q, requests = %v, want none", calls, out.String(), server.Methods())
		}
	})

	t.Run("not-applicable change plan write failure is returned without approval", func(t *testing.T) {
		calls := 0
		err := applyPlan(context.Background(), Config{Out: failingWriter{}, Approve: approve(&calls)}, nil, newPlan(t, shaPinningRequiredNotApplicable()))

		if !errors.Is(err, errWrite) {
			t.Errorf("applyPlan() error = %v, want %v", err, errWrite)
		}
		if calls != 0 {
			t.Errorf("approve calls = %d, want none", calls)
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

// TestApplyPlan_RulesetValueUpdateAndCreation は、既存の Ruleset の値の変更と Ruleset の作成が混ざる変更計画を、
// 承認を一度だけ求めてから、宣言の並びで別々の論理的な設定変更として実行することを確かめる。
// 値の変更を先に宣言し、作成を先に並べ替える実装を失敗させる。
//
//trace:checks INV-003 FR-010 FR-011
func TestApplyPlan_RulesetValueUpdateAndCreation(t *testing.T) {
	protect := githubtest.RulesetSummaryBody{ID: 42, Name: "protect"}
	release := githubtest.RulesetSummaryBody{ID: 43, Name: "release"}
	updated := protectRulesetBody(42, "protect", setting.RulesetEnforcementActive)
	created := protectRulesetBody(43, "release", setting.RulesetEnforcementActive)
	baseURL, server := newSequenceServer(t,
		okRepository(1, false, false), okRulesets(protect), okRuleset(protectRulesetBody(42, "protect", setting.RulesetEnforcementDisabled)),
		okRuleset(updated),
		okRepository(1, false, false), okRulesets(protect), okRuleset(updated),
		okRepository(1, false, false), okRulesets(protect),
		createdRuleset(created),
		okRepository(1, false, false), okRulesets(protect, release), okRuleset(created),
	)
	calls := 0
	approve := func(context.Context) error {
		calls++
		return nil
	}
	var out bytes.Buffer
	p := newPlan(t, rulesetValueMismatch(t, "protect", 42), rulesetMismatch(t, "release", setting.Absent()))

	err := applyPlan(context.Background(), Config{Out: &out, Approve: approve, After: elapsedAtOnce}, newClient(baseURL), p)

	if err != nil {
		t.Fatalf("applyPlan() error = %v", err)
	}
	if calls != 1 {
		t.Errorf("approve calls = %d, want 1", calls)
	}
	wantRequests := []string{
		"GET /repos/owner/repo", "GET /repos/owner/repo/rulesets", "GET /repos/owner/repo/rulesets/42",
		"PUT /repos/owner/repo/rulesets/42",
		"GET /repos/owner/repo", "GET /repos/owner/repo/rulesets", "GET /repos/owner/repo/rulesets/42",
		"GET /repos/owner/repo", "GET /repos/owner/repo/rulesets",
		"POST /repos/owner/repo/rulesets",
		"GET /repos/owner/repo", "GET /repos/owner/repo/rulesets", "GET /repos/owner/repo/rulesets/43",
	}
	if !slices.Equal(server.RequestLines(), wantRequests) {
		t.Errorf("requests = %v, want %v", server.RequestLines(), wantRequests)
	}
	if !strings.Contains(out.String(), "Apply complete.") {
		t.Errorf("got Out %q, want it to contain %q", out.String(), "Apply complete.")
	}
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

//trace:checks INV-001 INV-002 INV-004 INV-005 FR-008 FR-011
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

	// 計画時の観測値を関係する現在値とし、更新直前と更新後の観測が Actions access の領域を読むことを確かめる
	t.Run("access_level of a private repository is applied after a single approval", func(t *testing.T) {
		baseURL, server := newSequenceServer(t,
			okPrivateRepository(1), okActionsAccess("none"),
			okPrivateRepository(1), okActionsAccess("none"), response{status: http.StatusNoContent},
			okPrivateRepository(1), okActionsAccess("user"))
		var out bytes.Buffer
		calls := 0
		files := fstest.MapFS{"reco.yaml": {Data: []byte("repositories:\n  - name: owner/repo\n    actions:\n      access:\n        access_level: user\n")}}

		if err := Run(context.Background(), newConfig(files, &out, baseURL, &calls)); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		reads := []string{"GET /repos/owner/repo", "GET /repos/owner/repo/actions/permissions/access"}
		want := slices.Concat(reads, reads, []string{"PUT /repos/owner/repo/actions/permissions/access"}, reads)
		if !slices.Equal(server.RequestLines(), want) {
			t.Errorf("requests = %v, want %v", server.RequestLines(), want)
		}
		if calls != 1 || !strings.Contains(out.String(), "actions.access.access_level: none -> user") || !strings.Contains(out.String(), "owner/repo: updated") {
			t.Errorf("approve calls = %d, Out = %q, want one approval, the access level change, and the repository updated", calls, out.String())
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

	// sha_pinning_required の変更は、先行する論理的な設定変更とは別の更新要求で、更新要求の間隔を置いてから送ることを、計画の生成から確かめる
	shaPinningCases := []struct {
		name       string
		source     string
		responses  []response
		wantBodies []map[string]any
		wantOut    string
	}{
		{
			name:   "disabled actions declaring enabled true and sha_pinning_required true is enabled and then updated",
			source: "repositories:\n  - name: owner/repo\n    actions:\n      permissions:\n        enabled: true\n        sha_pinning_required: true\n",
			responses: []response{
				okRepository(1, false, false), okActionsDisabled(),
				okRepository(1, false, false), okActionsDisabled(), {status: http.StatusNoContent}, okRepository(1, false, false), okAllowedActions("all", false),
				okRepository(1, false, false), okAllowedActions("all", false), {status: http.StatusNoContent}, okRepository(1, false, false), okAllowedActions("all", true),
			},
			wantBodies: []map[string]any{nil, nil, nil, nil, {"enabled": true}, nil, nil, nil, nil, {"enabled": true, "sha_pinning_required": true}, nil, nil},
			wantOut:    "Previous settings: GitHub Actions resumes with allowed_actions and selected_actions kept",
		},
		{
			name:   "policy and sha_pinning_required changes are applied in two update requests",
			source: "repositories:\n  - name: owner/repo\n    actions:\n      permissions:\n        allowed_actions: local_only\n        sha_pinning_required: true\n",
			responses: []response{
				okRepository(1, false, false), okAllowedActions("all", false),
				okRepository(1, false, false), okAllowedActions("all", false), {status: http.StatusNoContent}, okRepository(1, false, false), okAllowedActions("local_only", false),
				okRepository(1, false, false), okAllowedActions("local_only", false), {status: http.StatusNoContent}, okRepository(1, false, false), okAllowedActions("local_only", true),
			},
			wantBodies: []map[string]any{nil, nil, nil, nil, {"enabled": true, "allowed_actions": "local_only"}, nil, nil, nil, nil, {"enabled": true, "sha_pinning_required": true}, nil, nil},
			wantOut:    "actions.permissions.sha_pinning_required: false -> true",
		},
	}
	for _, tc := range shaPinningCases {
		t.Run(tc.name, func(t *testing.T) {
			baseURL, server := newSequenceServer(t, tc.responses...)
			var out bytes.Buffer
			calls := 0
			var requestsAtWait []int
			cfg := newConfig(fstest.MapFS{"reco.yaml": {Data: []byte(tc.source)}}, &out, baseURL, &calls)
			cfg.After = func(d time.Duration) <-chan time.Time {
				requestsAtWait = append(requestsAtWait, len(server.RequestLines()))
				return elapsedAtOnce(d)
			}

			if err := Run(context.Background(), cfg); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			readAndUpdate := []string{getRepository, getActionsPermissions, putActionsPermissions, getRepository, getActionsPermissions}
			want := slices.Concat([]string{getRepository, getActionsPermissions}, readAndUpdate, readAndUpdate)
			if !slices.Equal(server.RequestLines(), want) {
				t.Errorf("requests = %v, want %v", server.RequestLines(), want)
			}
			if !reflect.DeepEqual(server.Bodies(), tc.wantBodies) {
				t.Errorf("request bodies = %v, want %v", server.Bodies(), tc.wantBodies)
			}
			// 間隔は1つ目の更新後の観測のあと、2つ目の更新直前の観測より前に入る
			if want := []int{7}; !slices.Equal(requestsAtWait, want) {
				t.Errorf("requests received at each wait = %v, want %v", requestsAtWait, want)
			}
			if calls != 1 || !strings.Contains(out.String(), tc.wantOut) || !strings.Contains(out.String(), "owner/repo: updated") {
				t.Errorf("approve calls = %d, Out = %q, want one approval, %q, and the repository updated", calls, out.String(), tc.wantOut)
			}
		})
	}

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

	// 作成と、別の Environment の方式の変更を、宣言の並びの順に実行し、宣言していない Environment には要求を送らないことを確かめる
	t.Run("creation and a mode change of another environment are applied in the declared order without a request to an undeclared environment", func(t *testing.T) {
		production := githubtest.EnvironmentBody{ID: 8, Name: "production", DeploymentBranchPolicy: protectedBranchesBody()}
		stagingProtected := githubtest.EnvironmentBody{ID: 7, Name: "staging", DeploymentBranchPolicy: protectedBranchesBody()}
		stagingAll := githubtest.EnvironmentBody{ID: 7, Name: "staging"}
		qa := githubtest.EnvironmentBody{ID: 9, Name: "qa"}
		baseURL, server := newSequenceServer(t,
			okRepository(1, false, false), okEnvironments(stagingProtected, qa),
			okRepository(1, false, false), okEnvironments(stagingProtected, qa), okEnvironment(production),
			okRepository(1, false, false), okEnvironments(production, stagingProtected, qa),
			okRepository(1, false, false), okEnvironments(production, stagingProtected, qa), okEnvironment(stagingAll),
			okRepository(1, false, false), okEnvironments(production, stagingAll, qa))
		var out bytes.Buffer
		calls := 0
		files := fstest.MapFS{"reco.yaml": {Data: []byte(`repositories:
  - name: owner/repo
    environments:
      - name: production
        deployment_branch_policy:
          mode: protected_branches
      - name: Staging
        deployment_branch_policy:
          mode: all
`)}}

		if err := Run(context.Background(), newConfig(files, &out, baseURL, &calls)); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		reads := []string{"GET /repos/owner/repo", "GET /repos/owner/repo/environments"}
		want := slices.Concat(reads,
			reads, []string{"PUT /repos/owner/repo/environments/production"}, reads,
			reads, []string{"PUT /repos/owner/repo/environments/staging"}, reads)
		if !slices.Equal(server.RequestLines(), want) {
			t.Errorf("requests = %v, want %v", server.RequestLines(), want)
		}
		var bodies []map[string]any
		for _, r := range server.Requests() {
			if r.Method == http.MethodPut {
				bodies = append(bodies, r.Body)
			}
		}
		wantBodies := []map[string]any{
			{"deployment_branch_policy": map[string]any{"protected_branches": true, "custom_branch_policies": false}},
			{"deployment_branch_policy": nil},
		}
		if !reflect.DeepEqual(bodies, wantBodies) {
			t.Errorf("update request bodies = %v, want %v", bodies, wantBodies)
		}
		if calls != 1 || !strings.Contains(out.String(), "owner/repo: updated") || !strings.Contains(out.String(), "Apply complete.") {
			t.Errorf("approve calls = %d, Out = %q, want one approval and the repository updated", calls, out.String())
		}
	})

	t.Run("mode change to selected with a pattern is applied by the mode and the pattern after a single approval", func(t *testing.T) {
		all := githubtest.EnvironmentBody{ID: 7, Name: "production"}
		selected := githubtest.EnvironmentBody{ID: 7, Name: "production", DeploymentBranchPolicy: selectedBody()}
		main := githubtest.DeploymentPatternBody{ID: 10, Name: "main", Type: "branch"}
		baseURL, server := newSequenceServer(t,
			okRepository(1, false, false), okEnvironments(all),
			okRepository(1, false, false), okEnvironments(all), okEnvironment(selected),
			okRepository(1, false, false), okEnvironments(selected), okDeploymentPatterns(), okDeploymentPattern(main),
			okRepository(1, false, false), okEnvironments(selected), okDeploymentPatterns(main))
		var out bytes.Buffer
		calls := 0
		files := fstest.MapFS{"reco.yaml": {Data: []byte(`repositories:
  - name: owner/repo
    environments:
      - name: production
        deployment_branch_policy:
          mode: selected
          patterns:
            - type: branch
              name: main
`)}}

		if err := Run(context.Background(), newConfig(files, &out, baseURL, &calls)); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		reads := []string{"GET /repos/owner/repo", "GET /repos/owner/repo/environments"}
		readPatterns := append(slices.Clone(reads), "GET /repos/owner/repo/environments/production/deployment-branch-policies")
		want := slices.Concat(reads,
			reads, []string{"PUT /repos/owner/repo/environments/production"},
			readPatterns, []string{"POST /repos/owner/repo/environments/production/deployment-branch-policies"}, readPatterns)
		if !slices.Equal(server.RequestLines(), want) {
			t.Errorf("requests = %v, want %v", server.RequestLines(), want)
		}
		if calls != 1 || !strings.Contains(out.String(), "owner/repo: updated") || !strings.Contains(out.String(), "Apply complete.") {
			t.Errorf("approve calls = %d, Out = %q, want one approval and the repository updated", calls, out.String())
		}
	})

	t.Run("variables are created and updated in the declared order without a request to an undeclared variable", func(t *testing.T) {
		alphaBefore := githubtest.VariableBody{Name: "ALPHA", Value: "1"}
		alphaAfter := githubtest.VariableBody{Name: "ALPHA", Value: "2"}
		zeta := githubtest.VariableBody{Name: "ZETA", Value: "z"}
		other := githubtest.VariableBody{Name: "OTHER", Value: "o"}
		baseURL, server := newSequenceServer(t,
			okRepository(1, false, false), okVariables(alphaBefore, other),
			okRepository(1, false, false), okVariables(alphaBefore, other), createdVariable(),
			okRepository(1, false, false), okVariables(alphaBefore, other, zeta),
			okRepository(1, false, false), okVariables(alphaBefore, other, zeta), noContent(),
			okRepository(1, false, false), okVariables(alphaAfter, other, zeta))
		var out bytes.Buffer
		calls := 0
		// 宣言の並びを名前順と逆にし、名前順に並べ替える実装を失敗させる
		files := fstest.MapFS{"reco.yaml": {Data: []byte(`repositories:
  - name: owner/repo
    actions:
      variables:
        - name: ZETA
          value: z
        - name: ALPHA
          value: "2"
`)}}
		if err := Run(context.Background(), newConfig(files, &out, baseURL, &calls)); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		reads := []string{"GET /repos/owner/repo", "GET /repos/owner/repo/actions/variables"}
		want := slices.Concat(reads,
			reads, []string{"POST /repos/owner/repo/actions/variables"}, reads,
			reads, []string{"PATCH /repos/owner/repo/actions/variables/ALPHA"}, reads)
		if !slices.Equal(server.RequestLines(), want) {
			t.Errorf("requests = %v, want %v", server.RequestLines(), want)
		}
		if calls != 1 || !strings.Contains(out.String(), "owner/repo: updated") || !strings.Contains(out.String(), "Apply complete.") {
			t.Errorf("approve calls = %d, Out = %q, want one approval and the repository updated", calls, out.String())
		}
	})

	// Environment の作成と変数の作成は別の論理的な設定変更で、作成が完了してから変数を作成することを、計画の生成から確かめる
	t.Run("variable of an absent environment is created after the environment in two update requests after a single approval", func(t *testing.T) {
		production := githubtest.EnvironmentBody{ID: 8, Name: "production"}
		baseURL, server := newSequenceServer(t,
			okRepository(1, false, false), okEnvironments(),
			okRepository(1, false, false), okEnvironments(), okEnvironment(production),
			okRepository(1, false, false), okEnvironments(production),
			okRepository(1, false, false), okEnvironments(production), okVariables(), createdVariable(),
			okRepository(1, false, false), okEnvironments(production), okVariables(githubtest.VariableBody{Name: "REGION", Value: "ap-northeast-1"}))
		var out bytes.Buffer
		calls := 0
		files := fstest.MapFS{"reco.yaml": {Data: []byte(`repositories:
  - name: owner/repo
    environments:
      - name: production
        variables:
          - name: REGION
            value: ap-northeast-1
`)}}

		if err := Run(context.Background(), newConfig(files, &out, baseURL, &calls)); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		reads := []string{"GET /repos/owner/repo", "GET /repos/owner/repo/environments"}
		readVariables := append(slices.Clone(reads), "GET /repos/owner/repo/environments/production/variables")
		want := slices.Concat(reads,
			reads, []string{"PUT /repos/owner/repo/environments/production"}, reads,
			readVariables, []string{"POST /repos/owner/repo/environments/production/variables"}, readVariables)
		if !slices.Equal(server.RequestLines(), want) {
			t.Errorf("requests = %v, want %v", server.RequestLines(), want)
		}
		if calls != 1 || !strings.Contains(out.String(), "owner/repo: updated") || !strings.Contains(out.String(), "Apply complete.") {
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

	// 1件目は Actions が無効のまま sha_pinning_required だけを宣言し、適用不能になる
	notApplicableFirstSource := "repositories:\n" +
		"  - name: owner/first\n    actions:\n      permissions:\n        sha_pinning_required: true\n" +
		"  - name: owner/second\n    general:\n      allow_auto_merge: true\n"

	t.Run("observation failure of the second repository is reported instead of the not-applicable first", func(t *testing.T) {
		baseURL, server := newSequenceServer(t, okRepository(1, false, false), okActionsDisabled(), response{status: http.StatusNotFound})
		var out bytes.Buffer
		calls := 0
		files := fstest.MapFS{"reco.yaml": {Data: []byte(notApplicableFirstSource)}}

		err := Run(context.Background(), newConfig(files, &out, baseURL, &calls))

		if !errors.Is(err, github.ErrNotFound) {
			t.Errorf("Run() error = %v, want %v", err, github.ErrNotFound)
		}
		if out.Len() != 0 || calls != 0 {
			t.Errorf("Out = %q, approve calls = %d, want neither", out.String(), calls)
		}
		want := []string{"GET /repos/owner/first", "GET /repos/owner/first/actions/permissions", "GET /repos/owner/second"}
		if !slices.Equal(server.RequestLines(), want) {
			t.Errorf("requests = %v, want %v", server.RequestLines(), want)
		}
	})

	t.Run("selected repository without not-applicable changes is not stopped by the unselected one", func(t *testing.T) {
		baseURL, server := newSequenceServer(t, okRepository(2, true, false))
		var out bytes.Buffer
		calls := 0
		files := fstest.MapFS{"reco.yaml": {Data: []byte(notApplicableFirstSource)}}
		cfg := newConfig(files, &out, baseURL, &calls)
		selection := mustReference(t, "owner/second")
		cfg.Selection = &selection

		if err := Run(context.Background(), cfg); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if want := "No changes.\nRepositories checked: 1\n"; out.String() != want {
			t.Errorf("got Out %q, want %q", out.String(), want)
		}
		if want := []string{"GET /repos/owner/second"}; !slices.Equal(server.RequestLines(), want) {
			t.Errorf("requests = %v, want %v", server.RequestLines(), want)
		}
	})
}

func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }

// actionsPermissionsConfigSource は、allowed_actions だけを policy で宣言する。
func actionsPermissionsConfigSource(policy string) string {
	return "repositories:\n  - name: owner/repo\n    actions:\n      permissions:\n        allowed_actions: " + policy + "\n"
}

// allowAutoMergeMismatch は、allowAutoMergeChange を導く適用可能な変更予定。
func allowAutoMergeMismatch() changeplan.SettingPlan {
	return changeplan.SettingPlan{
		Unit:          setting.Unit{Key: setting.AllowAutoMerge},
		Observed:      github.Observed(setting.BoolValue(false)),
		Desired:       setting.BoolValue(true),
		Result:        changeplan.Mismatch,
		Applicability: changeplan.Applicable(),
	}
}

// deleteBranchOnMergeMismatch は、deleteBranchOnMergeChange を導く適用可能な変更予定。
func deleteBranchOnMergeMismatch() changeplan.SettingPlan {
	return changeplan.SettingPlan{
		Unit:          setting.Unit{Key: setting.DeleteBranchOnMerge},
		Observed:      github.Observed(setting.BoolValue(false)),
		Desired:       setting.BoolValue(true),
		Result:        changeplan.Mismatch,
		Applicability: changeplan.Applicable(),
	}
}

// allowedActionsMismatch は、allowedActionsChange を導く適用可能な変更予定。
func allowedActionsMismatch() changeplan.SettingPlan {
	operation := allowedActionsChange().Operations[0]
	return changeplan.SettingPlan{
		Unit: operation.Unit, Observed: github.Observed(setting.AllowedActionsPolicyValue(setting.AllowedActionsAll)), Desired: operation.Value, Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
	}
}

// selectedToAllChange は、Actions 許可ポリシーを selected から all にし、lostCriteria を失わせる副作用を伴う論理的な設定変更。
// 失われる値も変更前提の基準に並ぶ。
func selectedToAllChange() changeplan.LogicalChange {
	return singleUpdate(setting.AllowedActions, setting.AllowedActionsPolicyValue(setting.AllowedActionsAll), []changeplan.RelevantSetting{
		{Unit: setting.Unit{Key: setting.AllowedActions}, AnticipatedValue: changeplan.AnticipateValue(setting.AllowedActionsPolicyValue(setting.AllowedActionsSelected))},
		{Unit: setting.Unit{Key: setting.SelectedActions}, AnticipatedValue: changeplan.AnticipateValue(setting.SelectedActionsCriteriaValue(lostCriteria))},
	})
}

// enableActionsNotComparable は、Actions が無効のリポジトリで Actions 許可ポリシーを local_only と宣言した、
// 比較不能で適用可能な変更予定。選択方式の Actions 許可条件が失われる可能性を副作用として伴う。
func enableActionsNotComparable() changeplan.SettingPlan {
	return changeplan.SettingPlan{
		Unit: setting.Unit{Key: setting.AllowedActions}, Observed: github.Unavailable[setting.Value](github.UnavailableActionsDisabled),
		Desired: setting.AllowedActionsPolicyValue(setting.AllowedActionsLocalOnly), Result: changeplan.NotComparable, Applicability: changeplan.Applicable(),
		SideEffects: []changeplan.SideEffect{{Kind: changeplan.SideEffectSelectedActionsDiscarded, Observed: github.Unavailable[setting.Value](github.UnavailableActionsDisabled)}},
	}
}

// enableSelectedChange は、Actions が無効のリポジトリで Actions を有効にして Actions 許可ポリシーを selected にし、
// 選択方式の Actions 許可条件を declaredCriteria にする、二つの更新操作からなる論理的な設定変更。
func enableSelectedChange() changeplan.LogicalChange {
	change := toSelectedChange()
	change.Operations[0].RelevantSettings = []changeplan.RelevantSetting{
		{Unit: setting.Unit{Key: setting.AllowedActions}, AnticipatedValue: changeplan.AnticipateUnchanged(github.Unavailable[setting.Value](github.UnavailableActionsDisabled))},
	}
	return change
}

// toSelectedChange は、Actions 許可ポリシーを local_only から selected にし、選択方式の Actions 許可条件を declaredCriteria にする、
// 二つの更新操作からなる論理的な設定変更。2つ目の基準は、1つ目による予定どおりの変化を反映したポリシーだけである。
func toSelectedChange() changeplan.LogicalChange {
	policy, criteria := setting.Unit{Key: setting.AllowedActions}, setting.Unit{Key: setting.SelectedActions}
	selected, declared := setting.AllowedActionsPolicyValue(setting.AllowedActionsSelected), setting.SelectedActionsCriteriaValue(declaredCriteria)
	return changeplan.LogicalChange{
		DesiredState: []changeplan.DesiredSetting{{Unit: policy, Value: selected}, {Unit: criteria, Value: declared}},
		Operations: []changeplan.UpdateOperation{
			{
				Kind: changeplan.SettingUpdate, Unit: policy, Value: selected,
				RelevantSettings: relevantTo(setting.AllowedActions, setting.AllowedActionsPolicyValue(setting.AllowedActionsLocalOnly)),
			},
			{
				Kind: changeplan.SettingUpdate, Unit: criteria, Value: declared,
				RelevantSettings: relevantTo(setting.AllowedActions, selected),
			},
		},
	}
}

// shaPinningRequiredNotApplicable は、Actions が無効で観測不能だった sha_pinning_required の、理由つきの適用不能な変更予定。
func shaPinningRequiredNotApplicable() changeplan.SettingPlan {
	return changeplan.SettingPlan{
		Unit: setting.Unit{Key: setting.SHAPinningRequired}, Observed: github.Unavailable[setting.Value](github.UnavailableActionsDisabled), Desired: setting.BoolValue(true),
		Result: changeplan.NotComparable, Applicability: changeplan.NotApplicable(changeplan.NotApplicableActionsRemainDisabled),
	}
}

func newPlan(t *testing.T, settings ...changeplan.SettingPlan) changeplan.Plan {
	t.Helper()
	return changeplan.Plan{Repositories: []changeplan.RepositoryPlan{newRepositoryPlan(t, settings...)}}
}

func mustVariableName(t *testing.T, name string) setting.VariableName {
	t.Helper()
	variable, err := setting.ParseVariableName(name)
	if err != nil {
		t.Fatalf("ParseVariableName(%q) error = %v", name, err)
	}
	return variable
}
