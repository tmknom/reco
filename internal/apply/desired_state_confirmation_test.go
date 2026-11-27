package apply

import (
	"slices"
	"testing"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

//trace:checks FR-012
func TestConfirmDesiredState(t *testing.T) {
	change := allowAutoMergeChange()
	cases := []struct {
		name     string
		observed github.ObservationResult
		want     desiredStateConfirmation
	}{
		{
			name:     "desired value matches",
			observed: github.ObservationResult{RepositoryID: 1, General: github.General{AllowAutoMerge: true}},
			want:     desiredStateMatches,
		},
		{
			name:     "other value is a mismatch",
			observed: github.ObservationResult{RepositoryID: 1, General: github.General{AllowAutoMerge: false}},
			want:     desiredStateMismatch,
		},
		{
			name:     "different ID is not confirmed even if the value matches",
			observed: github.ObservationResult{RepositoryID: 2, General: github.General{AllowAutoMerge: true}},
			want:     desiredStateIdentityMismatch,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got, unavailable := confirmDesiredState(1, change, tc.observed); got != tc.want || unavailable != (unavailableManagedSetting{}) {
				t.Errorf("confirmDesiredState() = %v, %v, want %v without an unavailable setting", got, unavailable, tc.want)
			}
		})
	}

	selectedCases := []struct {
		name       string
		change     changeplan.LogicalChange
		observed   *github.ActionsPermissions
		want       desiredStateConfirmation
		wantReason github.UnavailableReason
	}{
		{
			name:     "criteria differing from the desired value only in the order of patterns match",
			change:   selectedActionsChange(),
			observed: actionsSelected(setting.SelectedActionsCriteria{VerifiedAllowed: true, PatternsAllowed: []string{"b/*", "a/*"}}),
			want:     desiredStateMatches,
		},
		{
			name:       "unavailable criteria are unavailable rather than a mismatch",
			change:     selectedActionsChange(),
			observed:   actionsDisabled(),
			want:       desiredStateUnavailable,
			wantReason: github.UnavailableActionsDisabled,
		},
		{
			name:     "policy and criteria both desired, with criteria differing only in the order of patterns, match",
			change:   toSelectedChange(),
			observed: actionsSelected(setting.SelectedActionsCriteria{VerifiedAllowed: true, PatternsAllowed: []string{"b/*", "a/*"}}),
			want:     desiredStateMatches,
		},
		{
			name:     "selected policy with the criteria GitHub set back is a mismatch of the second desired value",
			change:   toSelectedChange(),
			observed: actionsSelected(setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, PatternsAllowed: []string{}}),
			want:     desiredStateMismatch,
		},
		{
			// ドメインモデルの期待状態の確認結果。観測できた値に期待値と異なるものがあれば、観測不能が残っても不一致になる
			name:     "policy other than selected is a mismatch even though the criteria are unavailable",
			change:   toSelectedChange(),
			observed: actionsEnabled(setting.AllowedActionsAll, false),
			want:     desiredStateMismatch,
		},
		{
			// 計画が作らない並び。観測不能の設定が先に並んでも、観測できた値の不一致が優先されることを確かめる
			name: "mismatch after an unavailable desired value in the order of the desired state is still a mismatch",
			change: changeplan.LogicalChange{DesiredState: []configuration.ManagedSetting{
				toSelectedChange().DesiredState[1], toSelectedChange().DesiredState[0],
			}},
			observed: actionsEnabled(setting.AllowedActionsAll, false),
			want:     desiredStateMismatch,
		},
		{
			name:       "actions disabled is unavailable for both desired values with the reason",
			change:     toSelectedChange(),
			observed:   actionsDisabled(),
			want:       desiredStateUnavailable,
			wantReason: github.UnavailableActionsDisabled,
		},
	}
	for _, tc := range selectedCases {
		t.Run(tc.name, func(t *testing.T) {
			observed := github.ObservationResult{RepositoryID: 1, ActionsPermissions: tc.observed}
			if got, unavailable := confirmDesiredState(1, tc.change, observed); got != tc.want || unavailable.reason != tc.wantReason {
				t.Errorf("confirmDesiredState() = %v, %v, want %v, %v", got, unavailable.reason, tc.want, tc.wantReason)
			}
		})
	}

	// 確認結果の表示は、観測不能だった管理設定の種類から文言を選ぶため、理由と同じ管理設定を返す
	t.Run("unavailable desired values return the first unavailable setting in the order of the desired state", func(t *testing.T) {
		change := toSelectedChange()
		observed := github.ObservationResult{RepositoryID: 1, ActionsPermissions: actionsDisabled()}
		want := unavailableManagedSetting{unit: change.DesiredState[0].Unit, reason: github.UnavailableActionsDisabled}
		if _, got := confirmDesiredState(1, change, observed); got != want {
			t.Errorf("confirmDesiredState() unavailable setting = %v, want %v", got, want)
		}
	})

	t.Run("logical change without a desired state panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("confirmDesiredState() did not panic")
			}
		}()
		change := allowAutoMergeChange()
		change.DesiredState = nil
		confirmDesiredState(1, change, github.ObservationResult{RepositoryID: 1})
	})
}

