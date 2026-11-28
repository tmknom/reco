package changeplandisplay

import (
	"testing"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

func environmentDeletionPlan(t *testing.T, loss *changeplan.EnvironmentLoss, applicability changeplan.Applicability) changeplan.SettingPlan {
	t.Helper()
	return changeplan.SettingPlan{
		Unit:     setting.Unit{Key: setting.EnvironmentResource, Environment: mustEnvironmentName(t, "legacy")},
		Observed: github.Observed(setting.Present()), Desired: setting.Absent(),
		Result: changeplan.Mismatch, Applicability: applicability, EnvironmentLoss: loss,
	}
}

func TestEnvironmentDeletedValueLines(t *testing.T) {
	octocat := setting.Reviewer{Type: setting.ReviewerUser, ID: 583231}
	team := setting.Reviewer{Type: setting.ReviewerTeam, ID: 42}
	selected := setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicySelected, Patterns: []setting.DeploymentPattern{{Type: setting.DeploymentPatternBranch, Name: "main"}}}
	cases := []struct {
		name  string
		value setting.EnvironmentDeletedValue
		want  string
	}{
		{
			name: "all five items in the decided order, with the patterns and the login of a reviewer",
			value: setting.EnvironmentDeletedValue{
				DeploymentBranchPolicy: selected,
				ReviewerSet:            setting.ReviewerSetOfRule(&setting.RequiredReviewers{Reviewers: []setting.Reviewer{octocat, team}, Logins: map[setting.Reviewer]string{octocat: "octocat"}}),
				PreventSelfReview:      true, WaitTimer: 30, CanAdminsBypass: true,
			},
			want: "    deployment_branch_policy: selected [branch \"main\"]\n    reviewers: [User 583231 (octocat), Team 42]\n    prevent_self_review: true\n    wait_timer: 30 minutes\n    can_admins_bypass: true\n",
		},
		{
			name: "an environment without protection rules still shows every item with its default value",
			value: setting.EnvironmentDeletedValue{
				DeploymentBranchPolicy: setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyAll},
			},
			want: "    deployment_branch_policy: all\n    reviewers: []\n    prevent_self_review: false\n    wait_timer: 0 minutes\n    can_admins_bypass: false\n",
		},
		{
			name: "protected branches mode and a protection rule without reviewers",
			value: setting.EnvironmentDeletedValue{
				DeploymentBranchPolicy: setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyProtectedBranches},
				ReviewerSet:            setting.ReviewerSetOfRule(&setting.RequiredReviewers{}),
				CanAdminsBypass:        true,
			},
			want: "    deployment_branch_policy: protected_branches\n    reviewers: [] (protection rule without reviewers)\n    prevent_self_review: false\n    wait_timer: 0 minutes\n    can_admins_bypass: true\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := environmentDeletionPlan(t, &changeplan.EnvironmentLoss{DeletedValue: tc.value}, changeplan.Applicable())
			if got := environmentDeletedValueLines(s); got != tc.want {
				t.Errorf("environmentDeletedValueLines() = %q, want %q", got, tc.want)
			}
		})
	}
	t.Run("a plan without the environment loss returns nothing", func(t *testing.T) {
		if got := environmentDeletedValueLines(environmentDeletionPlan(t, nil, changeplan.Applicable())); got != "" {
			t.Errorf("environmentDeletedValueLines() = %q, want empty", got)
		}
	})
}

