package changeplan

import (
	"reflect"
	"slices"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

//trace:checks FR-006
func TestJudgeApplicability(t *testing.T) {
	notComparable := func(key setting.Key, reason github.UnavailableReason) SettingPlan {
		return SettingPlan{Unit: setting.Unit{Key: key}, Observed: github.Unavailable[setting.Value](reason), Result: NotComparable}
	}
	shaPinningDisabled := notComparable(setting.SHAPinningRequired, github.UnavailableActionsDisabled)
	generalMismatch := SettingPlan{Unit: setting.Unit{Key: setting.AllowAutoMerge}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch}
	enablesActions := SettingPlan{Unit: setting.Unit{Key: setting.ActionsEnabled}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch}
	accessLevelPublic := notComparable(setting.ActionsAccessLevel, github.UnavailablePublicRepository)
	production, staging := mustEnvironmentName(t, "production"), mustEnvironmentName(t, "staging")
	creation := func(name setting.EnvironmentName) SettingPlan {
		return SettingPlan{Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: name}, Observed: github.Observed(setting.Absent()), Desired: setting.Present(), Result: Mismatch}
	}
	environmentPolicy := func(name setting.EnvironmentName, reason github.UnavailableReason) SettingPlan {
		return SettingPlan{
			Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: name}, Observed: github.Unavailable[setting.Value](reason),
			Desired: setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyAll}), Result: NotComparable,
		}
	}
	environmentVariable := func(name setting.EnvironmentName, reason github.UnavailableReason) SettingPlan {
		return SettingPlan{
			Unit: environmentVariableUnit(name, mustVariableName(t, "REGION")), Observed: github.Unavailable[setting.Value](reason),
			Desired: setting.VariableValue("eu"), Result: NotComparable,
		}
	}
	undeterminedEnvironment := SettingPlan{
		Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: production}, Observed: github.Unavailable[setting.Value](github.UnavailableNameUndetermined), Desired: setting.Present(), Result: NotComparable,
	}
	cases := []struct {
		name string
		in   SettingPlan
		// settings は、in と同じリポジトリの計画に並ぶ管理設定の計画。in を含む。
		settings []SettingPlan
		want     Applicability
	}{
		{
			name: "match is not judged",
			in:   SettingPlan{Unit: setting.Unit{Key: setting.AllowAutoMerge}, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(true), Result: Match},
			want: Applicability{},
		},
		{
			name: "mismatch is applicable",
			in:   SettingPlan{Unit: setting.Unit{Key: setting.AllowAutoMerge}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch},
			want: Applicable(),
		},
		{
			name: "allowed_actions not comparable because actions is disabled is applicable",
			in:   notComparable(setting.AllowedActions, github.UnavailableActionsDisabled),
			want: Applicable(),
		},
		{
			name: "selected_actions not comparable because the policy is not selected is applicable",
			in:   notComparable(setting.SelectedActions, github.UnavailableNotSelected),
			want: Applicable(),
		},
		{
			name: "selected_actions not comparable because actions is disabled is applicable",
			in:   notComparable(setting.SelectedActions, github.UnavailableActionsDisabled),
			want: Applicable(),
		},
		{
			name: "sha_pinning_required mismatch is applicable without a declaration enabling actions",
			in:   SettingPlan{Unit: setting.Unit{Key: setting.SHAPinningRequired}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch},
			want: Applicable(),
		},
		{
			name:     "sha_pinning_required not comparable because actions is disabled is not applicable when actions stays disabled",
			in:       shaPinningDisabled,
			settings: []SettingPlan{generalMismatch, shaPinningDisabled},
			want:     NotApplicable(NotApplicableActionsRemainDisabled),
		},
		{
			name:     "sha_pinning_required not comparable because actions is disabled is applicable when the same plan enables actions",
			in:       shaPinningDisabled,
			settings: []SettingPlan{enablesActions, shaPinningDisabled},
			want:     Applicable(),
		},
		{
			name: "access_level not comparable because the repository is public is not applicable",
			in:   accessLevelPublic,
			want: NotApplicable(NotApplicablePublicRepository),
		},
		{
			name:     "access_level not comparable because the repository is public stays not applicable when the same plan enables actions",
			in:       accessLevelPublic,
			settings: []SettingPlan{enablesActions, accessLevelPublic},
			want:     NotApplicable(NotApplicablePublicRepository),
		},
		{
			name: "environment whose existence cannot be determined is not applicable",
			in:   undeterminedEnvironment,
			want: NotApplicable(NotApplicableNameUndetermined),
		},
		{
			name:     "mode of an environment whose existence cannot be determined is not applicable",
			in:       environmentPolicy(production, github.UnavailableNameUndetermined),
			settings: []SettingPlan{undeterminedEnvironment, environmentPolicy(production, github.UnavailableNameUndetermined)},
			want:     NotApplicable(NotApplicableNameUndetermined),
		},
		{
			name: "secret_scanning not comparable because the repository is private is not applicable",
			in:   notComparable(setting.SecretScanningEnabled, github.UnavailablePrivateRepository),
			want: NotApplicable(NotApplicablePrivateRepository),
		},
		{
			name: "push protection not comparable because the repository is private is not applicable",
			in:   notComparable(setting.RepositoryPushProtection, github.UnavailablePrivateRepository),
			want: NotApplicable(NotApplicablePrivateRepository),
		},
		{
			name: "private_vulnerability_reporting not comparable because the repository is private is not applicable",
			in:   notComparable(setting.PrivateVulnerabilityReportingEnabled, github.UnavailablePrivateRepository),
			want: NotApplicable(NotApplicablePrivateRepository),
		},
		{
			name:     "approval_policy not comparable because the repository is private is not applicable even if the plan enables actions",
			in:       notComparable(setting.ForkPullRequestApproval, github.UnavailablePrivateRepository),
			settings: []SettingPlan{enablesActions, notComparable(setting.ForkPullRequestApproval, github.UnavailablePrivateRepository)},
			want:     NotApplicable(NotApplicablePrivateRepository),
		},
		{
			name:     "private fork pull request workflows not comparable because the repository is public is not applicable even if the plan enables actions",
			in:       notComparable(setting.PrivateForkPullRequestWorkflows, github.UnavailablePublicRepository),
			settings: []SettingPlan{enablesActions, notComparable(setting.PrivateForkPullRequestWorkflows, github.UnavailablePublicRepository)},
			want:     NotApplicable(NotApplicablePublicRepository),
		},
		{
			name:     "mode of an absent environment created in the same plan is applicable",
			in:       environmentPolicy(production, github.UnavailableEnvironmentAbsent),
			settings: []SettingPlan{creation(production), environmentPolicy(production, github.UnavailableEnvironmentAbsent)},
			want:     Applicable(),
		},
		{
			name:     "variable of an absent environment created in the same plan is applicable",
			in:       environmentVariable(production, github.UnavailableEnvironmentAbsent),
			settings: []SettingPlan{creation(production), environmentVariable(production, github.UnavailableEnvironmentAbsent)},
			want:     Applicable(),
		},
		{
			name:     "variable of an environment whose existence cannot be determined is not applicable",
			in:       environmentVariable(production, github.UnavailableNameUndetermined),
			settings: []SettingPlan{undeterminedEnvironment, environmentVariable(production, github.UnavailableNameUndetermined)},
			want:     NotApplicable(NotApplicableNameUndetermined),
		},
		{
			name: "allowed_actions not comparable because of an unknown value is not applicable",
			in:   notComparable(setting.AllowedActions, github.UnavailableUnknownValue),
			want: NotApplicable(NotApplicableUnknownValue),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			settings := tc.settings
			if settings == nil {
				settings = []SettingPlan{tc.in}
			}
			got := judgeApplicability(tc.in, settings, github.ObservationResult{})
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("judgeApplicability() = %+v, want %+v", got, tc.want)
			}
		})
	}

	// 遷移表にない比較不能を適用可能として黙って通さない
	panics := []struct {
		name     string
		in       SettingPlan
		settings []SettingPlan
	}{
		{name: "allowed_actions not comparable for a reason other than disabled actions panics", in: notComparable(setting.AllowedActions, github.UnavailableNotSelected)},
		{name: "not comparable setting without a fetch condition panics", in: notComparable(setting.AllowAutoMerge, github.UnavailableActionsDisabled)},
		{name: "sha_pinning_required not comparable for a reason other than disabled actions panics", in: notComparable(setting.SHAPinningRequired, github.UnavailableNotSelected)},
		{name: "access_level not comparable for a reason other than a public repository panics", in: notComparable(setting.ActionsAccessLevel, github.UnavailableActionsDisabled)},
		{name: "setting other than access_level not comparable because the repository is public panics", in: notComparable(setting.AllowAutoMerge, github.UnavailablePublicRepository)},
		{name: "approval_policy not comparable for a reason other than a private repository panics", in: notComparable(setting.ForkPullRequestApproval, github.UnavailablePublicRepository)},
		{name: "private fork pull request workflows not comparable for a reason other than a public repository panics", in: notComparable(setting.PrivateForkPullRequestWorkflows, github.UnavailablePrivateRepository)},
		{
			name:     "mode of an absent environment without its creation in the plan panics",
			in:       environmentPolicy(production, github.UnavailableEnvironmentAbsent),
			settings: []SettingPlan{creation(staging), environmentPolicy(production, github.UnavailableEnvironmentAbsent)},
		},
		{
			name:     "variable of an absent environment without its creation in the plan panics",
			in:       environmentVariable(production, github.UnavailableEnvironmentAbsent),
			settings: []SettingPlan{creation(staging), environmentVariable(production, github.UnavailableEnvironmentAbsent)},
		},
		{name: "repository setting not comparable because an environment is absent panics", in: notComparable(setting.AllowAutoMerge, github.UnavailableEnvironmentAbsent)},
		{name: "setting other than those available only on public repositories not comparable because the repository is private panics", in: notComparable(setting.AllowAutoMerge, github.UnavailablePrivateRepository)},
		{name: "secret scanning not comparable for a reason other than a private repository panics", in: notComparable(setting.SecretScanningEnabled, github.UnavailablePublicRepository)},
		{name: "private_vulnerability_reporting not comparable for a reason other than a private repository panics", in: notComparable(setting.PrivateVulnerabilityReportingEnabled, github.UnavailablePublicRepository)},
	}
	for _, tc := range panics {
		t.Run(tc.name, func(t *testing.T) {
			settings := tc.settings
			if settings == nil {
				settings = []SettingPlan{tc.in}
			}
			defer func() {
				if recover() == nil {
					t.Error("judgeApplicability() did not panic")
				}
			}()
			_ = judgeApplicability(tc.in, settings, github.ObservationResult{})
		})
	}
}