// TestConfirmDesiredState_Environments は、Environment の作成と方式の変更の更新後の判定表を確かめる。
//
//trace:checks INV-004 FR-012
func TestConfirmDesiredState_Environments(t *testing.T) {
	approvedStaging := observedEnvironment(7, "Staging", setting.DeploymentBranchPolicyAll)
	undetermined := observedEnvironment(9, "prodüction", setting.DeploymentBranchPolicyAll)
	cases := []struct {
		name         string
		change       changeplan.LogicalChange
		environments *github.Environments
		want         desiredStateConfirmation
		wantReason   github.UnavailableReason
	}{
		{
			name:         "creation without a mode matches when the environment exists under a spelling differing only in case",
			change:       creationChange(t, "production"),
			environments: environments(observedEnvironment(8, "Production", setting.DeploymentBranchPolicyAll)),
			want:         desiredStateMatches,
		},
		{
			name:         "creation is a mismatch when the environment is absent",
			change:       creationChange(t, "production"),
			environments: environments(approvedStaging),
			want:         desiredStateMismatch,
		},
		{
			name:         "creation is unavailable when the existence cannot be determined",
			change:       creationChange(t, "production"),
			environments: environments(undetermined),
			want:         desiredStateUnavailable,
			wantReason:   github.UnavailableNameUndetermined,
		},
		{
			name:         "creation with a mode matches when the environment has the mode",
			change:       creationWithModeChange(t, "production", setting.DeploymentBranchPolicyProtectedBranches),
			environments: environments(observedEnvironment(8, "production", setting.DeploymentBranchPolicyProtectedBranches)),
			want:         desiredStateMatches,
		},
		{
			name:         "creation with a mode is a mismatch when the environment has another mode",
			change:       creationWithModeChange(t, "production", setting.DeploymentBranchPolicyProtectedBranches),
			environments: environments(observedEnvironment(8, "production", setting.DeploymentBranchPolicyAll)),
			want:         desiredStateMismatch,
		},
		{
			name:         "mode change matches even if the environment was recreated with another ID",
			change:       modeChange(t, "staging", approvedStaging, setting.DeploymentBranchPolicyProtectedBranches),
			environments: environments(observedEnvironment(8, "staging", setting.DeploymentBranchPolicyProtectedBranches)),
			want:         desiredStateMatches,
		},
		{
			name:         "mode change is a mismatch when the mode differs",
			change:       modeChange(t, "staging", approvedStaging, setting.DeploymentBranchPolicyProtectedBranches),
			environments: environments(approvedStaging),
			want:         desiredStateMismatch,
		},
		{
			name:         "mode change is unavailable when the environment is absent",
			change:       modeChange(t, "staging", approvedStaging, setting.DeploymentBranchPolicyProtectedBranches),
			environments: environments(),
			want:         desiredStateUnavailable,
			wantReason:   github.UnavailableEnvironmentAbsent,
		},
		{
			name:         "mode change is unavailable when the existence cannot be determined",
			change:       modeChange(t, "staging", approvedStaging, setting.DeploymentBranchPolicyProtectedBranches),
			environments: environments(observedEnvironment(9, "stagïng", setting.DeploymentBranchPolicyProtectedBranches)),
			want:         desiredStateUnavailable,
			wantReason:   github.UnavailableNameUndetermined,
		},
		{
			// 方式を送らない変更でも、デプロイ元制限の値の全体を期待状態として確かめる
			name: "pattern updates alone are a mismatch when one declared pattern is missing",
			change: deploymentPatternsChange(t, "staging", selectedEnvironment(7, "Staging", []setting.DeploymentPattern{branchPattern("feature/*")}, 11),
				selectedPolicy(branchPattern("main"), branchPattern("release/*"))),
			environments: environments(selectedEnvironment(7, "Staging", []setting.DeploymentPattern{branchPattern("main")}, 12)),
			want:         desiredStateMismatch,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			observed := github.ObservationResult{RepositoryID: 1, Environments: tc.environments}
			if got, unavailable := confirmDesiredState(1, tc.change, observed); got != tc.want || unavailable.reason != tc.wantReason {
				t.Errorf("confirmDesiredState() = %v, %v, want %v, %v", got, unavailable.reason, tc.want, tc.wantReason)
			}
		})
	}
}

