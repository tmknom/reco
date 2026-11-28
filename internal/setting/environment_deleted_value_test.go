package setting

import "testing"

//trace:checks FR-007 INV-001
func TestEnvironmentDeletedValueEqual(t *testing.T) {
	base := EnvironmentDeletedValue{
		DeploymentBranchPolicy: DeploymentBranchPolicy{
			Mode: DeploymentBranchPolicySelected,
			Patterns: []DeploymentPattern{
				{Type: DeploymentPatternBranch, Name: "main"},
				{Type: DeploymentPatternTag, Name: "v*"},
			},
		},
		ReviewerSet:       observedReviewerSet(map[Reviewer]string{reviewerUser1: "octocat"}, reviewerUser1),
		PreventSelfReview: true,
		WaitTimer:         30,
		CanAdminsBypass:   true,
	}
	cases := []struct {
		name   string
		modify func(v *EnvironmentDeletedValue)
		want   bool
	}{
		{name: "same items are equal", modify: func(*EnvironmentDeletedValue) {}, want: true},
		{name: "different mode is not equal", modify: func(v *EnvironmentDeletedValue) {
			v.DeploymentBranchPolicy = DeploymentBranchPolicy{Mode: DeploymentBranchPolicyAll}
		}, want: false},
		{name: "patterns in another order are equal", modify: func(v *EnvironmentDeletedValue) {
			v.DeploymentBranchPolicy.Patterns = []DeploymentPattern{
				{Type: DeploymentPatternTag, Name: "v*"},
				{Type: DeploymentPatternBranch, Name: "main"},
			}
		}, want: true},
		{name: "reviewers differing only in login are equal", modify: func(v *EnvironmentDeletedValue) {
			v.ReviewerSet = observedReviewerSet(map[Reviewer]string{reviewerUser1: "renamed"}, reviewerUser1)
		}, want: true},
		{name: "different reviewer ID is not equal", modify: func(v *EnvironmentDeletedValue) {
			v.ReviewerSet = observedReviewerSet(nil, reviewerUser2)
		}, want: false},
		{name: "different prevent self review is not equal", modify: func(v *EnvironmentDeletedValue) {
			v.PreventSelfReview = false
		}, want: false},
		{name: "different wait timer is not equal", modify: func(v *EnvironmentDeletedValue) {
			v.WaitTimer = 0
		}, want: false},
		{name: "different admin bypass is not equal", modify: func(v *EnvironmentDeletedValue) {
			v.CanAdminsBypass = false
		}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			other := base
			tc.modify(&other)
			if got := base.Equal(other); got != tc.want {
				t.Errorf("Equal() = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestEnvironmentDeletedValueEqual_RuleWithoutReviewersAgainstEmptySet(t *testing.T) {
	withoutReviewers := EnvironmentDeletedValue{ReviewerSet: ReviewerSetOfRule(&RequiredReviewers{})}
	noRule := EnvironmentDeletedValue{ReviewerSet: ReviewerSetOf(nil)}
	if withoutReviewers.Equal(noRule) {
		t.Error("Equal() = true for a protection rule without reviewers and no rule, want false")
	}
}
