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

//trace:checks FR-012
func TestEnvironmentProtection_DifferencesTo(t *testing.T) {
	user1 := Reviewer{Type: ReviewerUser, ID: 1}
	user2 := Reviewer{Type: ReviewerUser, ID: 2}
	base := EnvironmentProtection{WaitTimer: new(30), RequiredReviewers: reviewers(true, user1, user2), CanAdminsBypass: true}
	cases := []struct {
		name  string
		other EnvironmentProtection
		want  []EnvironmentProtectionDifference
	}{
		{
			name:  "same protection with reviewers only reordered has no difference",
			other: EnvironmentProtection{WaitTimer: new(30), RequiredReviewers: reviewers(true, user2, user1), CanAdminsBypass: true},
		},
		{
			name:  "changed required_reviewers is one difference",
			other: EnvironmentProtection{WaitTimer: new(30), RequiredReviewers: reviewers(false, user1, user2), CanAdminsBypass: true},
			want: []EnvironmentProtectionDifference{{
				Item: "required_reviewers", Before: "reviewers: [User 1, User 2], prevent_self_review: true", After: "reviewers: [User 1, User 2], prevent_self_review: false",
			}},
		},
		{
			name:  "all three changed are three differences in the order wait_timer, required_reviewers, can_admins_bypass",
			other: EnvironmentProtection{WaitTimer: new(5)},
			want: []EnvironmentProtectionDifference{
				{Item: "wait_timer", Before: "30", After: "5"},
				{Item: "required_reviewers", Before: "reviewers: [User 1, User 2], prevent_self_review: true", After: "none"},
				{Item: "can_admins_bypass", Before: "true", After: "false"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := base.DifferencesTo(tc.other); !slices.Equal(got, tc.want) {
				t.Errorf("DifferencesTo() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEnvironmentProtection_sameWaitTimer(t *testing.T) {
	cases := []struct {
		name string
		a, b *int
		want bool
	}{
		{name: "both none are the same", a: nil, b: nil, want: true},
		{name: "same minutes in different pointers are the same", a: new(30), b: new(30), want: true},
		{name: "different minutes differ", a: new(30), b: new(31), want: false},
		{name: "none and zero minutes differ", a: nil, b: new(0), want: false},
		{name: "zero minutes and none differ", a: new(0), b: nil, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, b := EnvironmentProtection{WaitTimer: tc.a}, EnvironmentProtection{WaitTimer: tc.b}
			if got := a.sameWaitTimer(b); got != tc.want {
				t.Errorf("sameWaitTimer() = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestEnvironmentProtection_sameRequiredReviewers(t *testing.T) {
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
		{name: "different prevent_self_review differs", a: reviewers(true, user1), b: reviewers(false, user1), want: false},
		{name: "same ID of a different type differs", a: reviewers(false, user1), b: reviewers(false, team1), want: false},
		{name: "different ID differs", a: reviewers(false, user1), b: reviewers(false, user2), want: false},
		{name: "extra reviewer differs", a: reviewers(false, user1), b: reviewers(false, user1, user2), want: false},
		{name: "duplicated reviewer differs from a single one", a: reviewers(false, user1, user1), b: reviewers(false, user1), want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, b := EnvironmentProtection{RequiredReviewers: tc.a}, EnvironmentProtection{RequiredReviewers: tc.b}
			if got := a.sameRequiredReviewers(b); got != tc.want {
				t.Errorf("sameRequiredReviewers() = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestEnvironmentProtection_sameRequiredReviewersKeepsOrder(t *testing.T) {
	a := EnvironmentProtection{RequiredReviewers: reviewers(false, Reviewer{Type: ReviewerUser, ID: 2}, Reviewer{Type: ReviewerUser, ID: 1})}
	b := EnvironmentProtection{RequiredReviewers: reviewers(false, Reviewer{Type: ReviewerUser, ID: 1}, Reviewer{Type: ReviewerUser, ID: 2})}
	_ = a.sameRequiredReviewers(b)
	if got := a.RequiredReviewers.Reviewers[0].ID; got != 2 {
		t.Errorf("Reviewers[0].ID = %d, want 2 (the receiver must not be reordered)", got)
	}
	if got := b.RequiredReviewers.Reviewers[0].ID; got != 1 {
		t.Errorf("Reviewers[0].ID = %d, want 1 (the argument must not be reordered)", got)
	}
}

func TestEnvironmentProtection_waitTimerString(t *testing.T) {
	cases := []struct {
		name string
		p    *int
		want string
	}{
		{name: "none", p: nil, want: "none"},
		{name: "zero minutes", p: new(0), want: "0"},
		{name: "thirty minutes", p: new(30), want: "30"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := (EnvironmentProtection{WaitTimer: tc.p}).waitTimerString(); got != tc.want {
				t.Errorf("waitTimerString() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestEnvironmentProtection_requiredReviewersString(t *testing.T) {
	cases := []struct {
		name string
		r    *RequiredReviewers
		want string
	}{
		{name: "none", r: nil, want: "none"},
		{name: "empty reviewers", r: reviewers(false), want: "reviewers: [], prevent_self_review: false"},
		{
			name: "reviewers keep the observed order",
			r:    reviewers(true, Reviewer{Type: ReviewerUser, ID: 583231}, Reviewer{Type: ReviewerTeam, ID: 42}),
			want: "reviewers: [User 583231, Team 42], prevent_self_review: true",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := (EnvironmentProtection{RequiredReviewers: tc.r}).requiredReviewersString(); got != tc.want {
				t.Errorf("requiredReviewersString() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRequiredReviewers_ReviewersString(t *testing.T) {
	cases := []struct {
		name string
		r    RequiredReviewers
		want string
	}{
		{name: "empty", r: RequiredReviewers{}, want: "[]"},
		{name: "one reviewer", r: RequiredReviewers{Reviewers: []Reviewer{{Type: ReviewerTeam, ID: 42}}}, want: "[Team 42]"},
		{name: "observed order", r: RequiredReviewers{Reviewers: []Reviewer{{Type: ReviewerUser, ID: 583231}, {Type: ReviewerTeam, ID: 42}}}, want: "[User 583231, Team 42]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.r.ReviewersString(); got != tc.want {
				t.Errorf("ReviewersString() = %q, want %q", got, tc.want)
			}
		})
	}
}
