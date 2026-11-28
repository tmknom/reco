package apply

import (
	"reflect"
	"testing"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// TestMatchesAnticipatedValue は、承認時に予定した値と更新直前の設定の観測結果が同じかの規則を確かめる。
//
//trace:checks INV-001 INV-004 FR-010
func TestMatchesAnticipatedValue(t *testing.T) {
	disabled := github.Unavailable[setting.Value](github.UnavailableActionsDisabled)
	notSelected := github.Unavailable[setting.Value](github.UnavailableNotSelected)
	criteria := func(patterns ...string) setting.Value {
		return setting.SelectedActionsCriteriaValue(setting.SelectedActionsCriteria{PatternsAllowed: patterns})
	}
	cases := []struct {
		name        string
		anticipated changeplan.AnticipatedValue
		current     github.SettingObservation[setting.Value]
		want        bool
	}{
		{name: "unavailable for the same reason is the same", anticipated: changeplan.AnticipateUnchanged(disabled), current: disabled, want: true},
		{name: "unavailable for another reason differs", anticipated: changeplan.AnticipateUnchanged(disabled), current: notSelected, want: false},
		{name: "observed value where unavailable was approved differs", anticipated: changeplan.AnticipateUnchanged(disabled), current: github.Observed(criteria("a/*")), want: false},
		{name: "unavailable where a value was approved differs", anticipated: changeplan.AnticipateValue(criteria("a/*")), current: disabled, want: false},
		{name: "another observed value differs", anticipated: changeplan.AnticipateValue(criteria("a/*")), current: github.Observed(criteria("b/*")), want: false},
		{name: "observed values differing only in the order of patterns are the same", anticipated: changeplan.AnticipateValue(criteria("a/*", "b/*")), current: github.Observed(criteria("b/*", "a/*")), want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchesAnticipatedValue(tc.anticipated, tc.current); got != tc.want {
				t.Errorf("matchesAnticipatedValue() = %v, want %v", got, tc.want)
			}
		})
	}
}

//trace:checks INV-001 INV-004 FR-010
func TestCheckPrecondition(t *testing.T) {
	operation := allowAutoMergeChange().Operations[0]
	cases := []struct {
		name        string
		observed    github.ObservationResult
		want        preconditionCheck
		wantChanged setting.Key
	}{
		{
			name:     "same identity and current value hold",
			observed: github.ObservationResult{RepositoryID: 1, General: github.General{AllowAutoMerge: false}},
			want:     preconditionHolds,
		},
		{
			name:     "changed value of another setting alone holds",
			observed: github.ObservationResult{RepositoryID: 1, General: github.General{AllowAutoMerge: false, DeleteBranchOnMerge: true}},
			want:     preconditionHolds,
		},
		{
			name:     "different ID alone is an identity mismatch",
			observed: github.ObservationResult{RepositoryID: 2, General: github.General{AllowAutoMerge: false}},
			want:     preconditionIdentityMismatch,
		},
		{
			name:        "changed current value alone is a current value change of the setting even if it equals the desired value",
			observed:    github.ObservationResult{RepositoryID: 1, General: github.General{AllowAutoMerge: true}},
			want:        preconditionCurrentValueChanged,
			wantChanged: setting.AllowAutoMerge,
		},
		{
			name:     "different ID and current value is an identity mismatch",
			observed: github.ObservationResult{RepositoryID: 2, General: github.General{AllowAutoMerge: true}},
			want:     preconditionIdentityMismatch,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got, changed := checkPrecondition(1, operation, tc.observed, github.EnvironmentObservation{}); got != tc.want || changed != (setting.Unit{Key: tc.wantChanged}) {
				t.Errorf("checkPrecondition() = %v, %q, want %v, %q", got, changed.Key, tc.want, tc.wantChanged)
			}
		})
	}

	// 変更前提の基準が複数並ぶ変更と、承認時に予定した値が観測不能の変更。値の比べ方は TestMatchesAnticipatedValue が確かめる
	actionsCases := []struct {
		name   string
		change changeplan.LogicalChange
		// index は、確かめる更新操作の change.Operations の添字。
		index       int
		observed    *github.ActionsPermissions
		want        preconditionCheck
		wantChanged setting.Key
	}{
		{
			// 2つ目がすべて上書きする3項目は基準に並ばないため、GitHub が戻した値でも成立する
			name:     "second update to selected holds with the policy selected and the criteria GitHub set back",
			change:   toSelectedChange(),
			index:    1,
			observed: actionsSelected(setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, PatternsAllowed: []string{}}),
			want:     preconditionHolds,
		},
		{
			name:        "second update to selected with the policy not selected is a current value change of allowed_actions",
			change:      toSelectedChange(),
			index:       1,
			observed:    actionsEnabled(setting.AllowedActionsLocalOnly, false),
			want:        preconditionCurrentValueChanged,
			wantChanged: setting.AllowedActions,
		},
		{
			name:        "selected to all with criteria other than shown before approval is a current value change of selected_actions",
			change:      selectedToAllChange(),
			observed:    actionsSelected(setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, PatternsAllowed: []string{"a/*", "b/*", "c/*"}}),
			want:        preconditionCurrentValueChanged,
			wantChanged: setting.SelectedActions,
		},
		{
			name:        "selected to all with the policy no longer selected is a current value change of allowed_actions",
			change:      selectedToAllChange(),
			observed:    actionsEnabled(setting.AllowedActionsLocalOnly, false),
			want:        preconditionCurrentValueChanged,
			wantChanged: setting.AllowedActions,
		},
		{
			name:     "enabling actions holds while actions is still disabled",
			change:   enableActionsChange(),
			observed: actionsDisabled(),
			want:     preconditionHolds,
		},
	}
	for _, tc := range actionsCases {
		t.Run(tc.name, func(t *testing.T) {
			observed := github.ObservationResult{RepositoryID: 1, ActionsPermissions: tc.observed}
			if got, changed := checkPrecondition(1, tc.change.Operations[tc.index], observed, github.EnvironmentObservation{}); got != tc.want || changed != (setting.Unit{Key: tc.wantChanged}) {
				t.Errorf("checkPrecondition() = %v, %q, want %v, %q", got, changed.Key, tc.want, tc.wantChanged)
			}
		})
	}

	t.Run("update operation without precondition bases panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("checkPrecondition() did not panic")
			}
		}()
		empty := changeplan.UpdateOperation{Kind: changeplan.SettingUpdate, Unit: setting.Unit{Key: setting.AllowAutoMerge}, Value: setting.BoolValue(true)}
		checkPrecondition(1, empty, github.ObservationResult{RepositoryID: 1}, github.EnvironmentObservation{})
	})
}

