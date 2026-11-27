package setting

import (
	"reflect"
	"slices"
	"testing"
)

func TestValue_String(t *testing.T) {
	cases := []struct {
		value Value
		want  string
	}{
		{value: BoolValue(true), want: "true"},
		{value: PermissionLevelValue(PermissionRead), want: "read"},
		{value: AllowedActionsPolicyValue(AllowedActionsSelected), want: "selected"},
		{value: AccessLevelValue(AccessUser), want: "user"},
		{value: ForkPullRequestApprovalPolicyValue(ForkPullRequestApprovalAllExternalContributors), want: "all_external_contributors"},
		{value: RetentionDaysValue(30), want: "30"},
		{value: WaitTimerValue(0), want: "0 minutes"},
		{value: WaitTimerValue(1), want: "1 minute"},
		{value: WaitTimerValue(30), want: "30 minutes"},
		{value: DependabotSecurityUpdatesValue(DependabotSecurityUpdatesDisabled), want: "false"},
		{value: DependabotSecurityUpdatesValue(DependabotSecurityUpdatesEnabled), want: "true"},
		{value: DependabotSecurityUpdatesValue(DependabotSecurityUpdatesPaused), want: "true (paused by GitHub)"},
		{value: VariableValue("a\nb"), want: `"a\nb"`},
		{value: VariableValue("absent"), want: `"absent"`},
		{value: Absent(), want: "absent"},
		{value: Present(), want: "present"},
		{value: ReviewerSetValue(ReviewerSetOf([]Reviewer{{Type: ReviewerUser, ID: 1}})), want: "[User 1]"},
		{value: SquashMergeCommitDefaultValue(SquashMergeCommitDefault{Title: SquashMergeCommitTitleCommitOrPRTitle, Message: SquashMergeCommitMessageCommitMessages}), want: "title: COMMIT_OR_PR_TITLE, message: COMMIT_MESSAGES"},
		{value: MergeCommitDefaultValue(MergeCommitDefault{Title: MergeCommitTitleMergeMessage, Message: MergeCommitMessagePRTitle}), want: "title: MERGE_MESSAGE, message: PR_TITLE"},
		{
			value: SelectedActionsCriteriaValue(SelectedActionsCriteria{GitHubOwnedAllowed: true, VerifiedAllowed: false, PatternsAllowed: []string{"z/*", "a/*"}}),
			want:  `github_owned_allowed: true, verified_allowed: false, patterns_allowed: ["z/*", "a/*"]`,
		},
		{
			value: PrivateForkPullRequestWorkflowConditionsValue(PrivateForkPullRequestWorkflowConditions{RunWorkflowsFromForkPullRequests: true, SendWriteTokensToWorkflows: false, SendSecretsAndVariables: true, RequireApprovalForForkPullRequestWorkflows: false}),
			want:  "run_workflows_from_fork_pull_requests: true, send_write_tokens_to_workflows: false, send_secrets_and_variables: true, require_approval_for_fork_pr_workflows: false",
		},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.value.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}

//trace:checks FR-005
func TestValue_Equal(t *testing.T) {
	criteria := func(patterns ...string) Value {
		return SelectedActionsCriteriaValue(SelectedActionsCriteria{GitHubOwnedAllowed: true, VerifiedAllowed: true, PatternsAllowed: patterns})
	}
	cases := []struct {
		name  string
		a, b  Value
		equal bool
	}{
		{name: "same booleans are equal", a: BoolValue(true), b: BoolValue(true), equal: true},
		{name: "different booleans differ", a: BoolValue(true), b: BoolValue(false), equal: false},
		{name: "same permission levels are equal", a: PermissionLevelValue(PermissionRead), b: PermissionLevelValue(PermissionRead), equal: true},
		{name: "different permission levels differ", a: PermissionLevelValue(PermissionRead), b: PermissionLevelValue(PermissionWrite), equal: false},
		{name: "same policies are equal", a: AllowedActionsPolicyValue(AllowedActionsAll), b: AllowedActionsPolicyValue(AllowedActionsAll), equal: true},
		{name: "different policies differ", a: AllowedActionsPolicyValue(AllowedActionsAll), b: AllowedActionsPolicyValue(AllowedActionsSelected), equal: false},
		{name: "same access levels are equal", a: AccessLevelValue(AccessNone), b: AccessLevelValue(AccessNone), equal: true},
		{name: "different access levels differ", a: AccessLevelValue(AccessNone), b: AccessLevelValue(AccessUser), equal: false},
		{name: "same approval policies are equal", a: ForkPullRequestApprovalPolicyValue(ForkPullRequestApprovalFirstTimeContributors), b: ForkPullRequestApprovalPolicyValue(ForkPullRequestApprovalFirstTimeContributors), equal: true},
		{name: "different approval policies differ", a: ForkPullRequestApprovalPolicyValue(ForkPullRequestApprovalFirstTimeContributors), b: ForkPullRequestApprovalPolicyValue(ForkPullRequestApprovalAllExternalContributors), equal: false},
		{name: "same workflow conditions are equal", a: PrivateForkPullRequestWorkflowConditionsValue(PrivateForkPullRequestWorkflowConditions{RunWorkflowsFromForkPullRequests: true, RequireApprovalForForkPullRequestWorkflows: true}), b: PrivateForkPullRequestWorkflowConditionsValue(PrivateForkPullRequestWorkflowConditions{RunWorkflowsFromForkPullRequests: true, RequireApprovalForForkPullRequestWorkflows: true}), equal: true},
		{name: "workflow conditions differing only in approval differ", a: PrivateForkPullRequestWorkflowConditionsValue(PrivateForkPullRequestWorkflowConditions{RunWorkflowsFromForkPullRequests: true, RequireApprovalForForkPullRequestWorkflows: true}), b: PrivateForkPullRequestWorkflowConditionsValue(PrivateForkPullRequestWorkflowConditions{RunWorkflowsFromForkPullRequests: true}), equal: false},
		{name: "same retention days are equal", a: RetentionDaysValue(30), b: RetentionDaysValue(30), equal: true},
		{name: "different retention days differ", a: RetentionDaysValue(30), b: RetentionDaysValue(90), equal: false},
		{name: "same wait timers are equal", a: WaitTimerValue(30), b: WaitTimerValue(30), equal: true},
		{name: "different wait timers differ", a: WaitTimerValue(30), b: WaitTimerValue(31), equal: false},
		{name: "criteria differing only in pattern order are equal", a: criteria("a/*", "b/*"), b: criteria("b/*", "a/*"), equal: true},
		{name: "criteria with a different pattern differ", a: criteria("a/*"), b: criteria("b/*"), equal: false},
		{name: "values of different kinds differ", a: BoolValue(false), b: SelectedActionsCriteriaValue(SelectedActionsCriteria{}), equal: false},
		{name: "paused security updates equal enabled ones", a: DependabotSecurityUpdatesValue(DependabotSecurityUpdatesPaused), b: DependabotSecurityUpdatesValue(DependabotSecurityUpdatesEnabled), equal: true},
		{name: "enabled security updates differ from disabled ones", a: DependabotSecurityUpdatesValue(DependabotSecurityUpdatesEnabled), b: DependabotSecurityUpdatesValue(DependabotSecurityUpdatesDisabled), equal: false},
		{name: "paused security updates differ from disabled ones", a: DependabotSecurityUpdatesValue(DependabotSecurityUpdatesPaused), b: DependabotSecurityUpdatesValue(DependabotSecurityUpdatesDisabled), equal: false},
		{name: "security updates differ from a boolean of the same enablement", a: DependabotSecurityUpdatesValue(DependabotSecurityUpdatesDisabled), b: BoolValue(false), equal: false},
		{name: "same variable values are equal", a: VariableValue("1.27"), b: VariableValue("1.27"), equal: true},
		{name: "variable values differing only in a trailing newline differ", a: VariableValue("1.27"), b: VariableValue("1.27\n"), equal: false},
		{name: "empty variable value differs from an absent variable", a: VariableValue(""), b: Absent(), equal: false},
		{name: "absences are equal", a: Absent(), b: Absent(), equal: true},
		{name: "presence differs from an absence", a: Present(), b: Absent(), equal: false},
		{name: "variable value differs from a boolean of the same spelling", a: VariableValue("true"), b: BoolValue(true), equal: false},
		{name: "rulesets of different enforcement differ", a: RulesetValue(newTestRuleset()), b: RulesetValue(newTestRuleset(withEnforcement(RulesetEnforcementDisabled))), equal: false},
		{name: "reviewer sets differing only in order are equal", a: ReviewerSetValue(ReviewerSetOf([]Reviewer{{Type: ReviewerUser, ID: 1}, {Type: ReviewerUser, ID: 2}})), b: ReviewerSetValue(ReviewerSetOf([]Reviewer{{Type: ReviewerUser, ID: 2}, {Type: ReviewerUser, ID: 1}})), equal: true},
		{name: "reviewer sets with a different reviewer differ", a: ReviewerSetValue(ReviewerSetOf([]Reviewer{{Type: ReviewerUser, ID: 1}})), b: ReviewerSetValue(ReviewerSetOf([]Reviewer{{Type: ReviewerUser, ID: 2}})), equal: false},
		{name: "ruleset differs from an absence", a: RulesetValue(newTestRuleset()), b: Absent(), equal: false},
		{name: "same squash merge commit defaults are equal", a: squashMergeCommitValue(SquashMergeCommitTitlePRTitle, SquashMergeCommitMessagePRBody), b: squashMergeCommitValue(SquashMergeCommitTitlePRTitle, SquashMergeCommitMessagePRBody), equal: true},
		{name: "squash merge commit defaults differing only in title differ", a: squashMergeCommitValue(SquashMergeCommitTitlePRTitle, SquashMergeCommitMessageCommitMessages), b: squashMergeCommitValue(SquashMergeCommitTitleCommitOrPRTitle, SquashMergeCommitMessageCommitMessages), equal: false},
		{name: "squash merge commit defaults differing only in message differ", a: squashMergeCommitValue(SquashMergeCommitTitlePRTitle, SquashMergeCommitMessagePRBody), b: squashMergeCommitValue(SquashMergeCommitTitlePRTitle, SquashMergeCommitMessageBlank), equal: false},
		{name: "same merge commit defaults are equal", a: mergeCommitValue(MergeCommitTitleMergeMessage, MergeCommitMessagePRTitle), b: mergeCommitValue(MergeCommitTitleMergeMessage, MergeCommitMessagePRTitle), equal: true},
		{name: "merge commit defaults differing only in title differ", a: mergeCommitValue(MergeCommitTitlePRTitle, MergeCommitMessagePRBody), b: mergeCommitValue(MergeCommitTitleMergeMessage, MergeCommitMessagePRBody), equal: false},
		{name: "merge commit defaults differing only in message differ", a: mergeCommitValue(MergeCommitTitlePRTitle, MergeCommitMessagePRBody), b: mergeCommitValue(MergeCommitTitlePRTitle, MergeCommitMessageBlank), equal: false},
		{name: "squash merge and merge commit defaults of the same spelling differ", a: squashMergeCommitValue(SquashMergeCommitTitlePRTitle, SquashMergeCommitMessagePRBody), b: mergeCommitValue(MergeCommitTitlePRTitle, MergeCommitMessagePRBody), equal: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.Equal(tc.b); got != tc.equal {
				t.Errorf("Equal() = %v, want %v", got, tc.equal)
			}
		})
	}
}