// TestConfirmDesiredState_RepositoryVariables は、変数の作成、値の変更、削除の期待状態を、一覧から引いた存在と値で確かめることを確かめる。
//
//trace:checks FR-012
func TestConfirmDesiredState_RepositoryVariables(t *testing.T) {
	cases := []struct {
		name      string
		change    changeplan.LogicalChange
		variables *github.Variables
		want      desiredStateConfirmation
	}{
		{
			name:      "creation matches when the variable exists with the declared value",
			change:    variableCreationChange(t, "GO_VERSION", "1.27"),
			variables: variables(github.Variable{Name: "GO_VERSION", Value: "1.27"}),
			want:      desiredStateMatches,
		},
		{
			name:      "creation is a mismatch when the variable exists with another value",
			change:    variableCreationChange(t, "GO_VERSION", "1.27"),
			variables: variables(github.Variable{Name: "GO_VERSION", Value: "1.26"}),
			want:      desiredStateMismatch,
		},
		{
			name:      "value change is a mismatch when the variable is absent",
			change:    variableValueChange(t, "GO_VERSION", "1.26", "1.27"),
			variables: variables(),
			want:      desiredStateMismatch,
		},
		{
			name:      "deletion matches when the list lacks the variable",
			change:    variableDeletionChange(t, "GO_VERSION", "1.26"),
			variables: variables(github.Variable{Name: "OTHER", Value: "x"}),
			want:      desiredStateMatches,
		},
		{
			// 削除の要求への成功応答だけでは、存在しないことを確かめたことにならない
			name:      "deletion is a mismatch when the list still has the variable",
			change:    variableDeletionChange(t, "GO_VERSION", "1.26"),
			variables: variables(github.Variable{Name: "GO_VERSION", Value: "1.26"}),
			want:      desiredStateMismatch,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			observed := github.ObservationResult{RepositoryID: 1, RepositoryVariables: tc.variables}
			if got, unavailable := confirmDesiredState(1, tc.change, observed); got != tc.want || unavailable != (unavailableManagedSetting{}) {
				t.Errorf("confirmDesiredState() = %v, %v, want %v without an unavailable setting", got, unavailable, tc.want)
			}
		})
	}
}