// TestPrecedingUpdateNotReflected は、後続の更新操作の直前の観測が、先行する更新操作の直前の状態のままかの規則を確かめる。
// 真のときだけ反映の遅れの可能性として観測し直すため、偽の側に倒れる形を確かめる。
//
//trace:checks INV-004 INV-005
func TestPrecedingUpdateNotReflected(t *testing.T) {
	cases := []struct {
		name     string
		change   changeplan.LogicalChange
		index    int
		observed *github.ActionsPermissions
		want     bool
	}{
		{
			name:     "first update has no preceding update even if the observation is the value before it",
			change:   toSelectedChange(),
			index:    0,
			observed: actionsEnabled(setting.AllowedActionsLocalOnly, false),
			want:     false,
		},
		{
			name:     "policy still local_only as before the first update is not reflected",
			change:   toSelectedChange(),
			index:    1,
			observed: actionsEnabled(setting.AllowedActionsLocalOnly, false),
			want:     true,
		},
		{
			name:     "actions still disabled as before the first update is not reflected",
			change:   enableSelectedChange(),
			index:    1,
			observed: actionsDisabled(),
			want:     true,
		},
		{
			name:     "policy selected is reflected",
			change:   toSelectedChange(),
			index:    1,
			observed: actionsSelected(setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, PatternsAllowed: []string{}}),
			want:     false,
		},
		{
			name:     "policy neither the value before the first update nor selected is not a delay",
			change:   toSelectedChange(),
			index:    1,
			observed: actionsEnabled(setting.AllowedActionsAll, false),
			want:     false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			observed := github.ObservationResult{RepositoryID: 1, ActionsPermissions: tc.observed}
			if got := precedingUpdateNotReflected(1, tc.change.Operations[:tc.index], observed, github.EnvironmentObservation{}); got != tc.want {
				t.Errorf("precedingUpdateNotReflected() = %v, want %v", got, tc.want)
			}
		})
	}

	main, feature := branchPattern("main"), branchPattern("feature/*")
	environmentCases := []struct {
		name         string
		change       changeplan.LogicalChange
		index        int
		environments *github.Environments
		want         bool
	}{
		{
			name:         "pattern addition with the set still as before the preceding removal is not reflected",
			change:       deploymentPatternsChange(t, "staging", selectedEnvironment(7, "Staging", []setting.DeploymentPattern{main, feature}, 10, 11), selectedPolicy(main, branchPattern("release/*"))),
			index:        1,
			environments: environments(selectedEnvironment(7, "Staging", []setting.DeploymentPattern{main, feature}, 10, 11)),
			want:         true,
		},
		{
			name:         "pattern addition after a creation with the environment still absent is not reflected",
			change:       creationWithPatternsChange(t, "production", selectedPolicy(main)),
			index:        1,
			environments: environments(),
			want:         true,
		},
	}
	for _, tc := range environmentCases {
		t.Run(tc.name, func(t *testing.T) {
			observed := github.ObservationResult{RepositoryID: 1, Environments: tc.environments}
			if got := precedingUpdateNotReflected(1, tc.change.Operations[:tc.index], observed, github.EnvironmentObservation{}); got != tc.want {
				t.Errorf("precedingUpdateNotReflected() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestMatchesPreconditionBasis_EnvironmentExistence は、承認時に予定した Environment の存在と同一性の基準が、更新直前の観測と同じかの規則を確かめる。
//
//trace:checks INV-001 INV-004 FR-010
func TestMatchesPreconditionBasis_EnvironmentExistence(t *testing.T) {
	name := mustEnvironmentName(t, "production")
	unit := setting.Unit{Key: setting.EnvironmentResource, Environment: name}
	lookup := func(existing ...github.Environment) github.EnvironmentObservation {
		return github.Environments{Existing: existing}.Lookup(name)
	}
	absentBasis := changeplan.PreconditionBasis{Unit: unit, AnticipatedValue: changeplan.AnticipateValue(setting.Absent())}
	approvedBasis := changeplan.PreconditionBasis{Unit: unit, AnticipatedValue: changeplan.AnticipateValue(setting.Present()), AnticipatedEnvironmentIdentity: changeplan.ObservedEnvironmentIdentity(7)}
	createdBasis := changeplan.PreconditionBasis{Unit: unit, AnticipatedValue: changeplan.AnticipateValue(setting.Present()), AnticipatedEnvironmentIdentity: changeplan.CreatedByPrecedingOperation()}
	approved := observedEnvironment(7, "production", setting.DeploymentBranchPolicyAll)
	undetermined := observedEnvironment(9, "prodüction", setting.DeploymentBranchPolicyAll)
	createdBase := lookup(observedEnvironment(8, "production", setting.DeploymentBranchPolicySelected))
	cases := []struct {
		name    string
		basis   changeplan.PreconditionBasis
		current []github.Environment
		// created は、同じ論理的な設定変更が作成した Environment の ID の基準。基準がないことはゼロ値で表す。
		created github.EnvironmentObservation
		want    bool
	}{
		{name: "absent at approval and still absent is the same", basis: absentBasis, current: []github.Environment{observedEnvironment(9, "staging", setting.DeploymentBranchPolicyAll)}, want: true},
		{name: "absent at approval and existing now differs", basis: absentBasis, current: []github.Environment{approved}, want: false},
		{name: "absent at approval and undetermined now differs", basis: absentBasis, current: []github.Environment{undetermined}, want: false},
		{
			name: "existing with the same ID is the same even if the spelling and the mode differ", basis: approvedBasis,
			current: []github.Environment{observedEnvironment(7, "PRODUCTION", setting.DeploymentBranchPolicyProtectedBranches)}, want: true,
		},
		{name: "existing with another ID differs", basis: approvedBasis, current: []github.Environment{observedEnvironment(8, "production", setting.DeploymentBranchPolicyAll)}, want: false},
		{name: "existing at approval and absent now differs", basis: approvedBasis, want: false},
		{name: "created without a base ID yet is as expected when it exists", basis: createdBasis, current: []github.Environment{observedEnvironment(8, "production", setting.DeploymentBranchPolicySelected)}, want: true},
		{
			name: "created with an ID other than the base is not as expected", basis: createdBasis, created: createdBase,
			current: []github.Environment{observedEnvironment(9, "production", setting.DeploymentBranchPolicySelected)}, want: false,
		},
		{
			name: "created with the base ID is as expected", basis: createdBasis, created: createdBase,
			current: []github.Environment{observedEnvironment(8, "Production", setting.DeploymentBranchPolicyAll)}, want: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			observed := github.ObservationResult{RepositoryID: 1, Environments: environments(tc.current...)}
			if got := matchesPreconditionBasis(tc.basis, observed, tc.created); got != tc.want {
				t.Errorf("matchesPreconditionBasis() = %v, want %v", got, tc.want)
			}
		})
	}

	// changeplan が作らない基準。承認の前に適用不能として止める観測不能と、同一性の基準のない存在は、更新直前の観測によらず不変条件の破れとして止める。
	// 更新直前の観測を、基準を受け付けたときに成立する側の形にして、比べた結果で隠れないことを確かめる
	panicCases := []struct {
		name    string
		basis   changeplan.PreconditionBasis
		current []github.Environment
	}{
		{
			name:    "undetermined at approval panics even if still undetermined",
			basis:   changeplan.PreconditionBasis{Unit: unit, AnticipatedValue: changeplan.AnticipateUnchanged(github.Unavailable[setting.Value](github.UnavailableNameUndetermined))},
			current: []github.Environment{undetermined},
		},
		{name: "zero value at approval panics even if the environment exists", basis: changeplan.PreconditionBasis{Unit: unit}, current: []github.Environment{approved}},
		{name: "existing at approval without an identity panics even if absent now", basis: changeplan.PreconditionBasis{Unit: unit, AnticipatedValue: changeplan.AnticipateValue(setting.Present())}},
		{
			// 作成した Environment の基準として受け付けると、失われるものを比べずに削除し得る
			name: "loss shown at approval without an ID panics even if the created environment exists",
			basis: func() changeplan.PreconditionBasis {
				b := createdBasis
				b.AnticipatedEnvironmentLoss = &changeplan.EnvironmentLoss{}
				return b
			}(),
			current: []github.Environment{approved},
		},
		{
			name: "loss shown at approval with absence panics even if still absent",
			basis: func() changeplan.PreconditionBasis {
				b := absentBasis
				b.AnticipatedEnvironmentLoss = &changeplan.EnvironmentLoss{}
				return b
			}(),
		},
	}
	for _, tc := range panicCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("matchesPreconditionBasis() did not panic")
				}
			}()
			matchesPreconditionBasis(tc.basis, github.ObservationResult{RepositoryID: 1, Environments: environments(tc.current...)}, github.EnvironmentObservation{})
		})
	}
}