func TestValue_SquashMergeCommitDefault(t *testing.T) {
	want := SquashMergeCommitDefault{Title: SquashMergeCommitTitleCommitOrPRTitle, Message: SquashMergeCommitMessageCommitMessages}
	if got := SquashMergeCommitDefaultValue(want).SquashMergeCommitDefault(); got != want {
		t.Errorf("SquashMergeCommitDefault() = %v, want %v", got, want)
	}
}

func TestValue_SquashMergeCommitDefaultOfMergeCommitDefaultPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("SquashMergeCommitDefault() did not panic for a merge commit default")
		}
	}()
	_ = mergeCommitValue(MergeCommitTitlePRTitle, MergeCommitMessagePRBody).SquashMergeCommitDefault()
}

func TestValue_MergeCommitDefault(t *testing.T) {
	want := MergeCommitDefault{Title: MergeCommitTitleMergeMessage, Message: MergeCommitMessagePRTitle}
	if got := MergeCommitDefaultValue(want).MergeCommitDefault(); got != want {
		t.Errorf("MergeCommitDefault() = %v, want %v", got, want)
	}
}

func TestValue_MergeCommitDefaultOfSquashMergeCommitDefaultPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("MergeCommitDefault() did not panic for a squash merge commit default")
		}
	}()
	_ = squashMergeCommitValue(SquashMergeCommitTitlePRTitle, SquashMergeCommitMessagePRBody).MergeCommitDefault()
}

