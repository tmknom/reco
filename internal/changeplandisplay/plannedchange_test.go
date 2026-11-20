package changeplandisplay

import (
	"testing"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// 期待値は CLI 仕様 4章の表示例から導く。
//
//trace:checks FR-005
//trace:checks FR-007
//trace:checks QR-002
//trace:checks INV-001
func TestPlannedChangeText(t *testing.T) {
	criteria := setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, VerifiedAllowed: true, PatternsAllowed: []string{"example/*"}}
	policy := setting.AllowedActionsPolicyValue
	disabled := github.Unavailable[setting.Value](github.UnavailableActionsDisabled)
	notSelected := github.Unavailable[setting.Value](github.UnavailableNotSelected)
	const possibleLoss = "    Side effect: GitHub may discard selected_actions if allowed_actions was \"selected\" before GitHub Actions was disabled (reco cannot read them while GitHub Actions is disabled)\n"
	cases := []struct {
		name string
		s    changeplan.SettingPlan
		want string
	}{
		{
			name: "single setting mismatch is shown as current -> desired on one line",
			s: changeplan.SettingPlan{
				Key: setting.AllowAutoMerge, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true),
				Result: changeplan.Mismatch, Applicability: changeplan.Applicable,
			},
			want: "  general.allow_auto_merge: false -> true\n",
		},
		{
			name: "criteria group mismatch shows the current and desired values on separate lines",
			s: changeplan.SettingPlan{
				Key:      setting.SelectedActions,
				Observed: github.Observed(setting.SelectedActionsCriteriaValue(setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, PatternsAllowed: []string{}})),
				Desired:  setting.SelectedActionsCriteriaValue(criteria), Result: changeplan.Mismatch, Applicability: changeplan.Applicable,
			},
			want: "  actions.permissions.selected_actions:\n" +
				"    github_owned_allowed: true, verified_allowed: false, patterns_allowed: []\n" +
				"    -> github_owned_allowed: true, verified_allowed: true, patterns_allowed: [\"example/*\"]\n",
		},
		{
			name: "not-comparable criteria group under a policy other than selected shows not in use as the repository state on separate lines",
			s: changeplan.SettingPlan{
				Key: setting.SelectedActions, Observed: notSelected, Desired: setting.SelectedActionsCriteriaValue(criteria),
				Result: changeplan.NotComparable, Applicability: changeplan.Applicable,
			},
			want: "  actions.permissions.selected_actions:\n" +
				"    (not in use)\n" +
				"    -> github_owned_allowed: true, verified_allowed: true, patterns_allowed: [\"example/*\"]\n",
		},
		{
			name: "not-comparable criteria group on disabled actions shows actions disabled as the repository state",
			s: changeplan.SettingPlan{
				Key: setting.SelectedActions, Observed: disabled, Desired: setting.SelectedActionsCriteriaValue(criteria),
				Result: changeplan.NotComparable, Applicability: changeplan.Applicable,
			},
			want: "  actions.permissions.selected_actions:\n" +
				"    (GitHub Actions disabled)\n" +
				"    -> github_owned_allowed: true, verified_allowed: true, patterns_allowed: [\"example/*\"]\n",
		},
		{
			name: "not-comparable policy that enables actions shows the repository state on one line and marks the desired value",
			s: changeplan.SettingPlan{
				Key: setting.AllowedActions, Observed: disabled, Desired: policy(setting.AllowedActionsSelected),
				Result: changeplan.NotComparable, Applicability: changeplan.Applicable,
			},
			want: "  actions.permissions.allowed_actions: (GitHub Actions disabled) -> selected (enables GitHub Actions)\n",
		},
		{
			name: "known change shows the side effect with the observed criteria under it",
			s: changeplan.SettingPlan{
				Key: setting.AllowedActions, Observed: github.Observed(policy(setting.AllowedActionsSelected)), Desired: policy(setting.AllowedActionsAll),
				Result: changeplan.Mismatch, Applicability: changeplan.Applicable,
				SideEffects: []changeplan.SideEffect{{Key: setting.SelectedActions, Observed: github.Observed(setting.SelectedActionsCriteriaValue(criteria))}},
			},
			want: "  actions.permissions.allowed_actions: selected -> all\n" +
				"    Side effect: GitHub discards the current selected_actions (github_owned_allowed: true, verified_allowed: true, patterns_allowed: [\"example/*\"])\n",
		},
		{
			name: "not-comparable setting shows the possible loss when the observed criteria are unavailable",
			s: changeplan.SettingPlan{
				Key: setting.AllowedActions, Observed: disabled, Desired: policy(setting.AllowedActionsLocalOnly),
				Result: changeplan.NotComparable, Applicability: changeplan.Applicable,
				SideEffects: []changeplan.SideEffect{{Key: setting.SelectedActions, Observed: disabled}},
			},
			want: "  actions.permissions.allowed_actions: (GitHub Actions disabled) -> local_only (enables GitHub Actions)\n" +
				possibleLoss,
		},
		{
			name: "policy enabling actions shows the one resumed setting before the side effect",
			s: changeplan.SettingPlan{
				Key: setting.AllowedActions, Observed: disabled, Desired: policy(setting.AllowedActionsLocalOnly),
				Result: changeplan.NotComparable, Applicability: changeplan.Applicable,
				SideEffects:     []changeplan.SideEffect{{Key: setting.SelectedActions, Observed: disabled}},
				ResumedSettings: []setting.Key{setting.SHAPinningRequired},
			},
			want: "  actions.permissions.allowed_actions: (GitHub Actions disabled) -> local_only (enables GitHub Actions)\n" +
				"    Previous settings: GitHub Actions resumes with sha_pinning_required kept from before it was disabled (reco cannot read it while GitHub Actions is disabled)\n" +
				possibleLoss,
		},
		{
			name: "reenabling actions lists the resumed settings",
			s: changeplan.SettingPlan{
				Key: setting.ActionsEnabled, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true),
				Result: changeplan.Mismatch, Applicability: changeplan.Applicable,
				ResumedSettings: []setting.Key{setting.AllowedActions, setting.SelectedActions, setting.SHAPinningRequired},
			},
			want: "  actions.permissions.enabled: false -> true\n" +
				"    Previous settings: GitHub Actions resumes with allowed_actions, selected_actions, and sha_pinning_required kept from before it was disabled (reco cannot read them while GitHub Actions is disabled)\n",
		},
		{
			name: "reenabling actions with sha_pinning_required declared lists only the policy and the criteria",
			s: changeplan.SettingPlan{
				Key: setting.ActionsEnabled, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true),
				Result: changeplan.Mismatch, Applicability: changeplan.Applicable,
				ResumedSettings: []setting.Key{setting.AllowedActions, setting.SelectedActions},
			},
			want: "  actions.permissions.enabled: false -> true\n" +
				"    Previous settings: GitHub Actions resumes with allowed_actions and selected_actions kept from before it was disabled (reco cannot read them while GitHub Actions is disabled)\n",
		},
		{
			name: "not-applicable change shows the applicability with the reason under it",
			s: changeplan.SettingPlan{
				Key: setting.SHAPinningRequired, Observed: disabled, Desired: setting.BoolValue(true),
				Result: changeplan.NotComparable, Applicability: changeplan.NotApplicable, NotApplicableReason: changeplan.NotApplicableActionsRemainDisabled,
			},
			want: "  actions.permissions.sha_pinning_required: (GitHub Actions disabled) -> true\n" +
				"    Applicability: not applicable (GitHub Actions is disabled; declare enabled: true or allowed_actions to enable it)\n",
		},
		{
			name: "access_level of a public repository shows the repository state and the reason to remove the declaration",
			s: changeplan.SettingPlan{
				Key: setting.ActionsAccessLevel, Observed: github.Unavailable[setting.Value](github.UnavailablePublicRepository), Desired: setting.AccessLevelValue(setting.AccessNone),
				Result: changeplan.NotComparable, Applicability: changeplan.NotApplicable, NotApplicableReason: changeplan.NotApplicablePublicRepository,
			},
			want: "  actions.access.access_level: (public repository) -> none\n" +
				"    Applicability: not applicable (the repository is public; remove access_level, which applies only to private repositories)\n",
		},
		{
			name: "applicable not-comparable sha_pinning_required shows no applicability",
			s: changeplan.SettingPlan{
				Key: setting.SHAPinningRequired, Observed: disabled, Desired: setting.BoolValue(true),
				Result: changeplan.NotComparable, Applicability: changeplan.Applicable,
			},
			want: "  actions.permissions.sha_pinning_required: (GitHub Actions disabled) -> true\n",
		},
		{
			name: "match is empty",
			s: changeplan.SettingPlan{
				Key: setting.AllowAutoMerge, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(true),
				Result: changeplan.Match,
			},
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PlannedChangeText(tc.s); got != tc.want {
				t.Errorf("PlannedChangeText() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPlannedChangeText_SideEffectOnOtherSettingPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("PlannedChangeText() did not panic for a side effect on another setting")
		}
	}()
	_ = PlannedChangeText(changeplan.SettingPlan{
		Key: setting.AllowAutoMerge, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true),
		Result: changeplan.Mismatch, Applicability: changeplan.Applicable,
		SideEffects: []changeplan.SideEffect{{Key: setting.AllowAutoMerge, Observed: github.Observed(setting.BoolValue(true))}},
	})
}

func TestNotApplicableReasonText_UndefinedReasonPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("notApplicableReasonText() did not panic for an undefined not-applicable reason")
		}
	}()
	_ = notApplicableReasonText(0)
}

func TestRepositoryStateText_UndefinedReasonPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("repositoryStateText() did not panic for an undefined unavailable reason")
		}
	}()
	_ = repositoryStateText(0)
}