// TestMatchesAnticipatedEnvironmentLoss は、承認の前に示した削除で失われるもの（失われる値、変数の一覧、Secret の一覧）が、更新直前に引いた Environment と同じかの規則を確かめる。
// 失われる値の項目ごとの比べ方は setting の EnvironmentDeletedValue.Equal、観測不能の理由の導き方は github の Environment.DeletedValue が確かめる。
//
//trace:checks INV-001 INV-004 FR-010
func TestMatchesAnticipatedEnvironmentLoss(t *testing.T) {
	region := github.Variable{Name: "REGION", Value: "ap-northeast-1"}
	tier := github.Variable{Name: "TIER", Value: "1"}
	deployKey := github.EnvironmentSecret{Name: "DEPLOY_KEY", UpdatedAt: "2026-01-01T00:00:00Z"}
	token := github.EnvironmentSecret{Name: "TOKEN", UpdatedAt: "2026-01-02T00:00:00Z"}
	// with は、承認時の観測 deletableEnvironment を change で一か所だけ変えた現在の観測を作る
	with := func(change func(e github.Environment) github.Environment) github.Environment {
		return change(deletableEnvironment(7, "production"))
	}
	// empty は、変数も Secret もない以外は deletableEnvironment と同じ観測
	empty := func() github.Environment {
		return withSecrets(withVariables(deletableEnvironment(7, "production")))
	}
	cases := []struct {
		name     string
		approved github.Environment
		current  github.Environment
		want     bool
	}{
		{name: "same loss is the same", approved: deletableEnvironment(7, "production"), current: deletableEnvironment(7, "production"), want: true},
		{
			name: "variables and secrets differing only in order are the same", approved: deletableEnvironment(7, "production"),
			current: with(func(e github.Environment) github.Environment {
				return withSecrets(withVariables(e, tier, region), token, deployKey)
			}), want: true,
		},
		{
			name: "changed wait timer differs", approved: deletableEnvironment(7, "production"),
			current: with(func(e github.Environment) github.Environment { e.Protection.WaitTimer = 31; return e }), want: false,
		},
		{
			// 承認のあとに知らない種類の保護ルールが現れた Environment は、失われる値を示せないため、同じとしない
			name: "deleted value that became unavailable differs", approved: deletableEnvironment(7, "production"),
			current: with(func(e github.Environment) github.Environment {
				e.UninterpretedProtectionRules = []github.UninterpretedPart{{Kind: github.UninterpretedProtectionRuleType, Name: "custom"}}
				return e
			}),
			want: false,
		},
		{
			name: "added variable differs", approved: deletableEnvironment(7, "production"),
			current: with(func(e github.Environment) github.Environment {
				return withVariables(e, region, tier, github.Variable{Name: "ZONE", Value: "a"})
			}),
			want: false,
		},
		{name: "removed variable differs", approved: deletableEnvironment(7, "production"), current: with(func(e github.Environment) github.Environment { return withVariables(e, region) }), want: false},
		{
			// 件数は同じで、予定した変数の一つが別の名前に置き換わった
			name: "variable replaced by another name differs", approved: deletableEnvironment(7, "production"),
			current: with(func(e github.Environment) github.Environment {
				return withVariables(e, region, github.Variable{Name: "ZONE", Value: "1"})
			}), want: false,
		},
		{
			name: "variable with a changed value differs", approved: deletableEnvironment(7, "production"),
			current: with(func(e github.Environment) github.Environment {
				return withVariables(e, region, github.Variable{Name: "TIER", Value: "2"})
			}), want: false,
		},
		{
			name: "variable name differing only in case is the same", approved: deletableEnvironment(7, "production"),
			current: with(func(e github.Environment) github.Environment {
				return withVariables(e, github.Variable{Name: "region", Value: "ap-northeast-1"}, tier)
			}), want: true,
		},
		{
			name: "added secret differs", approved: deletableEnvironment(7, "production"),
			current: with(func(e github.Environment) github.Environment {
				return withSecrets(e, deployKey, token, github.EnvironmentSecret{Name: "NEW", UpdatedAt: "2026-01-03T00:00:00Z"})
			}),
			want: false,
		},
		{name: "removed secret differs", approved: deletableEnvironment(7, "production"), current: with(func(e github.Environment) github.Environment { return withSecrets(e, deployKey) }), want: false},
		{
			// 件数は同じで、予定した Secret の一つが別の名前に置き換わった
			name: "secret replaced by another name differs", approved: deletableEnvironment(7, "production"),
			current: with(func(e github.Environment) github.Environment {
				return withSecrets(e, deployKey, github.EnvironmentSecret{Name: "OTHER", UpdatedAt: token.UpdatedAt})
			}),
			want: false,
		},
		{
			// 登録し直された Secret は、値を読めなくても承認の前に示したものと同じとはいえない
			name: "secret registered again differs", approved: deletableEnvironment(7, "production"),
			current: with(func(e github.Environment) github.Environment {
				return withSecrets(e, deployKey, github.EnvironmentSecret{Name: "TOKEN", UpdatedAt: "2026-01-03T00:00:00Z"})
			}),
			want: false,
		},
		{
			name: "secret name differing only in case differs", approved: deletableEnvironment(7, "production"),
			current: with(func(e github.Environment) github.Environment {
				return withSecrets(e, github.EnvironmentSecret{Name: "deploy_key", UpdatedAt: deployKey.UpdatedAt}, token)
			}),
			want: false,
		},
		{name: "no variables or secrets at approval and none now is the same", approved: empty(), current: empty(), want: true},
		{name: "no variables at approval and a variable now differs", approved: empty(), current: withVariables(empty(), region), want: false},
		{name: "no secrets at approval and a secret now differs", approved: empty(), current: withSecrets(empty(), deployKey), want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchesAnticipatedEnvironmentLoss(lossOf(t, tc.approved), tc.current); got != tc.want {
				t.Errorf("matchesAnticipatedEnvironmentLoss() = %v, want %v", got, tc.want)
			}
		})
	}

	// 読んでいない一覧を空として比べると、承認のあとに足された変数か Secret ごと消すため、更新直前の観測によらず読む範囲の破れとして止める。
	// 承認時の一覧を空にし、失われる値も変えて、空として比べる実装と、失われる値の違いで一覧を引かずに偽を返す実装を失敗させる
	unread := func(change func(e *github.Environment)) github.Environment {
		e := empty()
		e.Protection.WaitTimer = 31
		change(&e)
		return e
	}
	panicCases := []struct {
		name    string
		current github.Environment
	}{
		{name: "current without its variables read panics", current: unread(func(e *github.Environment) { e.Variables = nil })},
		{name: "current without its secrets read panics", current: unread(func(e *github.Environment) { e.Secrets = nil })},
	}
	for _, tc := range panicCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("matchesAnticipatedEnvironmentLoss() did not panic")
				}
			}()
			matchesAnticipatedEnvironmentLoss(lossOf(t, empty()), tc.current)
		})
	}
}