func TestValue_EqualWithZeroValuePanics(t *testing.T) {
	cases := []struct {
		name string
		a, b Value
	}{
		{name: "zero value on the receiver side", a: Value{}, b: SelectedActionsCriteriaValue(SelectedActionsCriteria{})},
		{name: "zero value on the argument side", a: SelectedActionsCriteriaValue(SelectedActionsCriteria{}), b: Value{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("Equal() did not panic for the zero value")
				}
			}()
			_ = tc.a.Equal(tc.b)
		})
	}
}

// Value は許可パターンのスライスを持つため == で比べられない。比較可能な表現に戻ると、
// 順序を無視しない == が Equal の代わりに黙って使えるようになる。
func TestValue_IsNotComparable(t *testing.T) {
	if reflect.TypeFor[Value]().Comparable() {
		t.Error("Value is comparable, want == to be a compile error")
	}
}

func TestValue_StringOfZeroValuePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("String() did not panic for the zero value")
		}
	}()
	_ = Value{}.String()
}

// Ruleset の値は1行の表記を持たず、変更計画の表示は値の差から組み立てるため、String で表示しようとするのは不変条件の破れになる。
func TestValue_StringOfRulesetPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("String() did not panic for a ruleset value")
		}
	}()
	_ = RulesetValue(newTestRuleset()).String()
}