// Environment の削除の表示は、値の行、適用不能の行、解釈しなかったもの、副作用の順に並ぶ。
//
//trace:checks FR-005 FR-007 QR-002 INV-001
func TestPlannedChangeText_EnvironmentDeletion(t *testing.T) {
	deleted := setting.EnvironmentDeletedValue{
		DeploymentBranchPolicy: setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyAll},
		WaitTimer:              5,
	}
	const valueLines = "    deployment_branch_policy: all\n    reviewers: []\n    prevent_self_review: false\n    wait_timer: 5 minutes\n    can_admins_bypass: false\n"
	variables := []github.Variable{{Name: "REGION", Value: "ap-northeast-1"}, {Name: "NOTE", Value: "a\nb"}}
	secrets := []github.EnvironmentSecret{{Name: "DEPLOY_KEY"}, {Name: "TO\nKEN"}}
	variablesEffect := changeplan.SideEffect{Kind: changeplan.SideEffectEnvironmentVariablesDeleted}
	secretsEffect := changeplan.SideEffect{Kind: changeplan.SideEffectEnvironmentSecretsDeleted}
	parts := []github.UninterpretedPart{{Kind: github.UninterpretedProtectionRuleType, Name: "custom"}, {Kind: github.UninterpretedProtectionRuleType, Name: "a\"b"}}

	t.Run("deletion shows the lost values then the side effects in order", func(t *testing.T) {
		s := environmentDeletionPlan(t, &changeplan.EnvironmentLoss{DeletedValue: deleted, Variables: variables, Secrets: secrets}, changeplan.Applicable())
		s.SideEffects = []changeplan.SideEffect{variablesEffect, secretsEffect}
		want := "  environments.legacy: present -> absent\n" + valueLines +
			"    Side effect: GitHub also deletes the environment variables \"REGION\": \"ap-northeast-1\", \"NOTE\": \"a\\nb\"\n" +
			"    Side effect: GitHub also deletes the environment secrets \"DEPLOY_KEY\", \"TO\\nKEN\" (reco cannot read their values)\n"
		if got := PlannedChangeText(s); got != want {
			t.Errorf("PlannedChangeText() = %q, want %q", got, want)
		}
	})
	t.Run("deletion with only secrets shows only the secrets side effect", func(t *testing.T) {
		s := environmentDeletionPlan(t, &changeplan.EnvironmentLoss{DeletedValue: deleted, Secrets: secrets[:1]}, changeplan.Applicable())
		s.SideEffects = []changeplan.SideEffect{secretsEffect}
		want := "  environments.legacy: present -> absent\n" + valueLines +
			"    Side effect: GitHub also deletes the environment secrets \"DEPLOY_KEY\" (reco cannot read their values)\n"
		if got := PlannedChangeText(s); got != want {
			t.Errorf("PlannedChangeText() = %q, want %q", got, want)
		}
	})
	t.Run("deletion of an environment without variables and secrets shows no side effect", func(t *testing.T) {
		s := environmentDeletionPlan(t, &changeplan.EnvironmentLoss{DeletedValue: deleted}, changeplan.Applicable())
		want := "  environments.legacy: present -> absent\n" + valueLines
		if got := PlannedChangeText(s); got != want {
			t.Errorf("PlannedChangeText() = %q, want %q", got, want)
		}
	})
	t.Run("not applicable deletion shows no value lines, the reason, then the uninterpreted parts", func(t *testing.T) {
		s := environmentDeletionPlan(t, nil, changeplan.NotApplicable(changeplan.NotApplicableUninterpretedParts))
		s.UninterpretedParts = parts
		want := "  environments.legacy: present -> absent\n" +
			"    Applicability: not applicable (the environment on GitHub has protection rules that reco does not interpret; remove this environment from the configuration)\n" +
			"      protection rule type: \"custom\"\n      protection rule type: \"a\\\"b\"\n"
		if got := PlannedChangeText(s); got != want {
			t.Errorf("PlannedChangeText() = %q, want %q", got, want)
		}
	})
	t.Run("not applicable deletion for an unknown value shows the reason without parts", func(t *testing.T) {
		s := environmentDeletionPlan(t, nil, changeplan.NotApplicable(changeplan.NotApplicableUnknownValue))
		want := "  environments.legacy: present -> absent\n" +
			"    Applicability: not applicable (the environment on GitHub has a value that reco does not know; remove this environment from the configuration)\n"
		if got := PlannedChangeText(s); got != want {
			t.Errorf("PlannedChangeText() = %q, want %q", got, want)
		}
	})
	t.Run("deletion of an environment whose name cannot be compared shows existence unknown and the reason", func(t *testing.T) {
		s := changeplan.SettingPlan{
			Unit:     setting.Unit{Key: setting.EnvironmentResource, Environment: mustEnvironmentName(t, "legacy")},
			Observed: github.Unavailable[setting.Value](github.UnavailableNameUndetermined), Desired: setting.Absent(),
			Result: changeplan.NotComparable, Applicability: changeplan.NotApplicable(changeplan.NotApplicableNameUndetermined),
		}
		want := "  environments.legacy: (existence unknown) -> absent\n" +
			"    Applicability: not applicable (an environment on GitHub has a name reco cannot compare, such as one with non-ASCII characters; remove this environment from the configuration)\n"
		if got := PlannedChangeText(s); got != want {
			t.Errorf("PlannedChangeText() = %q, want %q", got, want)
		}
	})
}

// 解釈しなかったものと適用不能の理由の整合は、Ruleset と同じく Environment でも確かめる。
func TestPlannedChangeText_EnvironmentUninterpretedPartsMismatchPanics(t *testing.T) {
	part := []github.UninterpretedPart{{Kind: github.UninterpretedProtectionRuleType, Name: "custom"}}
	cases := []struct {
		name          string
		applicability changeplan.Applicability
		parts         []github.UninterpretedPart
	}{
		{"reason without parts panics", changeplan.NotApplicable(changeplan.NotApplicableUninterpretedParts), nil},
		{"parts without the reason panic", changeplan.NotApplicable(changeplan.NotApplicableUnknownValue), part},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("PlannedChangeText() did not panic")
				}
			}()
			s := environmentDeletionPlan(t, nil, tc.applicability)
			s.UninterpretedParts = tc.parts
			_ = PlannedChangeText(s)
		})
	}
}