// TestMatchesPreconditionBasis_EnvironmentDeletion は、Environment の削除の基準が、存在と ID に加えて、承認の前に示した削除で失われるものを比べる規則を確かめる。
// 失われるものの比べ方は TestMatchesAnticipatedEnvironmentLoss が確かめる。
//
//trace:checks INV-001 INV-004 FR-010
func TestMatchesPreconditionBasis_EnvironmentDeletion(t *testing.T) {
	basis := environmentDeletionChange(t, "production", deletableEnvironment(7, "production")).Operations[0].PreconditionBases[0]
	changed := deletableEnvironment(7, "production")
	changed.Protection.CanAdminsBypass = false
	cases := []struct {
		name    string
		current []github.Environment
		want    bool
	}{
		{name: "same ID and the same loss is the same", current: []github.Environment{deletableEnvironment(7, "PRODUCTION")}, want: true},
		{name: "same ID with a different loss differs", current: []github.Environment{changed}, want: false},
		{name: "another ID with the same loss differs", current: []github.Environment{deletableEnvironment(8, "production")}, want: false},
		// 存在しない Environment は失われるものを引かずに不成立にする。引けば読んでいない一覧で panic する
		{name: "absent environment differs", current: []github.Environment{deletableEnvironment(9, "staging")}, want: false},
		{name: "only an undetermined name differs", current: []github.Environment{deletableEnvironment(7, "prodüction")}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			observed := github.ObservationResult{RepositoryID: 1, Environments: environments(tc.current...)}
			if got := matchesPreconditionBasis(basis, observed, github.EnvironmentObservation{}); got != tc.want {
				t.Errorf("matchesPreconditionBasis() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestCheckPrecondition_ReviewerSetRemoval は、担当者をすべて外す更新操作の変更前提が、担当者の集合のあとに同じ Environment の自己承認の禁止を
// 計画時の値と比べ、変わっていた設定のうち基準の並びで最初のものを返す規則を確かめる。
//
//trace:checks INV-001 INV-004 FR-010
func TestCheckPrecondition_ReviewerSetRemoval(t *testing.T) {
	staging := observedEnvironment(7, "staging", setting.DeploymentBranchPolicyAll)
	name := mustEnvironmentName(t, "staging")
	reviewers := setting.Unit{Key: setting.EnvironmentReviewerSet, Environment: name}
	preventSelfReview := setting.Unit{Key: setting.EnvironmentPreventSelfReview, Environment: name}
	cases := []struct {
		name              string
		approved, current github.Environment
		desired           setting.ReviewerSet
		want              preconditionCheck
		wantChanged       setting.Unit
	}{
		{name: "removal anticipating true holds with true", approved: withReviewers(staging, true, 1), current: withReviewers(staging, true, 1), desired: reviewerSet(), want: preconditionHolds},
		{
			name: "removal with only prevent_self_review changed from true is a change of prevent_self_review", approved: withReviewers(staging, true, 1), current: withReviewers(staging, false, 1),
			desired: reviewerSet(), want: preconditionCurrentValueChanged, wantChanged: preventSelfReview,
		},
		{
			name: "removal with only prevent_self_review changed from false is a change of prevent_self_review", approved: withReviewers(staging, false, 1), current: withReviewers(staging, true, 1),
			desired: reviewerSet(), want: preconditionCurrentValueChanged, wantChanged: preventSelfReview,
		},
		{
			name: "removal with both reviewers and prevent_self_review changed is a change of reviewers", approved: withReviewers(staging, true, 1), current: withReviewers(staging, false, 2),
			desired: reviewerSet(), want: preconditionCurrentValueChanged, wantChanged: reviewers,
		},
		{name: "update keeping reviewers does not compare prevent_self_review", approved: withReviewers(staging, true, 1), current: withReviewers(staging, false, 1), desired: reviewerSet(2), want: preconditionHolds},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			operation := reviewersChange(t, "staging", tc.approved, tc.desired).Operations[0]
			observed := github.ObservationResult{RepositoryID: 1, Environments: environments(tc.current)}
			if got, changed := checkPrecondition(1, operation, observed, github.EnvironmentObservation{}); got != tc.want || changed != tc.wantChanged {
				t.Errorf("checkPrecondition() = %v, %v, want %v, %v", got, changed, tc.want, tc.wantChanged)
			}
		})
	}

	// 担当者の基準が予定どおりのまま保護ルールだけがなくなる観測は作れないため、自己承認の禁止の基準を直接比べる
	t.Run("prevent_self_review anticipated false holds without the protection rule", func(t *testing.T) {
		bases := reviewersChange(t, "staging", withReviewers(staging, false, 1), reviewerSet()).Operations[0].PreconditionBases
		basis := bases[len(bases)-1]
		if basis.Unit != preventSelfReview {
			t.Fatalf("last basis = %v, want %v", basis.Unit, preventSelfReview)
		}
		if !matchesPreconditionBasis(basis, github.ObservationResult{RepositoryID: 1, Environments: environments(staging)}, github.EnvironmentObservation{}) {
			t.Error("matchesPreconditionBasis() = false, want true")
		}
	})
}

// TestMatchesPreconditionBasis_Ruleset は、承認時に予定した Ruleset の存在、値の全体、ID の基準が、更新直前の観測と同じかの規則を確かめる。
// 名前で引く規則は github の Rulesets.Lookup、値の集合としての比べ方は setting の Ruleset のテストが確かめる。
//
//trace:checks INV-001 INV-004 FR-010
func TestMatchesPreconditionBasis_Ruleset(t *testing.T) {
	name := mustRulesetName(t, "protect")
	unit := setting.Unit{Key: setting.RulesetResource, Ruleset: name}
	// pullRequestRule は、許すマージ方式が methods の pull_request のルールを作る
	pullRequestRule := func(methods ...setting.AllowedMergeMethod) setting.RulesetRule {
		return setting.PullRequestRule(setting.PullRequestParameters{RequiredApprovingReviewCount: 1, AllowedMergeMethods: methods, RequireExtraApprovalForUnattributedChanges: true})
	}
	// approvedValue は、順序の違いを作れるよう、ルールと Bypass を二つずつ持つ
	approvedValue := setting.Ruleset{
		Target:      setting.RulesetTargetBranch,
		Enforcement: setting.RulesetEnforcementDisabled,
		Conditions:  setting.RulesetConditions{RefName: &setting.RefNameCondition{Include: []string{"~DEFAULT_BRANCH"}, Exclude: []string{}}},
		BypassActors: []setting.BypassActor{
			{ActorType: setting.BypassActorRepositoryRole, ActorID: 5, BypassMode: setting.BypassModeAlways},
			{ActorType: setting.BypassActorDeployKey, BypassMode: setting.BypassModeAlways},
		},
		Rules: []setting.RulesetRule{setting.DeletionRule(), pullRequestRule(setting.AllowedMergeMethodMerge, setting.AllowedMergeMethodSquash, setting.AllowedMergeMethodRebase)},
	}
	reordered := approvedValue
	reordered.BypassActors = []setting.BypassActor{approvedValue.BypassActors[1], approvedValue.BypassActors[0]}
	reordered.Rules = []setting.RulesetRule{approvedValue.Rules[1], approvedValue.Rules[0]}
	// added は、宣言が変えない部分に、外部主体が Bypass を足した値
	added := approvedValue
	added.BypassActors = []setting.BypassActor{
		approvedValue.BypassActors[0], approvedValue.BypassActors[1], {ActorType: setting.BypassActorUser, ActorID: 1, BypassMode: setting.BypassModeAlways},
	}
	approvedBasis := changeplan.PreconditionBasis{
		Unit: unit, AnticipatedValue: changeplan.AnticipateValue(setting.RulesetValue(approvedValue)), AnticipatedRulesetIdentity: changeplan.ObservedRulesetIdentity(42),
	}
	absentBasis := changeplan.PreconditionBasis{Unit: unit, AnticipatedValue: changeplan.AnticipateValue(setting.Absent())}
	cases := []struct {
		name    string
		basis   changeplan.PreconditionBasis
		current []github.Ruleset
		want    bool
	}{
		{name: "same ID and same value is the same", basis: approvedBasis, current: []github.Ruleset{github.InterpretedRuleset(42, "protect", approvedValue)}, want: true},
		{name: "value differing only in order is the same", basis: approvedBasis, current: []github.Ruleset{github.InterpretedRuleset(42, "protect", reordered)}, want: true},
		{name: "name differing only in case with the same ID and value is the same", basis: approvedBasis, current: []github.Ruleset{github.InterpretedRuleset(42, "PROTECT", approvedValue)}, want: true},
		{name: "same value with another ID differs", basis: approvedBasis, current: []github.Ruleset{github.InterpretedRuleset(43, "protect", approvedValue)}, want: false},
		{name: "same ID with an added bypass actor the declaration does not change differs", basis: approvedBasis, current: []github.Ruleset{github.InterpretedRuleset(42, "protect", added)}, want: false},
		{name: "deleted ruleset differs", basis: approvedBasis, want: false},
		{name: "same ID under another name differs", basis: approvedBasis, current: []github.Ruleset{github.InterpretedRuleset(42, "renamed", approvedValue)}, want: false},
		{
			name: "same ID with uninterpreted parts differs", basis: approvedBasis,
			current: []github.Ruleset{github.UninterpretedRuleset(42, "protect", []github.UninterpretedPart{{Kind: github.UninterpretedRuleType, Name: "creation"}})},
			want:    false,
		},
		{name: "only an undetermined name differs", basis: approvedBasis, current: []github.Ruleset{github.InterpretedRuleset(42, "protëct", approvedValue)}, want: false},
		{name: "absent at approval and still absent is the same", basis: absentBasis, current: []github.Ruleset{github.InterpretedRuleset(7, "other", approvedValue)}, want: true},
		{name: "absent at approval and existing now differs", basis: absentBasis, current: []github.Ruleset{github.InterpretedRuleset(42, "protect", approvedValue)}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			observed := github.ObservationResult{RepositoryID: 1, Rulesets: &github.Rulesets{Existing: github.Observed(tc.current)}}
			if got := matchesPreconditionBasis(tc.basis, observed, github.EnvironmentObservation{}); got != tc.want {
				t.Errorf("matchesPreconditionBasis() = %v, want %v", got, tc.want)
			}
		})
	}

	// changeplan が作らない基準。承認の前に適用不能として止める観測不能と、ID の基準のない値を伴う存在は、
	// 更新直前の観測を基準を受け付けたときに成立する側の形にしても、不変条件の破れとして止める
	uninterpreted := github.UninterpretedRuleset(42, "protect", []github.UninterpretedPart{{Kind: github.UninterpretedRuleType, Name: "creation"}})
	panicCases := []struct {
		name    string
		basis   changeplan.PreconditionBasis
		current []github.Ruleset
	}{
		{
			name:    "existing at approval without an identity panics even if the same ruleset exists",
			basis:   changeplan.PreconditionBasis{Unit: unit, AnticipatedValue: changeplan.AnticipateValue(setting.RulesetValue(approvedValue))},
			current: []github.Ruleset{github.InterpretedRuleset(42, "protect", approvedValue)},
		},
		{
			name: "uninterpreted at approval panics even if still uninterpreted with the same ID",
			basis: changeplan.PreconditionBasis{
				Unit: unit, AnticipatedValue: changeplan.AnticipateUnchanged(github.Unavailable[setting.Value](github.UnavailableUninterpretedParts)), AnticipatedRulesetIdentity: changeplan.ObservedRulesetIdentity(42),
			},
			current: []github.Ruleset{uninterpreted},
		},
	}
	for _, tc := range panicCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("matchesPreconditionBasis() did not panic")
				}
			}()
			observed := github.ObservationResult{RepositoryID: 1, Rulesets: &github.Rulesets{Existing: github.Observed(tc.current)}}
			matchesPreconditionBasis(tc.basis, observed, github.EnvironmentObservation{})
		})
	}
}

