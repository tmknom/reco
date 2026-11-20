package setting

import (
	"slices"
	"testing"
)

//trace:checks FR-004
func TestParseDeploymentPatternType(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		want   DeploymentPatternType
		wantOK bool
	}{
		{name: "branch is accepted", input: "branch", want: DeploymentPatternBranch, wantOK: true},
		{name: "tag is accepted", input: "tag", want: DeploymentPatternTag, wantOK: true},
		{name: "undefined value is rejected", input: "commit"},
		{name: "different case is rejected", input: "Branch"},
		{name: "empty string is rejected", input: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseDeploymentPatternType(tc.input)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("ParseDeploymentPatternType(%q) = (%q, %t), want (%q, %t)", tc.input, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

//trace:checks FR-001
func TestParseDeploymentBranchPolicyMode(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		want   DeploymentBranchPolicyMode
		wantOK bool
	}{
		{name: "all is accepted", input: "all", want: DeploymentBranchPolicyAll, wantOK: true},
		{name: "protected_branches is accepted", input: "protected_branches", want: DeploymentBranchPolicyProtectedBranches, wantOK: true},
		{name: "selected is accepted", input: "selected", want: DeploymentBranchPolicySelected, wantOK: true},
		{name: "different case is rejected", input: "All"},
		{name: "empty string is rejected", input: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseDeploymentBranchPolicyMode(tc.input)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("ParseDeploymentBranchPolicyMode(%q) = (%q, %t), want (%q, %t)", tc.input, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

//trace:checks FR-004
func TestDeploymentBranchPolicy_PatternsString(t *testing.T) {
	cases := []struct {
		name     string
		patterns []DeploymentPattern
		want     string
	}{
		{name: "empty set is shown as brackets", patterns: nil, want: `[]`},
		{
			name:     "patterns keep the given order with their type",
			patterns: []DeploymentPattern{{Type: DeploymentPatternTag, Name: "v*"}, {Type: DeploymentPatternBranch, Name: "main"}},
			want:     `[tag "v*", branch "main"]`,
		},
		{
			name:     "newline in a name is escaped instead of breaking the line",
			patterns: []DeploymentPattern{{Type: DeploymentPatternBranch, Name: "a\nb"}},
			want:     `[branch "a\nb"]`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := DeploymentBranchPolicy{Mode: DeploymentBranchPolicySelected, Patterns: tc.patterns}
			if got := p.PatternsString(); got != tc.want {
				t.Errorf("PatternsString() = %s, want %s", got, tc.want)
			}
		})
	}
}

//trace:checks FR-005
func TestDeploymentBranchPolicy_PatternsNotIn(t *testing.T) {
	branch := func(name string) DeploymentPattern {
		return DeploymentPattern{Type: DeploymentPatternBranch, Name: name}
	}
	tag := func(name string) DeploymentPattern { return DeploymentPattern{Type: DeploymentPatternTag, Name: name} }
	selected := func(patterns ...DeploymentPattern) DeploymentBranchPolicy {
		return DeploymentBranchPolicy{Mode: DeploymentBranchPolicySelected, Patterns: patterns}
	}
	cases := []struct {
		name  string
		p     DeploymentBranchPolicy
		other DeploymentBranchPolicy
		want  []DeploymentPattern
	}{
		{
			name:  "patterns in other are left out and the rest keep the order of p",
			p:     selected(branch("main"), branch("release/*"), branch("feature/*")),
			other: selected(branch("main")),
			want:  []DeploymentPattern{branch("release/*"), branch("feature/*")},
		},
		{
			name:  "pattern with the same name and another type is kept",
			p:     selected(tag("main")),
			other: selected(branch("main")),
			want:  []DeploymentPattern{tag("main")},
		},
		{
			name:  "pattern whose name differs only in letter case is kept",
			p:     selected(branch("MAIN")),
			other: selected(branch("main")),
			want:  []DeploymentPattern{branch("MAIN")},
		},
		{
			name:  "other without selected has no patterns, so all patterns of p are returned",
			p:     selected(branch("main")),
			other: DeploymentBranchPolicy{Mode: DeploymentBranchPolicyAll},
			want:  []DeploymentPattern{branch("main")},
		},
		{
			name:  "same set in another order returns nothing",
			p:     selected(branch("main"), tag("v*")),
			other: selected(tag("v*"), branch("main")),
			want:  nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.p.PatternsNotIn(tc.other); !slices.Equal(got, tc.want) {
				t.Errorf("PatternsNotIn() = %v, want %v", got, tc.want)
			}
		})
	}
}
