package setting

import "testing"

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