// TestCheckPrecondition_Environments は、Environment の作成、方式の変更、担当者の集合の変更の更新直前の判定表を確かめる。
// 存在の比べ方は TestMatchesPreconditionBasis_EnvironmentExistence、方式の値の比べ方は TestMatchesAnticipatedValue が確かめる。
//
//trace:checks INV-001 INV-004 FR-010
func TestCheckPrecondition_Environments(t *testing.T) {
	production := mustEnvironmentName(t, "production")
	staging := mustEnvironmentName(t, "staging")
	existence := func(name setting.EnvironmentName) setting.Unit {
		return setting.Unit{Key: setting.EnvironmentResource, Environment: name}
	}
	stagingPolicy := setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: staging}
	approvedStaging := observedEnvironment(7, "Staging", setting.DeploymentBranchPolicyAll)
	selected := func(id github.EnvironmentID, patterns ...string) github.Environment {
		e := observedEnvironment(id, "Staging", setting.DeploymentBranchPolicySelected)
		var read github.DeploymentPatterns
		for _, p := range patterns {
			read.Existing = append(read.Existing, setting.DeploymentPattern{Type: setting.DeploymentPatternBranch, Name: p})
		}
		observedPatterns := github.Observed(read)
		e.DeploymentPatterns = &observedPatterns
		return e
	}
	protectedStaging := approvedStaging
	protectedStaging.Protection = github.EnvironmentProtection{WaitTimer: 30, ReviewerSet: github.Observed(setting.ReviewerSetOfRule(nil)), CanAdminsBypass: true}
	main, feature := branchPattern("main"), branchPattern("feature/*")
	// 外す DELETE feature/*（基準 [main, feature/*]）、足す POST release/*（基準 [main]）の順
	replacement := deploymentPatternsChange(t, "staging", selectedEnvironment(7, "Staging", []setting.DeploymentPattern{main, feature}, 10, 11), selectedPolicy(main, branchPattern("release/*")))
	creation := creationWithPatternsChange(t, "production", selectedPolicy(main))

	cases := []struct {
		name   string
		change changeplan.LogicalChange
		// index は、確かめる更新操作の change.Operations の添字。
		index       int
		observed    github.ObservationResult
		want        preconditionCheck
		wantChanged setting.Unit
	}{
		{
			name:     "creation holds when no environment of the same name exists",
			change:   creationChange(t, "production"),
			observed: github.ObservationResult{RepositoryID: 1, Environments: environments(approvedStaging)},
			want:     preconditionHolds,
		},
		{
			name:        "creation with the environment created since approval is a change of the environment",
			change:      creationWithModeChange(t, "production", setting.DeploymentBranchPolicyProtectedBranches),
			observed:    github.ObservationResult{RepositoryID: 1, Environments: environments(observedEnvironment(8, "production", setting.DeploymentBranchPolicyProtectedBranches))},
			want:        preconditionCurrentValueChanged,
			wantChanged: existence(production),
		},
		{
			name:        "creation with an environment of a name differing only in case is a change of the environment",
			change:      creationChange(t, "production"),
			observed:    github.ObservationResult{RepositoryID: 1, Environments: environments(observedEnvironment(8, "Production", setting.DeploymentBranchPolicyAll))},
			want:        preconditionCurrentValueChanged,
			wantChanged: existence(production),
		},
		{
			name:        "creation with an undetermined name is a change of the environment",
			change:      creationChange(t, "production"),
			observed:    github.ObservationResult{RepositoryID: 1, Environments: environments(observedEnvironment(8, "prodüction", setting.DeploymentBranchPolicyAll))},
			want:        preconditionCurrentValueChanged,
			wantChanged: existence(production),
		},
		{
			name:     "creation in another repository is an identity mismatch",
			change:   creationChange(t, "production"),
			observed: github.ObservationResult{RepositoryID: 2, Environments: environments()},
			want:     preconditionIdentityMismatch,
		},
		{
			name:     "mode change holds with the same ID and mode",
			change:   modeChange(t, "staging", approvedStaging, setting.DeploymentBranchPolicyProtectedBranches),
			observed: github.ObservationResult{RepositoryID: 1, Environments: environments(approvedStaging)},
			want:     preconditionHolds,
		},
		{
			name:   "mode change holds when only the protection changed and an undetermined name appeared",
			change: modeChange(t, "staging", approvedStaging, setting.DeploymentBranchPolicyProtectedBranches),
			observed: github.ObservationResult{RepositoryID: 1, Environments: environments(
				observedEnvironment(8, "prodüction", setting.DeploymentBranchPolicyAll), protectedStaging,
			)},
			want: preconditionHolds,
		},
		{
			name:     "mode change holds when the patterns differ only in order",
			change:   modeChange(t, "staging", selected(7, "main", "release"), setting.DeploymentBranchPolicyProtectedBranches),
			observed: github.ObservationResult{RepositoryID: 1, Environments: environments(selected(7, "release", "main"))},
			want:     preconditionHolds,
		},
		{
			name:        "mode change with another set of patterns is a change of the mode",
			change:      modeChange(t, "staging", selected(7, "main"), setting.DeploymentBranchPolicyProtectedBranches),
			observed:    github.ObservationResult{RepositoryID: 1, Environments: environments(selected(7, "main", "release"))},
			want:        preconditionCurrentValueChanged,
			wantChanged: stagingPolicy,
		},
		{
			name:        "mode change with another mode is a change of the mode",
			change:      modeChange(t, "staging", approvedStaging, setting.DeploymentBranchPolicyProtectedBranches),
			observed:    github.ObservationResult{RepositoryID: 1, Environments: environments(observedEnvironment(7, "Staging", setting.DeploymentBranchPolicyProtectedBranches))},
			want:        preconditionCurrentValueChanged,
			wantChanged: stagingPolicy,
		},
		{
			name:        "mode change of an environment recreated with another ID is a change of the environment",
			change:      modeChange(t, "staging", approvedStaging, setting.DeploymentBranchPolicyProtectedBranches),
			observed:    github.ObservationResult{RepositoryID: 1, Environments: environments(observedEnvironment(8, "Staging", setting.DeploymentBranchPolicyAll))},
			want:        preconditionCurrentValueChanged,
			wantChanged: existence(staging),
		},
		{
			name:        "mode change of an environment recreated with another mode is reported as a change of the environment first",
			change:      modeChange(t, "staging", approvedStaging, setting.DeploymentBranchPolicyProtectedBranches),
			observed:    github.ObservationResult{RepositoryID: 1, Environments: environments(observedEnvironment(8, "Staging", setting.DeploymentBranchPolicyProtectedBranches))},
			want:        preconditionCurrentValueChanged,
			wantChanged: existence(staging),
		},
		{
			name:        "mode change of an environment that no longer exists is a change of the environment",
			change:      modeChange(t, "staging", approvedStaging, setting.DeploymentBranchPolicyProtectedBranches),
			observed:    github.ObservationResult{RepositoryID: 1, Environments: environments()},
			want:        preconditionCurrentValueChanged,
			wantChanged: existence(staging),
		},
		{
			name:     "pattern removal holds when only the IDs of the patterns differ",
			change:   replacement,
			observed: github.ObservationResult{RepositoryID: 1, Environments: environments(selectedEnvironment(7, "Staging", []setting.DeploymentPattern{main, feature}, 20, 21))},
			want:     preconditionHolds,
		},
		{
			name:        "pattern addition with a pattern outside the expected set is a change of the policy",
			change:      replacement,
			index:       1,
			observed:    github.ObservationResult{RepositoryID: 1, Environments: environments(selectedEnvironment(7, "Staging", []setting.DeploymentPattern{main, branchPattern("other/*")}, 10, 12))},
			want:        preconditionCurrentValueChanged,
			wantChanged: stagingPolicy,
		},
		{
			name:        "pattern addition to an environment recreated with another ID is a change of the environment even if the set is as expected",
			change:      replacement,
			index:       1,
			observed:    github.ObservationResult{RepositoryID: 1, Environments: environments(selectedEnvironment(8, "Staging", []setting.DeploymentPattern{main}, 10))},
			want:        preconditionCurrentValueChanged,
			wantChanged: existence(staging),
		},
		{
			name:     "reviewers update holds with the same ID and reviewers",
			change:   reviewersChange(t, "staging", withReviewers(approvedStaging, false, 1), reviewerSet(2)),
			observed: github.ObservationResult{RepositoryID: 1, Environments: environments(withReviewers(approvedStaging, false, 1))},
			want:     preconditionHolds,
		},
		{
			name:        "reviewers update with reviewers changed since approval is a change of the reviewers",
			change:      reviewersChange(t, "staging", withReviewers(approvedStaging, false, 1), reviewerSet(2)),
			observed:    github.ObservationResult{RepositoryID: 1, Environments: environments(withReviewers(approvedStaging, false, 1, 3))},
			want:        preconditionCurrentValueChanged,
			wantChanged: setting.Unit{Key: setting.EnvironmentReviewerSet, Environment: staging},
		},
		{
			name:     "pattern addition after a creation reads only that the preceding update created the environment",
			change:   creation,
			index:    1,
			observed: github.ObservationResult{RepositoryID: 1, Environments: environments(selectedEnvironment(8, "production", nil))},
			want:     preconditionHolds,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, changed := checkPrecondition(1, tc.change.Operations[tc.index], tc.observed, github.EnvironmentObservation{})
			if got != tc.want || changed != tc.wantChanged {
				t.Errorf("checkPrecondition() = %v, %+v, want %v, %+v", got, changed, tc.want, tc.wantChanged)
			}
		})
	}
}

