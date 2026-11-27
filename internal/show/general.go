package show

import (
	"fmt"
	"strings"

	"github.com/tmknom/reco/internal/github"
)

// writeGeneral は general を書く。
// マージメッセージ、リポジトリの機能、Web コミットのサインオフ必須は、設定宣言がまだ受け付けないため、すべての行を YAML のコメントにし、その前の行で書けないことを示す。
func writeGeneral(b *strings.Builder, g github.General) {
	fmt.Fprintf(b, "    general:\n      delete_branch_on_merge: %t\n      allow_auto_merge: %t\n"+
		"      allow_squash_merge: %t\n      allow_merge_commit: %t\n      allow_rebase_merge: %t\n      allow_update_branch: %t\n",
		g.DeleteBranchOnMerge, g.AllowAutoMerge, g.AllowSquashMerge, g.AllowMergeCommit, g.AllowRebaseMerge, g.AllowUpdateBranch)
	b.WriteString("      # reco does not accept the merge commit message settings in the configuration file yet\n")
	if d, ok := g.SquashMergeCommitDefault.Value(); ok {
		writeMergeMessage(b, "squash_merge_commit", string(d.Title), string(d.Message))
	} else {
		writeUnknownMergeMessage(b, "squash_merge_commit", g.SquashMergeCommitDefault.UnavailableReason())
	}
	if d, ok := g.MergeCommitDefault.Value(); ok {
		writeMergeMessage(b, "merge_commit", string(d.Title), string(d.Message))
	} else {
		writeUnknownMergeMessage(b, "merge_commit", g.MergeCommitDefault.UnavailableReason())
	}
	fmt.Fprintf(b, "      # reco does not accept the repository feature and web commit signoff settings in the configuration file yet\n"+
		"      # has_issues: %t\n      # has_projects: %t\n      # has_wiki: %t\n",
		g.HasIssues, g.HasProjects, g.HasWiki)
	writeWebCommitSignoffRequired(b, g.WebCommitSignoffRequired)
}

func writeMergeMessage(b *strings.Builder, key, title, message string) {
	fmt.Fprintf(b, "      # %s:\n      #   title: %s\n      #   message: %s\n", key, title, message)
}

// writeUnknownMergeMessage は、観測不能なマージメッセージを書く。
// 観測は知らない値だけをマージメッセージの観測不能にするため、ほかの理由を知らない値の文言で示さないよう、不変条件の破れとして panic する。
func writeUnknownMergeMessage(b *strings.Builder, key string, reason github.UnavailableReason) {
	if reason != github.UnavailableUnknownValue {
		panic(fmt.Sprintf("show: undefined unavailable reason %d for %s", int(reason), key))
	}
	fmt.Fprintf(b, "      # %s: unavailable (GitHub returned a value that reco does not know)\n", key)
}

// writeWebCommitSignoffRequired は web_commit_signoff_required を書く。
// 観測は値の欠落だけをこの設定の観測不能にするため、ほかの理由は不変条件の破れとして panic する。
func writeWebCommitSignoffRequired(b *strings.Builder, o github.SettingObservation[bool]) {
	if required, ok := o.Value(); ok {
		fmt.Fprintf(b, "      # web_commit_signoff_required: %t\n", required)
		return
	}
	if reason := o.UnavailableReason(); reason != github.UnavailableNotReturned {
		panic(fmt.Sprintf("show: undefined unavailable reason %d for web_commit_signoff_required", int(reason)))
	}
	b.WriteString("      # web_commit_signoff_required: unavailable (GitHub did not return the value)\n")
}
