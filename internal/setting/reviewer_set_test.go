package setting

import (
	"slices"
	"testing"
)

var (
	reviewerUser1 = Reviewer{Type: ReviewerUser, ID: 1}
	reviewerUser2 = Reviewer{Type: ReviewerUser, ID: 2}
	reviewerTeam1 = Reviewer{Type: ReviewerTeam, ID: 1}
)

// observedReviewerSet は、logins の login を添えた担当者の観測値を作る。
func observedReviewerSet(logins map[Reviewer]string, rs ...Reviewer) ReviewerSet {
	return ReviewerSetOfRule(&RequiredReviewers{Reviewers: rs, Logins: logins})
}

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

//trace:checks FR-005
func TestReviewerSet_equal(t *testing.T) {
	cases := []struct {
		name  string
		a, b  ReviewerSet
		equal bool
	}{
		{name: "sets that differ only in order are equal", a: ReviewerSetOf([]Reviewer{reviewerUser1, reviewerUser2}), b: ReviewerSetOf([]Reviewer{reviewerUser2, reviewerUser1}), equal: true},
		{name: "a set with a duplicate equals the set without it", a: ReviewerSetOf([]Reviewer{reviewerUser1, reviewerUser1, reviewerUser2}), b: ReviewerSetOf([]Reviewer{reviewerUser1, reviewerUser2}), equal: true},
		{
			name:  "sets that differ only in logins are equal",
			a:     observedReviewerSet(map[Reviewer]string{reviewerUser1: "octocat"}, reviewerUser1),
			b:     observedReviewerSet(map[Reviewer]string{reviewerUser1: "hubot"}, reviewerUser1),
			equal: true,
		},
		{name: "reviewers with the same id and different types differ", a: ReviewerSetOf([]Reviewer{reviewerUser1}), b: ReviewerSetOf([]Reviewer{reviewerTeam1})},
		{name: "sets of the same size with one different reviewer differ", a: ReviewerSetOf([]Reviewer{reviewerUser1}), b: ReviewerSetOf([]Reviewer{reviewerUser2})},
		{name: "no rule and a rule without reviewers differ", a: ReviewerSetOfRule(nil), b: ReviewerSetOfRule(&RequiredReviewers{Reviewers: []Reviewer{}})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.equal(tc.b); got != tc.equal {
				t.Errorf("%s.equal(%s) = %t, want %t", tc.a.String(), tc.b.String(), got, tc.equal)
			}
		})
	}
}

//trace:checks FR-004
func TestReviewerSetOfRule(t *testing.T) {
	cases := []struct {
		name string
		rule *RequiredReviewers
		want string
	}{
		{name: "no rule is the empty set", rule: nil, want: "[]"},
		{name: "a rule without reviewers is marked", rule: &RequiredReviewers{Reviewers: []Reviewer{}}, want: "[] (protection rule without reviewers)"},
		{
			name: "a rule with reviewers keeps their order and logins",
			rule: &RequiredReviewers{Reviewers: []Reviewer{reviewerUser2, reviewerUser1}, Logins: map[Reviewer]string{reviewerUser2: "hubot"}},
			want: "[User 2 (hubot), User 1]",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ReviewerSetOfRule(tc.rule).String(); got != tc.want {
				t.Errorf("ReviewerSetOfRule(...).String() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestReviewerSetOfRule_KeepsReviewersFromLaterChanges(t *testing.T) {
	rule := &RequiredReviewers{Reviewers: []Reviewer{reviewerUser1}, Logins: map[Reviewer]string{reviewerUser1: "octocat"}}
	set := ReviewerSetOfRule(rule)
	rule.Reviewers[0] = reviewerUser2
	rule.Logins[reviewerUser1] = "hubot"
	if got := set.String(); got != "[User 1 (octocat)]" {
		t.Errorf("String() after changing the rule = %q, want %q", got, "[User 1 (octocat)]")
	}
}

func TestReviewerSetOf_KeepsReviewersFromLaterChanges(t *testing.T) {
	declared := []Reviewer{reviewerUser1}
	set := ReviewerSetOf(declared)
	declared[0] = reviewerUser2
	if got := set.Reviewers(); !slices.Equal(got, []Reviewer{reviewerUser1}) {
		t.Errorf("Reviewers() after changing the argument = %v, want %v", got, []Reviewer{reviewerUser1})
	}
}

//trace:checks FR-007
func TestReviewerSet_NotIn(t *testing.T) {
	cases := []struct {
		name  string
		set   ReviewerSet
		other ReviewerSet
		want  string
	}{
		{
			name:  "reviewers missing from other are returned in the order of the receiver with logins",
			set:   observedReviewerSet(map[Reviewer]string{reviewerUser2: "hubot"}, reviewerUser2, reviewerUser1, reviewerTeam1),
			other: ReviewerSetOf([]Reviewer{reviewerUser1}),
			want:  "[User 2 (hubot), Team 1]",
		},
		{name: "a rule without reviewers has no reviewers missing from other", set: ReviewerSetOfRule(&RequiredReviewers{}), other: ReviewerSetOf(nil), want: "[]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.set.NotIn(tc.other).String(); got != tc.want {
				t.Errorf("NotIn(...).String() = %q, want %q", got, tc.want)
			}
		})
	}
}

//trace:checks FR-007
func TestReviewerSet_WithLoginsFrom(t *testing.T) {
	declared := ReviewerSetOf([]Reviewer{reviewerUser2, reviewerUser1})
	observed := observedReviewerSet(map[Reviewer]string{reviewerUser1: "octocat", reviewerTeam1: "unexpected"}, reviewerUser1, reviewerTeam1)
	if got, want := declared.WithLoginsFrom(observed).String(), "[User 2, User 1 (octocat)]"; got != want {
		t.Errorf("WithLoginsFrom(...).String() = %q, want %q", got, want)
	}
}

//trace:checks FR-012 INV-001
func TestReviewerSet_WithoutLogins(t *testing.T) {
	cases := []struct {
		name string
		set  ReviewerSet
		want string
	}{
		{
			name: "logins are dropped while the reviewers and their order are kept",
			set:  observedReviewerSet(map[Reviewer]string{reviewerUser2: "hubot", reviewerUser1: "octocat"}, reviewerUser2, reviewerTeam1, reviewerUser1),
			want: "[User 2, Team 1, User 1]",
		},
		{name: "a rule without reviewers keeps its mark", set: ReviewerSetOfRule(&RequiredReviewers{}), want: "[] (protection rule without reviewers)"},
		{name: "no rule stays the empty set", set: ReviewerSetOfRule(nil), want: "[]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.set.WithoutLogins().String(); got != tc.want {
				t.Errorf("WithoutLogins().String() = %q, want %q", got, tc.want)
			}
		})
	}
}