// TestCheckPrecondition_RepositoryVariables は、変数の作成、値の変更、削除の変更前提を、一覧から引いた存在と値で確かめることを確かめる。
//
//trace:checks INV-001 INV-004 FR-010
func TestCheckPrecondition_RepositoryVariables(t *testing.T) {
	goVersion := setting.Unit{Key: setting.RepositoryVariable, Variable: mustVariableName(t, "GO_VERSION")}
	cases := []struct {
		name        string
		change      changeplan.LogicalChange
		variables   *github.Variables
		want        preconditionCheck
		wantChanged setting.Unit
	}{
		{
			name:      "creation holds when only another variable exists",
			change:    variableCreationChange(t, "GO_VERSION", "1.27"),
			variables: variables(github.Variable{Name: "OTHER", Value: "x"}),
			want:      preconditionHolds,
		},
		{
			name:        "creation with the variable created since approval with the declared value is a change of the variable",
			change:      variableCreationChange(t, "GO_VERSION", "1.27"),
			variables:   variables(github.Variable{Name: "GO_VERSION", Value: "1.27"}),
			want:        preconditionCurrentValueChanged,
			wantChanged: goVersion,
		},
		{
			name:      "value change holds when the value is the planned value",
			change:    variableValueChange(t, "GO_VERSION", "1.26", "1.27"),
			variables: variables(github.Variable{Name: "GO_VERSION", Value: "1.26"}),
			want:      preconditionHolds,
		},
		{
			name:        "value change with the declared value since approval is a change of the variable",
			change:      variableValueChange(t, "GO_VERSION", "1.26", "1.27"),
			variables:   variables(github.Variable{Name: "GO_VERSION", Value: "1.27"}),
			want:        preconditionCurrentValueChanged,
			wantChanged: goVersion,
		},
		{
			name:        "value change with the variable deleted since approval is a change of the variable",
			change:      variableValueChange(t, "GO_VERSION", "1.26", "1.27"),
			variables:   variables(),
			want:        preconditionCurrentValueChanged,
			wantChanged: goVersion,
		},
		{
			// 承認の前に示していない値を失わせない
			name:        "deletion with another value since approval is a change of the variable",
			change:      variableDeletionChange(t, "GO_VERSION", "1.26"),
			variables:   variables(github.Variable{Name: "GO_VERSION", Value: "1.27"}),
			want:        preconditionCurrentValueChanged,
			wantChanged: goVersion,
		},
		{
			name:        "deletion with the variable deleted since approval is a change of the variable",
			change:      variableDeletionChange(t, "GO_VERSION", "1.26"),
			variables:   variables(),
			want:        preconditionCurrentValueChanged,
			wantChanged: goVersion,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			observed := github.ObservationResult{RepositoryID: 1, RepositoryVariables: tc.variables}
			got, changed := checkPrecondition(1, tc.change.Operations[0], observed, github.EnvironmentObservation{})
			if got != tc.want || changed != tc.wantChanged {
				t.Errorf("checkPrecondition() = %v, %+v, want %v, %+v", got, changed, tc.want, tc.wantChanged)
			}
		})
	}
}

