package setting

// SquashMergeCommitDefault は、squash merge のマージメッセージである。タイトルと本文の組を一つの値として持つ。
type SquashMergeCommitDefault struct {
	Title   SquashMergeCommitTitle
	Message SquashMergeCommitMessage
}

// MergeCommitDefault は、merge commit のマージメッセージである。タイトルと本文の組を一つの値として持つ。
type MergeCommitDefault struct {
	Title   MergeCommitTitle
	Message MergeCommitMessage
}

// SquashMergeCommitTitle は、squash merge のマージメッセージのタイトルが取り得る値。値は GitHub REST API の文字列で、表示にもそのまま使う。
type SquashMergeCommitTitle string

// SquashMergeCommitMessage は、squash merge のマージメッセージの本文が取り得る値。値は GitHub REST API の文字列で、表示にもそのまま使う。
type SquashMergeCommitMessage string

// MergeCommitTitle は、merge commit のマージメッセージのタイトルが取り得る値。値は GitHub REST API の文字列で、表示にもそのまま使う。
type MergeCommitTitle string

// MergeCommitMessage は、merge commit のマージメッセージの本文が取り得る値。値は GitHub REST API の文字列で、表示にもそのまま使う。
type MergeCommitMessage string

const (
	SquashMergeCommitTitlePRTitle         SquashMergeCommitTitle = "PR_TITLE"
	SquashMergeCommitTitleCommitOrPRTitle SquashMergeCommitTitle = "COMMIT_OR_PR_TITLE"
)

const (
	SquashMergeCommitMessagePRBody         SquashMergeCommitMessage = "PR_BODY"
	SquashMergeCommitMessageCommitMessages SquashMergeCommitMessage = "COMMIT_MESSAGES"
	SquashMergeCommitMessageBlank          SquashMergeCommitMessage = "BLANK"
)

const (
	MergeCommitTitlePRTitle      MergeCommitTitle = "PR_TITLE"
	MergeCommitTitleMergeMessage MergeCommitTitle = "MERGE_MESSAGE"
)

const (
	MergeCommitMessagePRBody  MergeCommitMessage = "PR_BODY"
	MergeCommitMessagePRTitle MergeCommitMessage = "PR_TITLE"
	MergeCommitMessageBlank   MergeCommitMessage = "BLANK"
)

// ParseSquashMergeCommitTitle は s が定義された値に完全一致するときだけ、その値と真を返す。
func ParseSquashMergeCommitTitle(s string) (SquashMergeCommitTitle, bool) {
	switch title := SquashMergeCommitTitle(s); title {
	case SquashMergeCommitTitlePRTitle, SquashMergeCommitTitleCommitOrPRTitle:
		return title, true
	default:
		return "", false
	}
}

// ParseSquashMergeCommitMessage は s が定義された値に完全一致するときだけ、その値と真を返す。
func ParseSquashMergeCommitMessage(s string) (SquashMergeCommitMessage, bool) {
	switch message := SquashMergeCommitMessage(s); message {
	case SquashMergeCommitMessagePRBody, SquashMergeCommitMessageCommitMessages, SquashMergeCommitMessageBlank:
		return message, true
	default:
		return "", false
	}
}

// ParseMergeCommitTitle は s が定義された値に完全一致するときだけ、その値と真を返す。
func ParseMergeCommitTitle(s string) (MergeCommitTitle, bool) {
	switch title := MergeCommitTitle(s); title {
	case MergeCommitTitlePRTitle, MergeCommitTitleMergeMessage:
		return title, true
	default:
		return "", false
	}
}

// ParseMergeCommitMessage は s が定義された値に完全一致するときだけ、その値と真を返す。
func ParseMergeCommitMessage(s string) (MergeCommitMessage, bool) {
	switch message := MergeCommitMessage(s); message {
	case MergeCommitMessagePRBody, MergeCommitMessagePRTitle, MergeCommitMessageBlank:
		return message, true
	default:
		return "", false
	}
}