//trace:checks FR-006
func TestReviewerSet_HasNoReviewers(t *testing.T) {
	cases := []struct {
		name string
		set  ReviewerSet
		want bool
	}{
		{name: "no rule has no reviewers", set: ReviewerSetOfRule(nil), want: true},
		{name: "a rule without reviewers has no reviewers", set: ReviewerSetOfRule(&RequiredReviewers{}), want: true},
		{name: "one reviewer is a reviewer", set: ReviewerSetOf([]Reviewer{reviewerUser1}), want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.set.HasNoReviewers(); got != tc.want {
				t.Errorf("HasNoReviewers() = %t, want %t", got, tc.want)
			}
		})
	}
}

//trace:checks FR-004
func TestReviewerSet_IsRuleWithoutReviewers(t *testing.T) {
	cases := []struct {
		name string
		set  ReviewerSet
		want bool
	}{
		{name: "a rule without reviewers is one", set: ReviewerSetOfRule(&RequiredReviewers{}), want: true},
		{name: "no rule is not one", set: ReviewerSetOfRule(nil), want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.set.IsRuleWithoutReviewers(); got != tc.want {
				t.Errorf("IsRuleWithoutReviewers() = %t, want %t", got, tc.want)
			}
		})
	}
}

//trace:checks FR-004
func TestReviewerSet_Login(t *testing.T) {
	set := observedReviewerSet(map[Reviewer]string{reviewerUser1: "octocat"}, reviewerUser1, reviewerTeam1)
	cases := []struct {
		name      string
		reviewer  Reviewer
		wantLogin string
		wantOK    bool
	}{
		{name: "a reviewer with an observed login has it", reviewer: reviewerUser1, wantLogin: "octocat", wantOK: true},
		{name: "a reviewer without an observed login has none", reviewer: reviewerTeam1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			login, ok := set.Login(tc.reviewer)
			if login != tc.wantLogin || ok != tc.wantOK {
				t.Errorf("Login(%v) = (%q, %t), want (%q, %t)", tc.reviewer, login, ok, tc.wantLogin, tc.wantOK)
			}
		})
	}
}

func TestReviewerSet_ReviewersIsolatesResult(t *testing.T) {
	set := ReviewerSetOf([]Reviewer{reviewerUser1})
	set.Reviewers()[0] = reviewerUser2
	if got := set.Reviewers(); !slices.Equal(got, []Reviewer{reviewerUser1}) {
		t.Errorf("Reviewers() after changing a result = %v, want %v", got, []Reviewer{reviewerUser1})
	}
}