func TestValue_Bool(t *testing.T) {
	for _, want := range []bool{true, false} {
		if got := BoolValue(want).Bool(); got != want {
			t.Errorf("BoolValue(%v).Bool() = %v, want %v", want, got, want)
		}
	}
}

func TestValue_BoolOfPermissionLevelPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Bool() did not panic for a permission level value")
		}
	}()
	_ = PermissionLevelValue(PermissionWrite).Bool()
}

func TestValue_PermissionLevel(t *testing.T) {
	for _, want := range []PermissionLevel{PermissionRead, PermissionWrite} {
		if got := PermissionLevelValue(want).PermissionLevel(); got != want {
			t.Errorf("PermissionLevelValue(%q).PermissionLevel() = %q, want %q", want, got, want)
		}
	}
}

func TestValue_PermissionLevelOfBoolPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("PermissionLevel() did not panic for a boolean value")
		}
	}()
	_ = BoolValue(true).PermissionLevel()
}

func TestValue_AllowedActionsPolicy(t *testing.T) {
	for _, want := range []AllowedActionsPolicy{AllowedActionsAll, AllowedActionsSelected} {
		if got := AllowedActionsPolicyValue(want).AllowedActionsPolicy(); got != want {
			t.Errorf("AllowedActionsPolicyValue(%q).AllowedActionsPolicy() = %q, want %q", want, got, want)
		}
	}
}

func TestValue_AllowedActionsPolicyOfBoolPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("AllowedActionsPolicy() did not panic for a boolean value")
		}
	}()
	_ = BoolValue(true).AllowedActionsPolicy()
}

func TestValue_AccessLevel(t *testing.T) {
	for _, want := range []AccessLevel{AccessNone, AccessUser, AccessOrganization} {
		if got := AccessLevelValue(want).AccessLevel(); got != want {
			t.Errorf("AccessLevelValue(%q).AccessLevel() = %q, want %q", want, got, want)
		}
	}
}

func TestValue_AccessLevelOfBoolPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("AccessLevel() did not panic for a boolean value")
		}
	}()
	_ = BoolValue(true).AccessLevel()
}

func TestValue_ForkPullRequestApprovalPolicy(t *testing.T) {
	for _, want := range []ForkPullRequestApprovalPolicy{ForkPullRequestApprovalFirstTimeContributorsNewToGitHub, ForkPullRequestApprovalFirstTimeContributors, ForkPullRequestApprovalAllExternalContributors} {
		if got := ForkPullRequestApprovalPolicyValue(want).ForkPullRequestApprovalPolicy(); got != want {
			t.Errorf("ForkPullRequestApprovalPolicyValue(%q).ForkPullRequestApprovalPolicy() = %q, want %q", want, got, want)
		}
	}
}

func TestValue_ForkPullRequestApprovalPolicyOfBoolPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("ForkPullRequestApprovalPolicy() did not panic for a boolean value")
		}
	}()
	_ = BoolValue(true).ForkPullRequestApprovalPolicy()
}

func TestValue_RetentionDays(t *testing.T) {
	if got := RetentionDaysValue(30).RetentionDays(); got != 30 {
		t.Errorf("RetentionDaysValue(30).RetentionDays() = %d, want 30", got)
	}
}

func TestValue_RetentionDaysOfBoolPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("RetentionDays() did not panic for a boolean value")
		}
	}()
	_ = BoolValue(true).RetentionDays()
}

func TestValue_WaitTimer(t *testing.T) {
	if got := WaitTimerValue(30).WaitTimer(); got != 30 {
		t.Errorf("WaitTimerValue(30).WaitTimer() = %d, want 30", got)
	}
}

func TestValue_WaitTimerOfRetentionDaysPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("WaitTimer() did not panic for retention days")
		}
	}()
	_ = RetentionDaysValue(30).WaitTimer()
}

func TestValue_PrivateForkPullRequestWorkflowConditions(t *testing.T) {
	want := PrivateForkPullRequestWorkflowConditions{RunWorkflowsFromForkPullRequests: true, SendWriteTokensToWorkflows: true, SendSecretsAndVariables: false, RequireApprovalForForkPullRequestWorkflows: true}
	if got := PrivateForkPullRequestWorkflowConditionsValue(want).PrivateForkPullRequestWorkflowConditions(); got != want {
		t.Errorf("PrivateForkPullRequestWorkflowConditions() = %+v, want %+v", got, want)
	}
}

func TestValue_PrivateForkPullRequestWorkflowConditionsOfBoolPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("PrivateForkPullRequestWorkflowConditions() did not panic for a boolean value")
		}
	}()
	_ = BoolValue(true).PrivateForkPullRequestWorkflowConditions()
}

func TestValue_DependabotSecurityUpdates(t *testing.T) {
	v := DependabotSecurityUpdatesValue(DependabotSecurityUpdatesPaused)
	if got := v.DependabotSecurityUpdates(); got != DependabotSecurityUpdatesPaused {
		t.Errorf("DependabotSecurityUpdates() = %v, want %v", got, DependabotSecurityUpdatesPaused)
	}
}

func TestDependabotSecurityUpdatesValue_UndefinedStatePanics(t *testing.T) {
	for _, state := range []DependabotSecurityUpdatesState{0, DependabotSecurityUpdatesPaused + 1} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("DependabotSecurityUpdatesValue(%d) did not panic", state)
				}
			}()
			DependabotSecurityUpdatesValue(state)
		}()
	}
}

func TestValue_DependabotSecurityUpdatesOfBoolPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("DependabotSecurityUpdates() did not panic for a boolean value")
		}
	}()
	BoolValue(true).DependabotSecurityUpdates()
}

func TestValue_VariableValue(t *testing.T) {
	v := VariableValue("a\"b\n")
	if got := v.VariableValue(); got != "a\"b\n" {
		t.Errorf("VariableValue() = %q, want %q", got, "a\"b\n")
	}
}

func TestValue_VariableValueOfAbsencePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("VariableValue() did not panic for an absent variable")
		}
	}()
	Absent().VariableValue()
}

func TestValue_SelectedActionsCriteria(t *testing.T) {
	want := SelectedActionsCriteria{GitHubOwnedAllowed: true, VerifiedAllowed: false, PatternsAllowed: []string{"z/*", "a/*"}}
	if got := SelectedActionsCriteriaValue(want).SelectedActionsCriteria(); !reflect.DeepEqual(got, want) {
		t.Errorf("SelectedActionsCriteria() = %+v, want %+v", got, want)
	}
}

func TestValue_SelectedActionsCriteriaOfBoolPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("SelectedActionsCriteria() did not panic for a boolean value")
		}
	}()
	_ = BoolValue(true).SelectedActionsCriteria()
}

func TestValue_SelectedActionsCriteriaIsolatesPatterns(t *testing.T) {
	want := []string{"a/*", "b/*"}

	t.Run("writing to the slice given to the constructor does not change the value", func(t *testing.T) {
		given := slices.Clone(want)
		v := SelectedActionsCriteriaValue(SelectedActionsCriteria{PatternsAllowed: given})
		given[0] = "changed"
		if got := v.SelectedActionsCriteria().PatternsAllowed; !slices.Equal(got, want) {
			t.Errorf("PatternsAllowed = %q, want %q", got, want)
		}
	})

	t.Run("writing to the slice returned by the accessor does not change the value", func(t *testing.T) {
		v := SelectedActionsCriteriaValue(SelectedActionsCriteria{PatternsAllowed: slices.Clone(want)})
		v.SelectedActionsCriteria().PatternsAllowed[0] = "changed"
		if got := v.SelectedActionsCriteria().PatternsAllowed; !slices.Equal(got, want) {
			t.Errorf("PatternsAllowed = %q, want %q", got, want)
		}
	})
}

