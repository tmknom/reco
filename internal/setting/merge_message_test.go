package setting

import "testing"

//trace:checks FR-004
func TestParseSquashMergeCommitTitle(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		want   SquashMergeCommitTitle
		wantOK bool
	}{
		{name: "PR_TITLE is accepted", input: "PR_TITLE", want: SquashMergeCommitTitlePRTitle, wantOK: true},
		{name: "COMMIT_OR_PR_TITLE is accepted", input: "COMMIT_OR_PR_TITLE", want: SquashMergeCommitTitleCommitOrPRTitle, wantOK: true},
		{name: "merge commit title MERGE_MESSAGE is rejected", input: "MERGE_MESSAGE"},
		{name: "different case is rejected", input: "pr_title"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseSquashMergeCommitTitle(tc.input)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("ParseSquashMergeCommitTitle(%q) = (%q, %t), want (%q, %t)", tc.input, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

//trace:checks FR-004
func TestParseSquashMergeCommitMessage(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		want   SquashMergeCommitMessage
		wantOK bool
	}{
		{name: "PR_BODY is accepted", input: "PR_BODY", want: SquashMergeCommitMessagePRBody, wantOK: true},
		{name: "COMMIT_MESSAGES is accepted", input: "COMMIT_MESSAGES", want: SquashMergeCommitMessageCommitMessages, wantOK: true},
		{name: "BLANK is accepted", input: "BLANK", want: SquashMergeCommitMessageBlank, wantOK: true},
		{name: "merge commit message PR_TITLE is rejected", input: "PR_TITLE"},
		{name: "different case is rejected", input: "blank"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseSquashMergeCommitMessage(tc.input)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("ParseSquashMergeCommitMessage(%q) = (%q, %t), want (%q, %t)", tc.input, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

//trace:checks FR-004
func TestParseMergeCommitTitle(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		want   MergeCommitTitle
		wantOK bool
	}{
		{name: "PR_TITLE is accepted", input: "PR_TITLE", want: MergeCommitTitlePRTitle, wantOK: true},
		{name: "MERGE_MESSAGE is accepted", input: "MERGE_MESSAGE", want: MergeCommitTitleMergeMessage, wantOK: true},
		{name: "squash merge title COMMIT_OR_PR_TITLE is rejected", input: "COMMIT_OR_PR_TITLE"},
		{name: "different case is rejected", input: "merge_message"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseMergeCommitTitle(tc.input)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("ParseMergeCommitTitle(%q) = (%q, %t), want (%q, %t)", tc.input, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

//trace:checks FR-004
func TestParseMergeCommitMessage(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		want   MergeCommitMessage
		wantOK bool
	}{
		{name: "PR_BODY is accepted", input: "PR_BODY", want: MergeCommitMessagePRBody, wantOK: true},
		{name: "PR_TITLE is accepted", input: "PR_TITLE", want: MergeCommitMessagePRTitle, wantOK: true},
		{name: "BLANK is accepted", input: "BLANK", want: MergeCommitMessageBlank, wantOK: true},
		{name: "squash merge message COMMIT_MESSAGES is rejected", input: "COMMIT_MESSAGES"},
		{name: "different case is rejected", input: "pr_body"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseMergeCommitMessage(tc.input)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("ParseMergeCommitMessage(%q) = (%q, %t), want (%q, %t)", tc.input, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}