// TestCheckPrecondition_EnvironmentVariables は、Environment Variable の作成、値の変更、削除の変更前提を、
// 計画時に存在した Environment では Environment の存在と ID、変数の順に、同じ変更計画が作成した Environment では変数が存在しないことだけで確かめることを確かめる。
//
//trace:checks INV-001 INV-004 FR-010
func TestCheckPrecondition_EnvironmentVariables(t *testing.T) {
	production := mustEnvironmentName(t, "production")
	existence := setting.Unit{Key: setting.EnvironmentResource, Environment: production}
	region := setting.Unit{Key: setting.EnvironmentVariable, Environment: production, Variable: mustVariableName(t, "REGION")}
	planned := github.Variable{Name: "REGION", Value: "ap-northeast-1"}
	approved := withVariables(observedEnvironment(7, "Production", setting.DeploymentBranchPolicyAll), planned)
	valueChange := environmentVariableValueChange(t, "production", approved, "REGION", "ap-northeast-1", "ap-northeast-3")
	creation := createdEnvironmentVariableChange(t, "production", "REGION", "ap-northeast-1")
	deletion := environmentVariableDeletionChange(t, "production", approved, "REGION", "ap-northeast-1")
	cases := []struct {
		name        string
		change      changeplan.LogicalChange
		existing    []github.Environment
		want        preconditionCheck
		wantChanged setting.Unit
	}{
		{
			name:     "value change holds when the environment and the value are as planned",
			change:   valueChange,
			existing: []github.Environment{approved},
			want:     preconditionHolds,
		},
		{
			name:        "value change of an environment that no longer exists is a change of the environment",
			change:      valueChange,
			existing:    nil,
			want:        preconditionCurrentValueChanged,
			wantChanged: existence,
		},
		{
			// 作り直された Environment の変数は、計画時と同じ値でも承認した変更の相手ではない
			name:        "value change of an environment recreated with another ID is a change of the environment even if the value is as planned",
			change:      valueChange,
			existing:    []github.Environment{withVariables(observedEnvironment(8, "Production", setting.DeploymentBranchPolicyAll), planned)},
			want:        preconditionCurrentValueChanged,
			wantChanged: existence,
		},
		{
			name:        "value change with another value since approval is a change of the variable",
			change:      valueChange,
			existing:    []github.Environment{withVariables(observedEnvironment(7, "Production", setting.DeploymentBranchPolicyAll), github.Variable{Name: "REGION", Value: "us-east-1"})},
			want:        preconditionCurrentValueChanged,
			wantChanged: region,
		},
		{
			// 作り直された Environment の同じ名前と値の変数は、承認した削除の相手ではない
			name:        "deletion in an environment recreated with another ID is a change of the environment even if the value is as planned",
			change:      deletion,
			existing:    []github.Environment{withVariables(observedEnvironment(8, "Production", setting.DeploymentBranchPolicyAll), planned)},
			want:        preconditionCurrentValueChanged,
			wantChanged: existence,
		},
		{
			name:     "creation in a created environment holds when the environment exists without the variable",
			change:   creation,
			existing: []github.Environment{withVariables(observedEnvironment(8, "production", setting.DeploymentBranchPolicyAll), github.Variable{Name: "OTHER", Value: "x"})},
			want:     preconditionHolds,
		},
		{
			// 変数は観測不能になり、存在しないことと同じには扱わない
			name:        "creation in a created environment that does not exist is a change of the variable",
			change:      creation,
			existing:    nil,
			want:        preconditionCurrentValueChanged,
			wantChanged: region,
		},
		{
			name:        "creation in a created environment with the variable created with the declared value is a change of the variable",
			change:      creation,
			existing:    []github.Environment{withVariables(observedEnvironment(8, "production", setting.DeploymentBranchPolicyAll), planned)},
			want:        preconditionCurrentValueChanged,
			wantChanged: region,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			observed := github.ObservationResult{RepositoryID: 1, Environments: environments(tc.existing...)}
			got, changed := checkPrecondition(1, tc.change.Operations[0], observed, github.EnvironmentObservation{})
			if got != tc.want || changed != tc.wantChanged {
				t.Errorf("checkPrecondition() = %v, %+v, want %v, %+v", got, changed, tc.want, tc.wantChanged)
			}
		})
	}
}