// TestConfirmDesiredState_RulesetOmittedParameter は、Ruleset の期待状態を、宣言が省略したパラメータを比べずに確かめることを確かめる。
// 省略したパラメータは管理範囲外で、送らなかったときに GitHub が決める値で返っても、完了を妨げない。
//
//trace:checks FR-001 FR-012
func TestConfirmDesiredState_RulesetOmittedParameter(t *testing.T) {
	name := mustRulesetName(t, "protect")
	unit := setting.Unit{Key: setting.RulesetResource, Ruleset: name}
	value := func(strict bool, doNotEnforceOnCreate *bool) setting.Ruleset {
		return setting.Ruleset{
			Target: setting.RulesetTargetBranch, Enforcement: setting.RulesetEnforcementActive,
			Conditions:   setting.RulesetConditions{RefName: &setting.RefNameCondition{Include: []string{"~DEFAULT_BRANCH"}, Exclude: []string{}}},
			BypassActors: []setting.BypassActor{},
			Rules: []setting.RulesetRule{setting.RequiredStatusChecksRule(setting.RequiredStatusChecksParameters{
				RequiredStatusChecks: []setting.StatusCheck{{Context: "test"}}, StrictRequiredStatusChecksPolicy: strict, DoNotEnforceOnCreate: doNotEnforceOnCreate,
			})},
		}
	}
	change := changeplan.LogicalChange{DesiredState: []configuration.ManagedSetting{{Unit: unit, Desired: setting.RulesetValue(value(true, nil))}}}
	cases := []struct {
		name     string
		observed setting.Ruleset
		want     desiredStateConfirmation
	}{
		{name: "value with the omitted parameter at the value GitHub sets matches", observed: value(true, new(false)), want: desiredStateMatches},
		{name: "value whose declared parameter differs is a mismatch", observed: value(false, new(false)), want: desiredStateMismatch},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			observed := github.ObservationResult{RepositoryID: 1, Rulesets: &github.Rulesets{Existing: github.Observed([]github.Ruleset{github.InterpretedRuleset(42, "protect", tc.observed)})}}
			if got, _ := confirmDesiredState(1, change, observed); got != tc.want {
				t.Errorf("confirmDesiredState() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestConfirmDesiredState_EnvironmentVariables は、Environment Variable の期待状態を、宣言の名前で引いた Environment の変数の値で確かめ、
// Environment がなければ存在しないことを理由に確認失敗にすることを確かめる。
//
//trace:checks FR-012
func TestConfirmDesiredState_EnvironmentVariables(t *testing.T) {
	staging := func(existing ...github.Variable) []github.Environment {
		return []github.Environment{withVariables(observedEnvironment(7, "Staging", setting.DeploymentBranchPolicyAll), existing...)}
	}
	change := environmentVariableValueChange(t, "staging", staging(github.Variable{Name: "REGION", Value: "us-east-1"})[0], "REGION", "us-east-1", "ap-northeast-1")
	cases := []struct {
		name       string
		existing   []github.Environment
		want       desiredStateConfirmation
		wantReason github.UnavailableReason
	}{
		{name: "variable with the declared value matches", existing: staging(github.Variable{Name: "REGION", Value: "ap-northeast-1"}), want: desiredStateMatches},
		{name: "absent environment is unavailable by its absence", existing: nil, want: desiredStateUnavailable, wantReason: github.UnavailableEnvironmentAbsent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			observed := github.ObservationResult{RepositoryID: 1, Environments: environments(tc.existing...)}
			if got, unavailable := confirmDesiredState(1, change, observed); got != tc.want || unavailable.reason != tc.wantReason {
				t.Errorf("confirmDesiredState() = %v, %v, want %v, %v", got, unavailable.reason, tc.want, tc.wantReason)
			}
		})
	}
}

// TestConfirmDesiredState_EnvironmentCreationWithAbsentVariable は、作成にまとめた、存在しないことを宣言した変数を、作成のあとの一覧で確かめることを確かめる。
//
//trace:checks FR-012
func TestConfirmDesiredState_EnvironmentCreationWithAbsentVariable(t *testing.T) {
	change := environmentCreationWithAbsentVariableChange(t, "staging", "DEBUG")
	cases := []struct {
		name           string
		existing       []github.Environment
		want           desiredStateConfirmation
		wantMismatched []setting.Unit
	}{
		{name: "created environment without the variable matches", existing: []github.Environment{withVariables(observedEnvironment(8, "staging", setting.DeploymentBranchPolicyAll))}, want: desiredStateMatches},
		{
			// 作成では先頭の期待値だけでなく、加えた変数の存在しないことも比べる
			name:           "created environment with the variable does not match by the variable",
			existing:       []github.Environment{withVariables(observedEnvironment(8, "staging", setting.DeploymentBranchPolicyAll), github.Variable{Name: "DEBUG", Value: "x"})},
			want:           desiredStateMismatch,
			wantMismatched: []setting.Unit{change.DesiredState[1].Unit},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			observed := github.ObservationResult{RepositoryID: 1, Environments: environments(tc.existing...)}
			if got, _ := confirmDesiredState(1, change, observed); got != tc.want {
				t.Errorf("confirmDesiredState() = %v, want %v", got, tc.want)
			}
			if got := mismatchedManagedSettings(change, observed); !slices.Equal(got, tc.wantMismatched) {
				t.Errorf("mismatchedManagedSettings() = %v, want %v", got, tc.wantMismatched)
			}
		})
	}
}

// TestMismatchedManagedSettings は、期待状態のうち、期待値と異なる値を観測した管理設定を、期待状態の並びで返すことを確かめる。
//
//trace:checks FR-012 FR-013
func TestMismatchedManagedSettings(t *testing.T) {
	general := func(squash, mergeCommit bool) github.ObservationResult {
		return github.ObservationResult{RepositoryID: 1, General: github.General{AllowSquashMerge: squash, AllowMergeCommit: mergeCommit}}
	}
	cases := []struct {
		name     string
		change   changeplan.LogicalChange
		observed github.ObservationResult
		want     []setting.Unit
	}{
		{name: "all matching settings return none", change: mergeMethodSwapChange(t), observed: general(true, false), want: nil},
		{
			name: "only the differing one of the bundled settings is returned", change: mergeMethodSwapChange(t), observed: general(true, true),
			want: []setting.Unit{{Key: setting.AllowMergeCommit}},
		},
		{
			name: "both differing settings are returned in the order of the desired state", change: mergeMethodSwapChange(t), observed: general(false, true),
			want: []setting.Unit{{Key: setting.AllowSquashMerge}, {Key: setting.AllowMergeCommit}},
		},
		{
			// 値を確認できていない設定を、期待値と異なるとは示さないため
			name: "unavailable setting is not returned", change: toSelectedChange(),
			observed: github.ObservationResult{RepositoryID: 1, ActionsPermissions: actionsEnabled(setting.AllowedActionsAll, false)},
			want:     []setting.Unit{{Key: setting.AllowedActions}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mismatchedManagedSettings(tc.change, tc.observed); !slices.Equal(got, tc.want) {
				t.Errorf("mismatchedManagedSettings() = %v, want %v", got, tc.want)
			}
		})
	}
}