//trace:checks FR-005
func TestDeploymentBranchPolicyValue_Equal(t *testing.T) {
	selected := func(patterns ...DeploymentPattern) Value {
		return DeploymentBranchPolicyValue(DeploymentBranchPolicy{Mode: DeploymentBranchPolicySelected, Patterns: patterns})
	}
	mode := func(m DeploymentBranchPolicyMode) Value {
		return DeploymentBranchPolicyValue(DeploymentBranchPolicy{Mode: m})
	}
	main := DeploymentPattern{Type: DeploymentPatternBranch, Name: "main"}
	tagMain := DeploymentPattern{Type: DeploymentPatternTag, Name: "main"}
	release := DeploymentPattern{Type: DeploymentPatternTag, Name: "v*"}
	cases := []struct {
		name  string
		a, b  Value
		equal bool
	}{
		{name: "same mode is equal", a: mode(DeploymentBranchPolicyProtectedBranches), b: mode(DeploymentBranchPolicyProtectedBranches), equal: true},
		{name: "different modes differ", a: mode(DeploymentBranchPolicyAll), b: mode(DeploymentBranchPolicyProtectedBranches), equal: false},
		{name: "patterns differing only in order are equal", a: selected(main, release), b: selected(release, main), equal: true},
		{name: "different patterns differ", a: selected(main), b: selected(release), equal: false},
		{name: "same name of a different type differs", a: selected(main), b: selected(tagMain), equal: false},
		{name: "names differing only in letter case differ", a: selected(main), b: selected(DeploymentPattern{Type: DeploymentPatternBranch, Name: "MAIN"}), equal: false},
		{name: "different kind of value differs", a: mode(DeploymentBranchPolicyAll), b: AllowedActionsPolicyValue(AllowedActionsAll), equal: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.Equal(tc.b); got != tc.equal {
				t.Errorf("Equal() = %t, want %t", got, tc.equal)
			}
		})
	}
}

//trace:checks FR-007
func TestDeploymentBranchPolicyValue_String(t *testing.T) {
	cases := []struct {
		policy DeploymentBranchPolicy
		want   string
	}{
		{policy: DeploymentBranchPolicy{Mode: DeploymentBranchPolicyAll}, want: "all"},
		{policy: DeploymentBranchPolicy{Mode: DeploymentBranchPolicyProtectedBranches}, want: "protected_branches"},
		{
			policy: DeploymentBranchPolicy{Mode: DeploymentBranchPolicySelected, Patterns: []DeploymentPattern{{Type: DeploymentPatternBranch, Name: "main"}, {Type: DeploymentPatternTag, Name: "v*"}}},
			want:   `selected [branch "main", tag "v*"]`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := DeploymentBranchPolicyValue(tc.policy).String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDeploymentBranchPolicyValue_KeepsPatternsFromLaterChanges(t *testing.T) {
	patterns := []DeploymentPattern{{Type: DeploymentPatternBranch, Name: "main"}}
	v := DeploymentBranchPolicyValue(DeploymentBranchPolicy{Mode: DeploymentBranchPolicySelected, Patterns: patterns})
	patterns[0].Name = "changed"
	if got := v.String(); got != `selected [branch "main"]` {
		t.Errorf("String() = %q, want the patterns at construction", got)
	}
}

func TestValue_DeploymentBranchPolicy(t *testing.T) {
	want := DeploymentBranchPolicy{Mode: DeploymentBranchPolicySelected, Patterns: []DeploymentPattern{{Type: DeploymentPatternBranch, Name: "main"}, {Type: DeploymentPatternTag, Name: "v*"}}}
	if got := DeploymentBranchPolicyValue(want).DeploymentBranchPolicy(); !reflect.DeepEqual(got, want) {
		t.Errorf("DeploymentBranchPolicy() = %+v, want %+v", got, want)
	}
}

func TestValue_DeploymentBranchPolicyOfBoolPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("DeploymentBranchPolicy() did not panic for a boolean value")
		}
	}()
	_ = BoolValue(true).DeploymentBranchPolicy()
}

