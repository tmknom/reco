package setting

import (
	"slices"
	"testing"
)

//trace:checks FR-004
func TestParseReviewerType(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		want   ReviewerType
		wantOK bool
	}{
		{name: "User is accepted", input: "User", want: ReviewerUser, wantOK: true},
		{name: "Team is accepted", input: "Team", want: ReviewerTeam, wantOK: true},
		{name: "undefined value is rejected", input: "Bot"},
		{name: "different case is rejected", input: "user"},
		{name: "empty string is rejected", input: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseReviewerType(tc.input)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("ParseReviewerType(%q) = (%q, %t), want (%q, %t)", tc.input, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func reviewers(preventSelfReview bool, rs ...Reviewer) *RequiredReviewers {
	return &RequiredReviewers{PreventSelfReview: preventSelfReview, Reviewers: rs}
}

//trace:checks FR-012 INV-001
func TestEnvironmentProtection_DifferencesTo(t *testing.T) {
	user1 := Reviewer{Type: ReviewerUser, ID: 1}
	user2 := Reviewer{Type: ReviewerUser, ID: 2}
	base := EnvironmentProtection{WaitTimer: 30, RequiredReviewers: reviewers(true, user1, user2), CanAdminsBypass: true}
	cases := []struct {
		name   string
		before EnvironmentProtection
		other  EnvironmentProtection
		want   []EnvironmentProtectionDifference
	}{
		{
			name:   "same protection with reviewers only reordered has no difference",
			before: base,
			other:  EnvironmentProtection{WaitTimer: 30, RequiredReviewers: reviewers(true, user2, user1), CanAdminsBypass: true},
		},
		{
			name:   "different reviewers only is one required_reviewers difference",
			before: base,
			other:  EnvironmentProtection{WaitTimer: 30, RequiredReviewers: reviewers(true, user1), CanAdminsBypass: true},
			want: []EnvironmentProtectionDifference{{
				Item: "required_reviewers", Key: EnvironmentRequiredReviewers, Before: "[User 1, User 2]", After: "[User 1]",
			}},
		},
		{
			name:   "different prevent_self_review only is one prevent_self_review difference with its key",
			before: base,
			other:  EnvironmentProtection{WaitTimer: 30, RequiredReviewers: reviewers(false, user1, user2), CanAdminsBypass: true},
			want: []EnvironmentProtectionDifference{{
				Item: "prevent_self_review", Key: EnvironmentPreventSelfReview, Before: "true", After: "false",
			}},
		},
		{
			name:   "different wait_timer only is one wait_timer difference in minutes with its key",
			before: base,
			other:  EnvironmentProtection{WaitTimer: 1, RequiredReviewers: reviewers(true, user1, user2), CanAdminsBypass: true},
			want: []EnvironmentProtectionDifference{{
				Item: "wait_timer", Key: EnvironmentWaitTimer, Before: "30 minutes", After: "1 minute",
			}},
		},
		{
			name:   "no rule to a rule with prevent_self_review true also differs in prevent_self_review from false",
			before: EnvironmentProtection{},
			other:  EnvironmentProtection{RequiredReviewers: reviewers(true, user1)},
			want: []EnvironmentProtectionDifference{
				{Item: "required_reviewers", Key: EnvironmentRequiredReviewers, Before: "[]", After: "[User 1]"},
				{Item: "prevent_self_review", Key: EnvironmentPreventSelfReview, Before: "false", After: "true"},
			},
		},
		{
			name:   "no rule to a rule with prevent_self_review false is only a required_reviewers difference",
			before: EnvironmentProtection{},
			other:  EnvironmentProtection{RequiredReviewers: reviewers(false, user1)},
			want: []EnvironmentProtectionDifference{{
				Item: "required_reviewers", Key: EnvironmentRequiredReviewers, Before: "[]", After: "[User 1]",
			}},
		},
		{
			name:   "a rule with prevent_self_review true to no rule also differs in prevent_self_review to false",
			before: EnvironmentProtection{RequiredReviewers: reviewers(true, user1)},
			other:  EnvironmentProtection{},
			want: []EnvironmentProtectionDifference{
				{Item: "required_reviewers", Key: EnvironmentRequiredReviewers, Before: "[User 1]", After: "[]"},
				{Item: "prevent_self_review", Key: EnvironmentPreventSelfReview, Before: "true", After: "false"},
			},
		},
		{
			name:   "wait_timer removed is a difference to 0 minutes",
			before: EnvironmentProtection{WaitTimer: 30},
			other:  EnvironmentProtection{},
			want: []EnvironmentProtectionDifference{{
				Item: "wait_timer", Key: EnvironmentWaitTimer, Before: "30 minutes", After: "0 minutes",
			}},
		},
		{
			name:   "all four changed are four differences in the order wait_timer, required_reviewers, prevent_self_review, can_admins_bypass",
			before: base,
			other:  EnvironmentProtection{WaitTimer: 5, RequiredReviewers: reviewers(false, user1)},
			want: []EnvironmentProtectionDifference{
				{Item: "wait_timer", Key: EnvironmentWaitTimer, Before: "30 minutes", After: "5 minutes"},
				{Item: "required_reviewers", Key: EnvironmentRequiredReviewers, Before: "[User 1, User 2]", After: "[User 1]"},
				{Item: "prevent_self_review", Key: EnvironmentPreventSelfReview, Before: "true", After: "false"},
				{Item: "can_admins_bypass", Before: "true", After: "false"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.before.DifferencesTo(tc.other); !slices.Equal(got, tc.want) {
				t.Errorf("DifferencesTo() = %v, want %v", got, tc.want)
			}
		})
	}
}

//trace:checks FR-005
func TestEnvironmentProtection_PreventSelfReview(t *testing.T) {
	user1 := Reviewer{Type: ReviewerUser, ID: 1}
	cases := []struct {
		name string
		rule *RequiredReviewers
		want bool
	}{
		{name: "no rule is false", rule: nil, want: false},
		{name: "a rule with true is true", rule: reviewers(true, user1), want: true},
		{name: "a rule with false is false", rule: reviewers(false, user1), want: false},
		{name: "a rule without reviewers keeps its true", rule: reviewers(true), want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := (EnvironmentProtection{RequiredReviewers: tc.rule}).PreventSelfReview(); got != tc.want {
				t.Errorf("PreventSelfReview() = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestEnvironmentProtection_sameReviewerSet(t *testing.T) {
	user1 := Reviewer{Type: ReviewerUser, ID: 1}
	user2 := Reviewer{Type: ReviewerUser, ID: 2}
	team1 := Reviewer{Type: ReviewerTeam, ID: 1}
	cases := []struct {
		name string
		a, b *RequiredReviewers
		want bool
	}{
		{name: "both none are the same", a: nil, b: nil, want: true},
		{name: "none and empty reviewers differ", a: nil, b: reviewers(false), want: false},
		{name: "empty reviewers and none differ", a: reviewers(false), b: nil, want: false},
		{name: "same reviewers are the same", a: reviewers(true, user1, team1), b: reviewers(true, user1, team1), want: true},
		{name: "reviewers differing only in order are the same", a: reviewers(false, user1, user2), b: reviewers(false, user2, user1), want: true},
		{name: "reviewers of the same ID differing only in order are the same", a: reviewers(false, user1, team1), b: reviewers(false, team1, user1), want: true},
		{name: "different prevent_self_review with the same reviewers is the same", a: reviewers(true, user1), b: reviewers(false, user1), want: true},
		{name: "same ID of a different type differs", a: reviewers(false, user1), b: reviewers(false, team1), want: false},
		{name: "different ID differs", a: reviewers(false, user1), b: reviewers(false, user2), want: false},
		{name: "extra reviewer differs", a: reviewers(false, user1), b: reviewers(false, user1, user2), want: false},
		// 管理設定の担当者の集合の判定と同じく、重複と login を値の違いにしない
		{name: "duplicated reviewer is the same as a single one", a: reviewers(false, user1, user1), b: reviewers(false, user1), want: true},
		{
			name: "reviewers differing only in logins are the same",
			a:    &RequiredReviewers{Reviewers: []Reviewer{user1}, Logins: map[Reviewer]string{user1: "octocat"}},
			b:    &RequiredReviewers{Reviewers: []Reviewer{user1}, Logins: map[Reviewer]string{user1: "hubot"}},
			want: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, b := EnvironmentProtection{RequiredReviewers: tc.a}, EnvironmentProtection{RequiredReviewers: tc.b}
			if got := a.sameReviewerSet(b); got != tc.want {
				t.Errorf("sameReviewerSet() = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestEnvironmentProtection_sameReviewerSetKeepsOrder(t *testing.T) {
	a := EnvironmentProtection{RequiredReviewers: reviewers(false, Reviewer{Type: ReviewerUser, ID: 2}, Reviewer{Type: ReviewerUser, ID: 1})}
	b := EnvironmentProtection{RequiredReviewers: reviewers(false, Reviewer{Type: ReviewerUser, ID: 1}, Reviewer{Type: ReviewerUser, ID: 2})}
	_ = a.sameReviewerSet(b)
	if got := a.RequiredReviewers.Reviewers[0].ID; got != 2 {
		t.Errorf("Reviewers[0].ID = %d, want 2 (the receiver must not be reordered)", got)
	}
	if got := b.RequiredReviewers.Reviewers[0].ID; got != 1 {
		t.Errorf("Reviewers[0].ID = %d, want 1 (the argument must not be reordered)", got)
	}
}

func TestEnvironmentProtection_reviewerSetString(t *testing.T) {
	cases := []struct {
		name string
		r    *RequiredReviewers
		want string
	}{
		{name: "no rule is the empty set", r: nil, want: "[]"},
		{name: "rule without reviewers is marked", r: reviewers(false), want: "[] (protection rule without reviewers)"},
		{
			name: "reviewers keep the observed order",
			r:    reviewers(true, Reviewer{Type: ReviewerUser, ID: 583231}, Reviewer{Type: ReviewerTeam, ID: 42}),
			want: "[User 583231, Team 42]",
		},
		{
			name: "logins are not shown",
			r:    &RequiredReviewers{Reviewers: []Reviewer{{Type: ReviewerUser, ID: 583231}}, Logins: map[Reviewer]string{{Type: ReviewerUser, ID: 583231}: "octocat"}},
			want: "[User 583231]",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := (EnvironmentProtection{RequiredReviewers: tc.r}).reviewerSetString(); got != tc.want {
				t.Errorf("reviewerSetString() = %q, want %q", got, tc.want)
			}
		})
	}
}
