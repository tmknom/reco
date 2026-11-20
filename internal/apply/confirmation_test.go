package apply

import (
	"testing"

	"github.com/tmknom/reco/internal/changeplan"
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
			if got, reason := confirmDesiredState(1, change, tc.observed); got != tc.want || reason != 0 {
				t.Errorf("confirmDesiredState() = %v, %v, want %v without a reason", got, reason, tc.want)
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
			name: "mismatch after an unavailable desired value in the order of updates is still a mismatch",
			change: changeplan.LogicalChange{Operations: []changeplan.UpdateOperation{
				toSelectedChange().Operations[1], toSelectedChange().Operations[0],
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
			if got, reason := confirmDesiredState(1, tc.change, observed); got != tc.want || reason != tc.wantReason {
				t.Errorf("confirmDesiredState() = %v, %v, want %v, %v", got, reason, tc.want, tc.wantReason)
			}
		})
	}
}

// TestConfirmDesiredState_Environments は、Environment の作成と方式の変更の更新後の判定表を確かめる。
//
//trace:checks INV-004
//trace:checks FR-012
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
			change:       creationChange(t, "production", setting.Value{}),
			environments: environments(observedEnvironment(8, "Production", setting.DeploymentBranchPolicyAll)),
			want:         desiredStateMatches,
		},
		{
			name:         "creation is a mismatch when the environment is absent",
			change:       creationChange(t, "production", setting.Value{}),
			environments: environments(approvedStaging),
			want:         desiredStateMismatch,
		},
		{
			name:         "creation is unavailable when the existence cannot be determined",
			change:       creationChange(t, "production", setting.Value{}),
			environments: environments(undetermined),
			want:         desiredStateUnavailable,
			wantReason:   github.UnavailableEnvironmentNameUndetermined,
		},
		{
			name:         "creation with a mode matches when the environment has the mode",
			change:       creationChange(t, "production", modeValue(setting.DeploymentBranchPolicyProtectedBranches)),
			environments: environments(observedEnvironment(8, "production", setting.DeploymentBranchPolicyProtectedBranches)),
			want:         desiredStateMatches,
		},
		{
			name:         "creation with a mode is a mismatch when the environment has another mode",
			change:       creationChange(t, "production", modeValue(setting.DeploymentBranchPolicyProtectedBranches)),
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
			wantReason:   github.UnavailableEnvironmentNameUndetermined,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			observed := github.ObservationResult{RepositoryID: 1, Environments: tc.environments}
			if got, reason := confirmDesiredState(1, tc.change, observed); got != tc.want || reason != tc.wantReason {
				t.Errorf("confirmDesiredState() = %v, %v, want %v, %v", got, reason, tc.want, tc.wantReason)
			}
		})
	}
}