func TestValue_DeploymentBranchPolicyIsolatesPatterns(t *testing.T) {
	v := DeploymentBranchPolicyValue(DeploymentBranchPolicy{Mode: DeploymentBranchPolicySelected, Patterns: []DeploymentPattern{{Type: DeploymentPatternBranch, Name: "main"}}})
	v.DeploymentBranchPolicy().Patterns[0].Name = "changed"
	if got := v.DeploymentBranchPolicy().Patterns[0].Name; got != "main" {
		t.Errorf("Patterns[0].Name = %q, want %q", got, "main")
	}
}

func TestValue_RulesetOfBoolPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Ruleset() did not panic for a boolean value")
		}
	}()
	_ = BoolValue(true).Ruleset()
}

func TestRulesetValue_IsolatesSlices(t *testing.T) {
	newRuleset := func() Ruleset {
		return newTestRuleset(withRefName([]string{"a"}, []string{"x"}))
	}
	changes := []struct {
		name   string
		change func(*Ruleset)
	}{
		{name: "include", change: func(r *Ruleset) { r.Conditions.RefName.Include[0] = "changed" }},
		{name: "exclude", change: func(r *Ruleset) { r.Conditions.RefName.Exclude[0] = "changed" }},
		{name: "ref_name", change: func(r *Ruleset) { r.Conditions.RefName.Include = []string{"changed"} }},
		{name: "bypass actors", change: func(r *Ruleset) { r.BypassActors[0].ActorID = 2 }},
		{name: "rules", change: func(r *Ruleset) { r.Rules[0] = PullRequestRule(PullRequestParameters{}) }},
	}
	for _, c := range changes {
		t.Run("writing to the "+c.name+" given to the constructor does not change the value", func(t *testing.T) {
			given := newRuleset()
			v := RulesetValue(given)
			c.change(&given)
			if got := v.Ruleset(); !reflect.DeepEqual(got, newRuleset()) {
				t.Errorf("Ruleset() = %+v, want the value at construction", got)
			}
		})
		t.Run("writing to the "+c.name+" returned by the accessor does not change the value", func(t *testing.T) {
			v := RulesetValue(newRuleset())
			returned := v.Ruleset()
			c.change(&returned)
			if got := v.Ruleset(); !reflect.DeepEqual(got, newRuleset()) {
				t.Errorf("Ruleset() = %+v, want the value at construction", got)
			}
		})
	}
}

func newOIDCSubjectCustomization() OIDCSubjectCustomization {
	immutable := true
	return OIDCSubjectCustomization{UseDefault: false, IncludeClaimKeys: []string{"repo", "context"}, UseImmutableSubject: &immutable}
}

// Value.Equal が subject のカスタマイズの同じ値の判定に委ねることを確かめる。判定の規則は OIDCSubjectCustomization.equal のテストが持つ。
func TestOIDCSubjectCustomizationValue_Equal(t *testing.T) {
	reordered := newOIDCSubjectCustomization()
	reordered.IncludeClaimKeys = []string{"context", "repo"}
	if OIDCSubjectCustomizationValue(newOIDCSubjectCustomization()).Equal(OIDCSubjectCustomizationValue(reordered)) {
		t.Error("Equal() = true for claim keys differing only in order, want false")
	}
	if !OIDCSubjectCustomizationValue(newOIDCSubjectCustomization()).Equal(OIDCSubjectCustomizationValue(newOIDCSubjectCustomization())) {
		t.Error("Equal() = false for the same customization, want true")
	}
}

func TestOIDCSubjectCustomizationValue_String(t *testing.T) {
	want := `use_default: false, include_claim_keys: ["repo", "context"], use_immutable_subject: true`
	if got := OIDCSubjectCustomizationValue(newOIDCSubjectCustomization()).String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestValue_OIDCSubjectCustomizationOfBoolPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("OIDCSubjectCustomization() did not panic for a boolean value")
		}
	}()
	_ = BoolValue(true).OIDCSubjectCustomization()
}

