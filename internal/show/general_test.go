package show

import (
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

//trace:checks FR-004 QR-002
func TestWriteGeneral(t *testing.T) {
	cases := []struct {
		name    string
		general github.General
		want    string
	}{
		{
			name: "every general setting is shown as a value in the order of the configuration",
			general: github.General{
				DeleteBranchOnMerge: true, AllowAutoMerge: false, AllowSquashMerge: true, AllowMergeCommit: false, AllowRebaseMerge: true, AllowUpdateBranch: false,
				SquashMergeCommitDefault: github.Observed(setting.SquashMergeCommitDefault{Title: setting.SquashMergeCommitTitleCommitOrPRTitle, Message: setting.SquashMergeCommitMessageCommitMessages}),
				MergeCommitDefault:       github.Observed(setting.MergeCommitDefault{Title: setting.MergeCommitTitleMergeMessage, Message: setting.MergeCommitMessagePRTitle}),
				HasIssues:                true, HasProjects: false, HasWiki: true, WebCommitSignoffRequired: github.Observed(false),
			},
			want: "    general:\n" +
				"      delete_branch_on_merge: true\n" +
				"      allow_auto_merge: false\n" +
				"      allow_squash_merge: true\n" +
				"      allow_merge_commit: false\n" +
				"      allow_rebase_merge: true\n" +
				"      allow_update_branch: false\n" +
				"      squash_merge_commit:\n" +
				"        title: COMMIT_OR_PR_TITLE\n" +
				"        message: COMMIT_MESSAGES\n" +
				"      merge_commit:\n" +
				"        title: MERGE_MESSAGE\n" +
				"        message: PR_TITLE\n" +
				"      has_issues: true\n" +
				"      has_projects: false\n" +
				"      has_wiki: true\n" +
				"      web_commit_signoff_required: false\n",
		},
		{
			name: "opposite values are shown as observed",
			general: github.General{
				DeleteBranchOnMerge: false, AllowAutoMerge: true, AllowSquashMerge: false, AllowMergeCommit: true, AllowRebaseMerge: false, AllowUpdateBranch: true,
				SquashMergeCommitDefault: github.Observed(setting.SquashMergeCommitDefault{Title: setting.SquashMergeCommitTitlePRTitle, Message: setting.SquashMergeCommitMessagePRBody}),
				MergeCommitDefault:       github.Observed(setting.MergeCommitDefault{Title: setting.MergeCommitTitlePRTitle, Message: setting.MergeCommitMessageBlank}),
				HasIssues:                false, HasProjects: true, HasWiki: false, WebCommitSignoffRequired: github.Observed(true),
			},
			want: "    general:\n" +
				"      delete_branch_on_merge: false\n" +
				"      allow_auto_merge: true\n" +
				"      allow_squash_merge: false\n" +
				"      allow_merge_commit: true\n" +
				"      allow_rebase_merge: false\n" +
				"      allow_update_branch: true\n" +
				"      squash_merge_commit:\n" +
				"        title: PR_TITLE\n" +
				"        message: PR_BODY\n" +
				"      merge_commit:\n" +
				"        title: PR_TITLE\n" +
				"        message: BLANK\n" +
				"      has_issues: false\n" +
				"      has_projects: true\n" +
				"      has_wiki: false\n" +
				"      web_commit_signoff_required: true\n",
		},
		{
			name: "unknown squash merge message and missing signoff are shown as unavailable on their own lines",
			general: github.General{
				AllowSquashMerge:         true,
				SquashMergeCommitDefault: github.Unavailable[setting.SquashMergeCommitDefault](github.UnavailableUnknownValue),
				MergeCommitDefault:       github.Observed(setting.MergeCommitDefault{Title: setting.MergeCommitTitleMergeMessage, Message: setting.MergeCommitMessagePRTitle}),
				HasIssues:                true, HasProjects: true, HasWiki: true,
				WebCommitSignoffRequired: github.Unavailable[bool](github.UnavailableNotReturned),
			},
			want: "    general:\n" +
				"      delete_branch_on_merge: false\n" +
				"      allow_auto_merge: false\n" +
				"      allow_squash_merge: true\n" +
				"      allow_merge_commit: false\n" +
				"      allow_rebase_merge: false\n" +
				"      allow_update_branch: false\n" +
				"      # squash_merge_commit: unavailable (GitHub returned a value that reco does not know)\n" +
				"      merge_commit:\n" +
				"        title: MERGE_MESSAGE\n" +
				"        message: PR_TITLE\n" +
				"      has_issues: true\n" +
				"      has_projects: true\n" +
				"      has_wiki: true\n" +
				"      # web_commit_signoff_required: unavailable (GitHub did not return the value)\n",
		},
		{
			name: "unknown merge commit message is shown as unavailable",
			general: github.General{
				AllowMergeCommit:         true,
				SquashMergeCommitDefault: github.Observed(setting.SquashMergeCommitDefault{Title: setting.SquashMergeCommitTitlePRTitle, Message: setting.SquashMergeCommitMessageBlank}),
				MergeCommitDefault:       github.Unavailable[setting.MergeCommitDefault](github.UnavailableUnknownValue),
				WebCommitSignoffRequired: github.Observed(false),
			},
			want: "    general:\n" +
				"      delete_branch_on_merge: false\n" +
				"      allow_auto_merge: false\n" +
				"      allow_squash_merge: false\n" +
				"      allow_merge_commit: true\n" +
				"      allow_rebase_merge: false\n" +
				"      allow_update_branch: false\n" +
				"      squash_merge_commit:\n" +
				"        title: PR_TITLE\n" +
				"        message: BLANK\n" +
				"      # merge_commit: unavailable (GitHub returned a value that reco does not know)\n" +
				"      has_issues: false\n" +
				"      has_projects: false\n" +
				"      has_wiki: false\n" +
				"      web_commit_signoff_required: false\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder
			writeGeneral(&b, tc.general)
			if got := b.String(); got != tc.want {
				t.Errorf("writeGeneral() =\n%s\nwant\n%s", got, tc.want)
			}
		})
	}

	t.Run("merge message unavailable for a reason other than an unknown value panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("writeGeneral() did not panic for a merge message unavailable because the repository is private")
			}
		}()
		writeGeneral(&strings.Builder{}, github.General{
			SquashMergeCommitDefault: github.Unavailable[setting.SquashMergeCommitDefault](github.UnavailablePrivateRepository),
			MergeCommitDefault:       github.Observed(setting.MergeCommitDefault{Title: setting.MergeCommitTitleMergeMessage, Message: setting.MergeCommitMessagePRTitle}),
			WebCommitSignoffRequired: github.Observed(false),
		})
	})

	t.Run("signoff unavailable for a reason other than not returned panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("writeGeneral() did not panic for the signoff unavailable because of an unknown value")
			}
		}()
		writeGeneral(&strings.Builder{}, github.General{
			SquashMergeCommitDefault: github.Observed(setting.SquashMergeCommitDefault{Title: setting.SquashMergeCommitTitlePRTitle, Message: setting.SquashMergeCommitMessageBlank}),
			MergeCommitDefault:       github.Observed(setting.MergeCommitDefault{Title: setting.MergeCommitTitleMergeMessage, Message: setting.MergeCommitMessagePRTitle}),
			WebCommitSignoffRequired: github.Unavailable[bool](github.UnavailableUnknownValue),
		})
	})
}