// TestEnvironmentCreatedIn は、同じ論理的な設定変更が作成した Environment の ID の基準を、作成に続く最初の更新操作の直前の観測から引く規則を確かめる。
//
//trace:checks INV-004 FR-010
func TestEnvironmentCreatedIn(t *testing.T) {
	creation := creationWithPatternsChange(t, "production", selectedPolicy(branchPattern("main"), tagPattern("v*")))
	observedWith := func(id github.EnvironmentID) github.ObservationResult {
		return github.ObservationResult{RepositoryID: 1, Environments: environments(selectedEnvironment(id, "Production", nil))}
	}
	t.Run("creation with one read before the update has no base yet", func(t *testing.T) {
		// 作成の直前の観測を引けば、そこにある Environment が基準になってしまうことを、存在する Environment を置いて確かめる
		before := []github.ObservationResult{observedWith(7)}
		if got := environmentCreatedIn(creation, before); !reflect.DeepEqual(got, github.EnvironmentObservation{}) {
			t.Errorf("environmentCreatedIn() = %+v, want the zero value", got)
		}
	})
	t.Run("creation with later reads is the environment read before the first update after the creation", func(t *testing.T) {
		before := []github.ObservationResult{{RepositoryID: 1, Environments: environments()}, observedWith(8), observedWith(9)}
		got, ok := environmentCreatedIn(creation, before).Existing()
		if !ok || got.ID != 8 {
			t.Errorf("environmentCreatedIn() = %+v, %v, want the environment with ID 8", got, ok)
		}
	})
	t.Run("change without a creation has no base", func(t *testing.T) {
		change := deploymentPatternsChange(t, "production", observedEnvironment(8, "production", setting.DeploymentBranchPolicyAll), selectedPolicy(branchPattern("main")))
		before := []github.ObservationResult{observedWith(8), observedWith(8)}
		if got := environmentCreatedIn(change, before); !reflect.DeepEqual(got, github.EnvironmentObservation{}) {
			t.Errorf("environmentCreatedIn() = %+v, want the zero value", got)
		}
	})
	t.Run("creation with the environment not existing before the first update after it panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("environmentCreatedIn() did not panic")
			}
		}()
		environmentCreatedIn(creation, []github.ObservationResult{{RepositoryID: 1, Environments: environments()}, {RepositoryID: 1, Environments: environments()}})
	})
	t.Run("change without a creation expecting a created environment panics", func(t *testing.T) {
		change := deploymentPatternsChange(t, "production", observedEnvironment(8, "production", setting.DeploymentBranchPolicyAll), selectedPolicy(branchPattern("main")))
		change.Operations[1].PreconditionBases[0] = changeplan.PreconditionBasis{
			Unit: change.Operations[1].PreconditionBases[0].Unit, AnticipatedValue: changeplan.AnticipateValue(setting.Present()), AnticipatedEnvironmentIdentity: changeplan.CreatedByPrecedingOperation(),
		}
		defer func() {
			if recover() == nil {
				t.Error("environmentCreatedIn() did not panic")
			}
		}()
		environmentCreatedIn(change, []github.ObservationResult{observedWith(8)})
	})
}

// lossOf は、削除で失われるものまで読んだ観測 e から、changeplan と同じく削除で失われるものを作る。
func lossOf(t *testing.T, e github.Environment) changeplan.EnvironmentLoss {
	t.Helper()
	deletedValue, ok := e.DeletedValue().Value()
	if !ok {
		t.Fatalf("deleted value of %q is unavailable, want an observed value", e.Name)
	}
	return changeplan.EnvironmentLoss{DeletedValue: deletedValue, Variables: e.Variables.Existing, Secrets: e.Secrets.Existing}
}