func TestValue_OIDCSubjectCustomizationIsolatesItems(t *testing.T) {
	changes := []struct {
		name   string
		change func(*OIDCSubjectCustomization)
	}{
		{name: "claim keys", change: func(c *OIDCSubjectCustomization) { c.IncludeClaimKeys[0] = "changed" }},
		{name: "immutable subject", change: func(c *OIDCSubjectCustomization) { *c.UseImmutableSubject = false }},
	}
	for _, c := range changes {
		t.Run("writing to the "+c.name+" given to the constructor does not change the value", func(t *testing.T) {
			given := newOIDCSubjectCustomization()
			v := OIDCSubjectCustomizationValue(given)
			c.change(&given)
			if got := v.OIDCSubjectCustomization(); !reflect.DeepEqual(got, newOIDCSubjectCustomization()) {
				t.Errorf("OIDCSubjectCustomization() = %+v, want the value at construction", got)
			}
		})
		t.Run("writing to the "+c.name+" returned by the accessor does not change the value", func(t *testing.T) {
			v := OIDCSubjectCustomizationValue(newOIDCSubjectCustomization())
			returned := v.OIDCSubjectCustomization()
			c.change(&returned)
			if got := v.OIDCSubjectCustomization(); !reflect.DeepEqual(got, newOIDCSubjectCustomization()) {
				t.Errorf("OIDCSubjectCustomization() = %+v, want the value at construction", got)
			}
		})
	}
}

func TestValue_WithItemsDeclaredIn(t *testing.T) {
	observed := OIDCSubjectCustomizationValue(newOIDCSubjectCustomization())
	omitted := newOIDCSubjectCustomization()
	omitted.UseImmutableSubject = nil

	t.Run("observed value drops the item the desired value omits", func(t *testing.T) {
		got := observed.WithItemsDeclaredIn(OIDCSubjectCustomizationValue(OIDCSubjectCustomization{UseDefault: true}))
		if got.OIDCSubjectCustomization().UseImmutableSubject != nil {
			t.Errorf("UseImmutableSubject = %v, want nil", *got.OIDCSubjectCustomization().UseImmutableSubject)
		}
		if want := OIDCSubjectCustomizationValue(omitted); got.String() != want.String() {
			t.Errorf("WithItemsDeclaredIn() = %s, want %s", got.String(), want.String())
		}
	})

	t.Run("observed value keeps the item the desired value declares", func(t *testing.T) {
		got := observed.WithItemsDeclaredIn(OIDCSubjectCustomizationValue(newOIDCSubjectCustomization()))
		if got.String() != observed.String() {
			t.Errorf("WithItemsDeclaredIn() = %s, want %s", got.String(), observed.String())
		}
	})

	t.Run("dropping the item leaves the original value unchanged", func(t *testing.T) {
		_ = observed.WithItemsDeclaredIn(OIDCSubjectCustomizationValue(omitted))
		if got := observed.OIDCSubjectCustomization(); !reflect.DeepEqual(got, newOIDCSubjectCustomization()) {
			t.Errorf("OIDCSubjectCustomization() = %+v, want the value before dropping", got)
		}
	})

	t.Run("values of other kinds are returned unchanged", func(t *testing.T) {
		got := BoolValue(true).WithItemsDeclaredIn(BoolValue(false))
		if !got.Equal(BoolValue(true)) {
			t.Errorf("WithItemsDeclaredIn() = %s, want true", got.String())
		}
	})

	// 値の全体を置き換える更新が GitHub の決める値に変えるため、変更前提で比べる
	t.Run("ruleset parameter omitted by the declaration is kept in the observed value", func(t *testing.T) {
		observed := RulesetValue(newTestRuleset(withRules(statusChecksRule(false, new(true)))))
		got := observed.WithItemsDeclaredIn(RulesetValue(newTestRuleset(withRules(statusChecksRule(false, nil)))))
		if p := got.Ruleset().Rules[0].RequiredStatusChecks().DoNotEnforceOnCreate; p == nil || !*p {
			t.Errorf("WithItemsDeclaredIn() do_not_enforce_on_create = %v, want true kept", p)
		}
	})
}

func TestValue_ReviewerSet(t *testing.T) {
	set := ReviewerSetOf([]Reviewer{{Type: ReviewerUser, ID: 1}})
	if got := ReviewerSetValue(set).ReviewerSet(); !got.equal(set) {
		t.Errorf("ReviewerSet() = %s, want %s", got.String(), set.String())
	}
}

func TestValue_ReviewerSetOfBoolPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("ReviewerSet() did not panic for a boolean value")
		}
	}()
	_ = BoolValue(true).ReviewerSet()
}

func squashMergeCommitValue(title SquashMergeCommitTitle, message SquashMergeCommitMessage) Value {
	return SquashMergeCommitDefaultValue(SquashMergeCommitDefault{Title: title, Message: message})
}

func mergeCommitValue(title MergeCommitTitle, message MergeCommitMessage) Value {
	return MergeCommitDefaultValue(MergeCommitDefault{Title: title, Message: message})
}