// TestJudgeApplicability_EnvironmentDeletion は、Environment の削除の変更予定を、削除で失われる値の全体を承認の前に示せないときだけ、
// 示せない理由 (解釈しない部分か知らない値) で適用不能とすることを確かめる。
// 示せるかと理由は、reconcile が計画に入れた削除で失われるものと解釈しなかったものから決める。
// 計画時の観測は、計画と食い違う Environment (知らない種類の保護ルールを持つ) にし、観測を引き直す実装を失敗させる。
//
//trace:checks FR-006 INV-001
func TestJudgeApplicability_EnvironmentDeletion(t *testing.T) {
	production := mustEnvironmentName(t, "production")
	unit := setting.Unit{Key: setting.EnvironmentResource, Environment: production}
	uninterpreted := github.UninterpretedPart{Kind: github.UninterpretedProtectionRuleType, Name: "custom"}
	// deletion は、削除で失われるもの loss と解釈しなかったもの parts を持つ削除の変更予定
	deletion := func(loss *EnvironmentLoss, parts ...github.UninterpretedPart) SettingPlan {
		return SettingPlan{Unit: unit, Observed: github.Observed(setting.Present()), Desired: setting.Absent(), Result: Mismatch, EnvironmentLoss: loss, UninterpretedParts: parts}
	}
	withUnknownRule := github.Environment{ID: 7, Name: "production", DeploymentBranchPolicyMode: setting.DeploymentBranchPolicyAll, Protection: unprotected()}
	withUnknownRule.UninterpretedProtectionRules = []github.UninterpretedPart{uninterpreted}
	observed := github.ObservationResult{Environments: &github.Environments{Existing: []github.Environment{withUnknownRule}}}
	cases := []struct {
		name string
		in   SettingPlan
		want Applicability
	}{
		{name: "deletion that can show what it loses is applicable", in: deletion(&EnvironmentLoss{}), want: Applicable()},
		{
			name: "deletion without the loss with uninterpreted parts is not applicable because of the uninterpreted parts",
			in:   deletion(nil, uninterpreted), want: NotApplicable(NotApplicableUninterpretedParts),
		},
		{
			name: "deletion without the loss or uninterpreted parts is not applicable because of the unknown value",
			in:   deletion(nil), want: NotApplicable(NotApplicableUnknownValue),
		},
		{
			name: "creation without the loss is applicable",
			in:   SettingPlan{Unit: unit, Observed: github.Observed(setting.Absent()), Desired: setting.Present(), Result: Mismatch},
			want: Applicable(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := judgeApplicability(tc.in, []SettingPlan{tc.in}, observed); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("judgeApplicability() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestJudgeApplicability_PreventSelfReview は、自己承認の禁止の変更予定を、更新時点に担当者がいない状態になる見込みで、
// prevent_self_review を送らずには期待値にならないときに適用不能とすることを確かめる。
//
//trace:checks FR-006 INV-001
func TestJudgeApplicability_PreventSelfReview(t *testing.T) {
	production := mustEnvironmentName(t, "production")
	preventUnit := setting.Unit{Key: setting.EnvironmentPreventSelfReview, Environment: production}
	reviewersUnit := setting.Unit{Key: setting.EnvironmentReviewerSet, Environment: production}
	user1 := setting.Reviewer{Type: setting.ReviewerUser, ID: 1}
	observedWith := func(protection github.EnvironmentProtection) github.ObservationResult {
		return github.ObservationResult{Environments: &github.Environments{Existing: []github.Environment{{ID: 7, Name: "production", Protection: protection}}}}
	}
	noRule := observedWith(unprotected())
	withReviewer := observedWith(protectedByRule(setting.RequiredReviewers{Reviewers: []setting.Reviewer{user1}}, true))
	ruleWithoutReviewers := observedWith(protectedByRule(setting.RequiredReviewers{Reviewers: []setting.Reviewer{}}, true))
	unknownReviewers := observedWith(github.EnvironmentProtection{ReviewerSet: github.Unavailable[setting.ReviewerSet](github.UnavailableUnknownValue)})
	absent := github.ObservationResult{Environments: &github.Environments{}}
	prevent := func(observed github.ObservationResult, desired bool) SettingPlan {
		return newSettingPlan(preventUnit, observed.Setting(preventUnit), setting.BoolValue(desired))
	}
	reviewers := func(observed github.ObservationResult, desired ...setting.Reviewer) SettingPlan {
		return newSettingPlan(reviewersUnit, observed.Setting(reviewersUnit), setting.ReviewerSetValue(setting.ReviewerSetOf(desired)))
	}
	creation := SettingPlan{Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: production}, Observed: github.Observed(setting.Absent()), Desired: setting.Present(), Result: Mismatch}
	notApplicable := NotApplicable(NotApplicableNoReviewers)
	cases := []struct {
		name     string
		observed github.ObservationResult
		in       SettingPlan
		// others は、in と同じリポジトリの計画に並ぶ、in のほかの管理設定の計画。
		others []SettingPlan
		want   Applicability
	}{
		{name: "true without a protection rule and without declared reviewers is not applicable", observed: noRule, in: prevent(noRule, true), want: notApplicable},
		{name: "false of a rule without reviewers and without declared reviewers is not applicable", observed: ruleWithoutReviewers, in: prevent(ruleWithoutReviewers, false), want: notApplicable},
		{name: "true without a protection rule is applicable with a declared reviewer", observed: noRule, in: prevent(noRule, true), others: []SettingPlan{reviewers(noRule, user1)}, want: Applicable()},
		{name: "false with an observed reviewer and without declared reviewers is applicable", observed: withReviewer, in: prevent(withReviewer, false), want: Applicable()},
		{
			name: "false with declared no reviewers is applicable because removing every reviewer makes it false", observed: withReviewer, in: prevent(withReviewer, false),
			others: []SettingPlan{reviewers(withReviewer)}, want: Applicable(),
		},
		{
			// 担当者の集合が知らない値で観測不能になるのは、知らない種類の担当者が一人以上いるときに限る
			name: "true with reviewers of an unknown type and without declared reviewers is applicable", observed: unknownReviewers, in: prevent(unknownReviewers, true),
			want: Applicable(),
		},
		{name: "true of an environment to create without declared reviewers is not applicable", observed: absent, in: prevent(absent, true), others: []SettingPlan{creation}, want: notApplicable},
		{name: "false of an environment to create is applicable because a created environment has false", observed: absent, in: prevent(absent, false), others: []SettingPlan{creation}, want: Applicable()},
		{
			name: "true of an environment to create is applicable with a declared reviewer", observed: absent, in: prevent(absent, true),
			others: []SettingPlan{creation, reviewers(absent, user1)}, want: Applicable(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			settings := append([]SettingPlan{tc.in}, tc.others...)
			if got := judgeApplicability(tc.in, settings, tc.observed); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("judgeApplicability() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestJudgeApplicability_Rulesets は、Ruleset の不一致を適用可能とし、比較不能を理由ごとの適用不能にすることを確かめる。
//
//trace:checks FR-006 INV-001
func TestJudgeApplicability_Rulesets(t *testing.T) {
	unit := rulesetUnit(t, "protect")
	desired := setting.RulesetValue(protectRuleset())
	mismatch := func(observed setting.Value) SettingPlan {
		return newSettingPlan(unit, github.Observed(observed), desired)
	}
	notComparable := func(reason github.UnavailableReason) SettingPlan {
		return newSettingPlan(unit, github.Unavailable[setting.Value](reason), desired)
	}
	statusChecksPlan := func(observed, declared setting.Ruleset) SettingPlan {
		return newSettingPlan(unit, github.Observed(setting.RulesetValue(observed)), setting.RulesetValue(declared))
	}
	absentDeclared := func(observed github.SettingObservation[setting.Value]) SettingPlan {
		return newSettingPlan(unit, observed, setting.Absent())
	}
	cases := []struct {
		name string
		in   SettingPlan
		want Applicability
	}{
		{name: "creation of an absent ruleset is applicable", in: mismatch(setting.Absent()), want: Applicable()},
		{name: "ruleset not comparable because the repository is private is not applicable", in: notComparable(github.UnavailablePrivateRepository), want: NotApplicable(NotApplicablePrivateRepository)},
		{name: "ruleset whose existence cannot be determined is not applicable", in: notComparable(github.UnavailableNameUndetermined), want: NotApplicable(NotApplicableNameUndetermined)},
		{name: "ruleset with uninterpreted parts is not applicable", in: notComparable(github.UnavailableUninterpretedParts), want: NotApplicable(NotApplicableUninterpretedParts)},
		{
			name: "value change of an observed ruleset is applicable",
			in:   statusChecksPlan(statusChecksRuleset(false, true), statusChecksRuleset(false, false)),
			want: Applicable(),
		},
		{name: "deletion of an observed ruleset is applicable", in: absentDeclared(github.Observed(setting.RulesetValue(statusChecksRuleset(false, true)))), want: Applicable()},
		{name: "declared absence not comparable because of uninterpreted parts is not applicable", in: absentDeclared(github.Unavailable[setting.Value](github.UnavailableUninterpretedParts)), want: NotApplicable(NotApplicableUninterpretedParts)},
		{name: "declared absence not comparable because the name cannot be determined is not applicable", in: absentDeclared(github.Unavailable[setting.Value](github.UnavailableNameUndetermined)), want: NotApplicable(NotApplicableNameUndetermined)},
		{name: "declared absence not comparable because the repository is private is not applicable", in: absentDeclared(github.Unavailable[setting.Value](github.UnavailablePrivateRepository)), want: NotApplicable(NotApplicablePrivateRepository)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := judgeApplicability(tc.in, []SettingPlan{tc.in}, github.ObservationResult{}); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("judgeApplicability() = %+v, want %+v", got, tc.want)
			}
		})
	}

	// 遷移表にない比較不能を適用可能として黙って通さない
	panics := []struct {
		name string
		in   SettingPlan
	}{
		{name: "ruleset not comparable because actions is disabled panics", in: notComparable(github.UnavailableActionsDisabled)},
		{name: "repository setting not comparable because of uninterpreted parts panics", in: newSettingPlan(setting.Unit{Key: setting.AllowAutoMerge}, github.Unavailable[setting.Value](github.UnavailableUninterpretedParts), setting.BoolValue(true))},
		{name: "repository setting not comparable because a name cannot be compared panics", in: newSettingPlan(setting.Unit{Key: setting.AllowAutoMerge}, github.Unavailable[setting.Value](github.UnavailableNameUndetermined), setting.BoolValue(true))},
	}
	for _, tc := range panics {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("judgeApplicability() did not panic")
				}
			}()
			_ = judgeApplicability(tc.in, []SettingPlan{tc.in}, github.ObservationResult{})
		})
	}
}

//trace:checks FR-006 INV-001
func TestJudgeApplicability_DependabotSecurityUpdates(t *testing.T) {
	updatesPlan := func(observed setting.DependabotSecurityUpdatesState, desired bool) SettingPlan {
		return newSettingPlan(setting.Unit{Key: setting.DependabotSecurityUpdates}, github.Observed(setting.DependabotSecurityUpdatesValue(observed)), setting.DependabotSecurityUpdatesValue(setting.DependabotSecurityUpdatesStateOf(desired)))
	}
	alertsPlan := func(observed, desired bool) SettingPlan {
		return newSettingPlan(setting.Unit{Key: setting.DependabotAlerts}, github.Observed(setting.BoolValue(observed)), setting.BoolValue(desired))
	}
	observation := func(alerts bool, updates setting.DependabotSecurityUpdatesState) github.ObservationResult {
		return github.ObservationResult{Dependabot: &github.Dependabot{Alerts: alerts, SecurityUpdates: updates}}
	}
	cases := []struct {
		name     string
		in       SettingPlan
		alerts   []SettingPlan
		observed github.ObservationResult
		want     Applicability
	}{
		{
			name:     "enabling with alerts enabled and undeclared is applicable",
			in:       updatesPlan(setting.DependabotSecurityUpdatesDisabled, true),
			observed: observation(true, setting.DependabotSecurityUpdatesDisabled),
			want:     Applicable(),
		},
		{
			name:     "disabling with alerts enabled and to be disabled is applicable",
			in:       updatesPlan(setting.DependabotSecurityUpdatesEnabled, false),
			alerts:   []SettingPlan{alertsPlan(true, false)},
			observed: observation(true, setting.DependabotSecurityUpdatesEnabled),
			want:     Applicable(),
		},
		{
			name:     "enabling with alerts disabled and declared true is applicable",
			in:       updatesPlan(setting.DependabotSecurityUpdatesDisabled, true),
			alerts:   []SettingPlan{alertsPlan(false, true)},
			observed: observation(false, setting.DependabotSecurityUpdatesDisabled),
			want:     Applicable(),
		},
		{
			name:     "enabling with alerts disabled and undeclared is not applicable because alerts remain disabled",
			in:       updatesPlan(setting.DependabotSecurityUpdatesDisabled, true),
			observed: observation(false, setting.DependabotSecurityUpdatesDisabled),
			want:     NotApplicable(NotApplicableDependabotAlertsRemainDisabled),
		},
		{
			name:     "disabling with alerts disabled and undeclared is not applicable because GitHub rejects it too",
			in:       updatesPlan(setting.DependabotSecurityUpdatesEnabled, false),
			observed: observation(false, setting.DependabotSecurityUpdatesEnabled),
			want:     NotApplicable(NotApplicableDependabotAlertsRemainDisabled),
		},
		{
			name:     "disabling with alerts disabled and declared false is not applicable",
			in:       updatesPlan(setting.DependabotSecurityUpdatesEnabled, false),
			alerts:   []SettingPlan{alertsPlan(false, false)},
			observed: observation(false, setting.DependabotSecurityUpdatesEnabled),
			want:     NotApplicable(NotApplicableDependabotAlertsRemainDisabled),
		},
		{
			name:     "alerts mismatch is applicable",
			in:       alertsPlan(true, false),
			observed: observation(true, setting.DependabotSecurityUpdatesEnabled),
			want:     Applicable(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			settings := slices.Concat(tc.alerts, []SettingPlan{tc.in})
			got := judgeApplicability(tc.in, settings, tc.observed)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("judgeApplicability() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

//trace:checks FR-006 INV-001
func TestJudgeApplicability_RepositoryPushProtection(t *testing.T) {
	boolPlan := func(key setting.Key, observed, desired bool) SettingPlan {
		return newSettingPlan(setting.Unit{Key: key}, github.Observed(setting.BoolValue(observed)), setting.BoolValue(desired))
	}
	observation := func(enabled, pushProtection bool) github.ObservationResult {
		return github.ObservationResult{SecretScanning: &github.SecretScanning{Enabled: github.Observed(enabled), PushProtection: github.Observed(pushProtection)}}
	}
	secretScanningUnknown := func(pushProtection bool) github.ObservationResult {
		return github.ObservationResult{SecretScanning: &github.SecretScanning{
			Enabled: github.Unavailable[bool](github.UnavailableUnknownValue), PushProtection: github.Observed(pushProtection),
		}}
	}
	unknownSecretScanningPlan := func(desired bool) SettingPlan {
		return newSettingPlan(setting.Unit{Key: setting.SecretScanningEnabled}, github.Unavailable[setting.Value](github.UnavailableUnknownValue), setting.BoolValue(desired))
	}
	cases := []struct {
		name           string
		in             SettingPlan
		secretScanning []SettingPlan
		observed       github.ObservationResult
		want           Applicability
	}{
		{
			name:     "enabling with secret scanning enabled and undeclared is applicable",
			in:       boolPlan(setting.RepositoryPushProtection, false, true),
			observed: observation(true, false),
			want:     Applicable(),
		},
		{
			name:           "enabling with secret scanning disabled and declared true is applicable",
			in:             boolPlan(setting.RepositoryPushProtection, false, true),
			secretScanning: []SettingPlan{boolPlan(setting.SecretScanningEnabled, false, true)},
			observed:       observation(false, false),
			want:           Applicable(),
		},
		{
			name:     "enabling with secret scanning disabled and undeclared is not applicable because GitHub does not apply it",
			in:       boolPlan(setting.RepositoryPushProtection, false, true),
			observed: observation(false, false),
			want:     NotApplicable(NotApplicableSecretScanningRemainsDisabled),
		},
		{
			name:     "disabling with secret scanning disabled and undeclared is applicable because only enabling is not applied",
			in:       boolPlan(setting.RepositoryPushProtection, true, false),
			observed: observation(false, true),
			want:     Applicable(),
		},
		{
			name:     "secret scanning mismatch is applicable",
			in:       boolPlan(setting.SecretScanningEnabled, true, false),
			observed: observation(true, true),
			want:     Applicable(),
		},
		{
			name:     "enabling with secret scanning of an unknown value and undeclared is not applicable because reco cannot confirm it is enabled",
			in:       boolPlan(setting.RepositoryPushProtection, false, true),
			observed: secretScanningUnknown(false),
			want:     NotApplicable(NotApplicableSecretScanningUnknown),
		},
		{
			name:           "enabling with secret scanning of an unknown value and declared true is applicable",
			in:             boolPlan(setting.RepositoryPushProtection, false, true),
			secretScanning: []SettingPlan{unknownSecretScanningPlan(true)},
			observed:       secretScanningUnknown(false),
			want:           Applicable(),
		},
		{
			name:     "disabling with secret scanning of an unknown value is applicable because only enabling is not applied",
			in:       boolPlan(setting.RepositoryPushProtection, true, false),
			observed: secretScanningUnknown(true),
			want:     Applicable(),
		},
		{
			name: "disabling secret scanning with push protection of an unknown value is applicable",
			in:   boolPlan(setting.SecretScanningEnabled, true, false),
			observed: github.ObservationResult{SecretScanning: &github.SecretScanning{
				Enabled: github.Observed(true), PushProtection: github.Unavailable[bool](github.UnavailableUnknownValue),
			}},
			want: Applicable(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			settings := slices.Concat(tc.secretScanning, []SettingPlan{tc.in})
			got := judgeApplicability(tc.in, settings, tc.observed)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("judgeApplicability() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestExpectsActionsEnabled(t *testing.T) {
	plan := func(key setting.Key, desired setting.Value, result ReconciliationResult) SettingPlan {
		return SettingPlan{Unit: setting.Unit{Key: key}, Desired: desired, Result: result}
	}
	cases := []struct {
		name     string
		settings []SettingPlan
		want     bool
	}{
		{name: "general settings alone do not expect actions enabled", settings: []SettingPlan{plan(setting.AllowAutoMerge, setting.BoolValue(true), Mismatch)}, want: false},
		{name: "matched allowed_actions expects actions enabled", settings: []SettingPlan{plan(setting.AllowedActions, setting.AllowedActionsPolicyValue(setting.AllowedActionsAll), Match)}, want: true},
		{name: "enabled declared true expects actions enabled", settings: []SettingPlan{plan(setting.ActionsEnabled, setting.BoolValue(true), Match)}, want: true},
		{name: "enabled declared false does not expect actions enabled", settings: []SettingPlan{plan(setting.ActionsEnabled, setting.BoolValue(false), Mismatch)}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := expectsActionsEnabled(tc.settings); got != tc.want {
				t.Errorf("expectsActionsEnabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

//trace:checks FR-006 INV-001
func TestJudgeApplicability_MergeMethods(t *testing.T) {
	disableSquash := newSettingPlan(setting.Unit{Key: setting.AllowSquashMerge}, github.Observed(setting.BoolValue(true)), setting.BoolValue(false))
	cases := []struct {
		name     string
		observed github.ObservationResult
		want     Applicability
	}{
		{
			name:     "disabling with another merge method left enabled is applicable",
			observed: github.ObservationResult{General: github.General{AllowSquashMerge: true, AllowRebaseMerge: true}},
			want:     Applicable(),
		},
		{
			name:     "disabling the last enabled merge method is not applicable",
			observed: github.ObservationResult{General: github.General{AllowSquashMerge: true}},
			want:     NotApplicable(NotApplicableNoMergeMethodEnabled, setting.AllowMergeCommit, setting.AllowRebaseMerge),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := judgeApplicability(disableSquash, []SettingPlan{disableSquash}, tc.observed)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("judgeApplicability() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestJudgeApplicability_MergeMessages は、不一致のマージメッセージの変更予定を、更新する時点で対応するマージ方式が有効と見込めるときに限り適用可能とし、
// 知らない値で比較不能なマージメッセージを適用不能にすることを確かめる (General の設計の適用可能性と副作用)。
//
//trace:checks FR-006 INV-001
func TestJudgeApplicability_MergeMessages(t *testing.T) {
	current := setting.SquashMergeCommitDefault{Title: setting.SquashMergeCommitTitleCommitOrPRTitle, Message: setting.SquashMergeCommitMessageCommitMessages}
	declared := setting.SquashMergeCommitDefault{Title: setting.SquashMergeCommitTitlePRTitle, Message: setting.SquashMergeCommitMessagePRBody}
	squashPlan := func(observed, desired setting.SquashMergeCommitDefault) SettingPlan {
		return newSettingPlan(setting.Unit{Key: setting.SquashMergeCommit}, github.Observed(setting.SquashMergeCommitDefaultValue(observed)), setting.SquashMergeCommitDefaultValue(desired))
	}
	mergeCommitPlan := newSettingPlan(setting.Unit{Key: setting.MergeCommit},
		github.Observed(setting.MergeCommitDefaultValue(setting.MergeCommitDefault{Title: setting.MergeCommitTitleMergeMessage, Message: setting.MergeCommitMessagePRTitle})),
		setting.MergeCommitDefaultValue(setting.MergeCommitDefault{Title: setting.MergeCommitTitlePRTitle, Message: setting.MergeCommitMessagePRBody}))
	methodPlan := func(key setting.Key, observed, desired bool) SettingPlan {
		return newSettingPlan(setting.Unit{Key: key}, github.Observed(setting.BoolValue(observed)), setting.BoolValue(desired))
	}
	cases := []struct {
		name     string
		in       SettingPlan
		methods  []SettingPlan
		observed github.General
		want     Applicability
	}{
		{
			name:     "match with squash merge disabled has no judgement",
			in:       squashPlan(declared, declared),
			observed: github.General{AllowMergeCommit: true},
			want:     Applicability{},
		},
		{
			name:     "mismatch with squash merge enabled and undeclared is applicable",
			in:       squashPlan(current, declared),
			observed: github.General{AllowSquashMerge: true},
			want:     Applicable(),
		},
		{
			name:     "mismatch with squash merge enabled and to be disabled is applicable",
			in:       squashPlan(current, declared),
			methods:  []SettingPlan{methodPlan(setting.AllowSquashMerge, true, false)},
			observed: github.General{AllowSquashMerge: true, AllowMergeCommit: true},
			want:     Applicable(),
		},
		{
			name:     "mismatch with squash merge disabled and to be enabled is applicable",
			in:       squashPlan(current, declared),
			methods:  []SettingPlan{methodPlan(setting.AllowSquashMerge, false, true)},
			observed: github.General{AllowMergeCommit: true},
			want:     Applicable(),
		},
		{
			name:     "mismatch with squash merge disabled and undeclared is not applicable because the merge method remains disabled",
			in:       squashPlan(current, declared),
			observed: github.General{AllowMergeCommit: true},
			want:     NotApplicable(NotApplicableMergeMethodRemainsDisabled),
		},
		{
			name:     "mismatch with squash merge disabled and declared false is not applicable",
			in:       squashPlan(current, declared),
			methods:  []SettingPlan{methodPlan(setting.AllowSquashMerge, false, false)},
			observed: github.General{AllowMergeCommit: true},
			want:     NotApplicable(NotApplicableMergeMethodRemainsDisabled),
		},
		{
			name:     "merge commit mismatch with merge commit disabled and squash merge enabled is not applicable",
			in:       mergeCommitPlan,
			observed: github.General{AllowSquashMerge: true},
			want:     NotApplicable(NotApplicableMergeMethodRemainsDisabled),
		},
		{
			name:     "not comparable because of an unknown value is not applicable",
			in:       newSettingPlan(setting.Unit{Key: setting.MergeCommit}, github.Unavailable[setting.Value](github.UnavailableUnknownValue), mergeCommitPlan.Desired),
			observed: github.General{AllowMergeCommit: true},
			want:     NotApplicable(NotApplicableUnknownValue),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			settings := slices.Concat(tc.methods, []SettingPlan{tc.in})
			got := judgeApplicability(tc.in, settings, github.ObservationResult{General: tc.observed})
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("judgeApplicability() = %+v, want %+v", got, tc.want)
			}
		})
	}

	t.Run("merge message not comparable for a reason other than an unknown value panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("judgeApplicability() did not panic")
			}
		}()
		in := newSettingPlan(setting.Unit{Key: setting.MergeCommit}, github.Unavailable[setting.Value](github.UnavailablePrivateRepository),
			setting.MergeCommitDefaultValue(setting.MergeCommitDefault{Title: setting.MergeCommitTitlePRTitle, Message: setting.MergeCommitMessagePRBody}))
		judgeApplicability(in, []SettingPlan{in}, github.ObservationResult{})
	})
}

// TestJudgeApplicability_RepositoryFeaturesAndWebCommitSignoff は、リポジトリの機能と Web コミットのサインオフ必須の不一致を適用可能とし、
// 値が返らなかった Web コミットのサインオフ必須を適用不能にすることを確かめる (General の設計の適用可能性と副作用)。
//
//trace:checks FR-006 INV-001
func TestJudgeApplicability_RepositoryFeaturesAndWebCommitSignoff(t *testing.T) {
	cases := []struct {
		name string
		in   SettingPlan
		want Applicability
	}{
		{
			// GitHub Free の非公開リポジトリでは反映されないが、計画の時点では料金プランを観測できない
			name: "has_wiki mismatch is applicable",
			in:   newSettingPlan(setting.Unit{Key: setting.HasWiki}, github.Observed(setting.BoolValue(false)), setting.BoolValue(true)),
			want: Applicable(),
		},
		{
			name: "web_commit_signoff_required mismatch is applicable",
			in:   newSettingPlan(setting.Unit{Key: setting.WebCommitSignoffRequired}, github.Observed(setting.BoolValue(false)), setting.BoolValue(true)),
			want: Applicable(),
		},
		{
			name: "web_commit_signoff_required not comparable because the value was not returned is not applicable",
			in:   newSettingPlan(setting.Unit{Key: setting.WebCommitSignoffRequired}, github.Unavailable[setting.Value](github.UnavailableNotReturned), setting.BoolValue(false)),
			want: NotApplicable(NotApplicableNotReturned),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := judgeApplicability(tc.in, []SettingPlan{tc.in}, github.ObservationResult{})
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("judgeApplicability() = %+v, want %+v", got, tc.want)
			}
		})
	}

	panics := []struct {
		name string
		in   SettingPlan
	}{
		{
			name: "web_commit_signoff_required not comparable for another reason panics",
			in:   newSettingPlan(setting.Unit{Key: setting.WebCommitSignoffRequired}, github.Unavailable[setting.Value](github.UnavailablePublicRepository), setting.BoolValue(true)),
		},
		{
			// リポジトリの機能は必ず値が返り、欠落は観測操作の失敗になる
			name: "repository feature not comparable because the value was not returned panics",
			in:   newSettingPlan(setting.Unit{Key: setting.HasIssues}, github.Unavailable[setting.Value](github.UnavailableNotReturned), setting.BoolValue(true)),
		},
	}
	for _, tc := range panics {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("judgeApplicability() did not panic")
				}
			}()
			judgeApplicability(tc.in, []SettingPlan{tc.in}, github.ObservationResult{})
		})
	}
}

// TestJudgeApplicability_RetentionDays は、保持日数の期待値を計画時に観測した上限と比べ、上限を超える変更予定を、上限を伴って適用不能にすることを確かめる。
//
//trace:checks FR-006
func TestJudgeApplicability_RetentionDays(t *testing.T) {
	cases := []struct {
		name     string
		observed int
		desired  int
		maximum  int
		want     Applicability
	}{
		{name: "desired days equal to the maximum is applicable", observed: 30, desired: 90, maximum: 90, want: Applicable()},
		{name: "desired days over the maximum is not applicable with the maximum", observed: 90, desired: 91, maximum: 90, want: NotApplicableAboveMaximum(90)},
		{name: "observed days over the maximum does not prevent a change within it", observed: 120, desired: 90, maximum: 90, want: Applicable()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := newSettingPlan(setting.Unit{Key: setting.RetentionDays}, github.Observed(setting.RetentionDaysValue(tc.observed)), setting.RetentionDaysValue(tc.desired))
			observed := github.ObservationResult{Retention: &github.Retention{Days: tc.observed, MaximumAllowedDays: tc.maximum}}
			if got := judgeApplicability(plan, []SettingPlan{plan}, observed); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("judgeApplicability() = %+v, want %+v", got, tc.want)
			}
		})
	}

	// 上限があとから下がったときに生じる状況で、reco が送る更新がない
	t.Run("matching days over the maximum is not judged", func(t *testing.T) {
		plan := newSettingPlan(setting.Unit{Key: setting.RetentionDays}, github.Observed(setting.RetentionDaysValue(120)), setting.RetentionDaysValue(120))
		observed := github.ObservationResult{Retention: &github.Retention{Days: 120, MaximumAllowedDays: 90}}
		if got := judgeApplicability(plan, []SettingPlan{plan}, observed); !reflect.DeepEqual(got, Applicability{}) {
			t.Errorf("judgeApplicability() = %+v, want no judgment", got)
		}
	})
}

func TestNotApplicable_PanicsOnInconsistentReason(t *testing.T) {
	cases := []struct {
		name      string
		reason    NotApplicableReason
		unmanaged []setting.Key
	}{
		{name: "missing reason", reason: 0},
		{name: "no merge method enabled without merge methods left disabled", reason: NotApplicableNoMergeMethodEnabled},
		{name: "reason without unmanaged settings given unmanaged settings", reason: NotApplicablePublicRepository, unmanaged: []setting.Key{setting.AllowRebaseMerge}},
		{name: "above the maximum allowed must be built with NotApplicableAboveMaximum", reason: NotApplicableAboveMaximumAllowed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("NotApplicable() did not panic")
				}
			}()
			_ = NotApplicable(tc.reason, tc.unmanaged...)
		})
	}
}

func TestApplicability(t *testing.T) {
	cases := []struct {
		name              string
		in                Applicability
		wantApplicable    bool
		wantNotApplicable bool
		wantReason        NotApplicableReason
		wantUnmanaged     []setting.Key
		wantMaximum       int
	}{
		{name: "no judgment is neither applicable nor not applicable", in: Applicability{}},
		{name: "applicable has no reason", in: Applicable(), wantApplicable: true},
		{name: "not applicable has its reason", in: NotApplicable(NotApplicablePublicRepository), wantNotApplicable: true, wantReason: NotApplicablePublicRepository},
		{
			name: "no merge method enabled has the merge methods left disabled", in: NotApplicable(NotApplicableNoMergeMethodEnabled, setting.AllowRebaseMerge),
			wantNotApplicable: true, wantReason: NotApplicableNoMergeMethodEnabled, wantUnmanaged: []setting.Key{setting.AllowRebaseMerge},
		},
		{
			name: "above the maximum has the maximum", in: NotApplicableAboveMaximum(90),
			wantNotApplicable: true, wantReason: NotApplicableAboveMaximumAllowed, wantMaximum: 90,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.in.IsApplicable(); got != tc.wantApplicable {
				t.Errorf("IsApplicable() = %t, want %t", got, tc.wantApplicable)
			}
			if got := tc.in.IsNotApplicable(); got != tc.wantNotApplicable {
				t.Errorf("IsNotApplicable() = %t, want %t", got, tc.wantNotApplicable)
			}
			if got := tc.in.Reason(); got != tc.wantReason {
				t.Errorf("Reason() = %d, want %d", got, tc.wantReason)
			}
			if got := tc.in.UnmanagedSettings(); !slices.Equal(got, tc.wantUnmanaged) {
				t.Errorf("UnmanagedSettings() = %v, want %v", got, tc.wantUnmanaged)
			}
			if got := tc.in.MaximumAllowed(); got != tc.wantMaximum {
				t.Errorf("MaximumAllowed() = %d, want %d", got, tc.wantMaximum)
			}
		})
	}
}
