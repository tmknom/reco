package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/setting"
)

// privateRepositoryBody は repositoryBody の非公開のリポジトリである。
const privateRepositoryBody = `{
	"id": 1296269,
	"name": "repo",
	"full_name": "owner/repo",
	"owner": {"login": "owner"},
	"private": true,
	"html_url": "https://github.com/owner/repo",
	"allow_auto_merge": true,
	"delete_branch_on_merge": false,
	"allow_squash_merge": true,
	"allow_merge_commit": false,
	"allow_rebase_merge": true,
	"allow_update_branch": false,
	"squash_merge_commit_title": "COMMIT_OR_PR_TITLE",
	"squash_merge_commit_message": "COMMIT_MESSAGES",
	"merge_commit_title": "MERGE_MESSAGE",
	"merge_commit_message": "PR_TITLE",
	"has_issues": true,
	"has_projects": false,
	"has_wiki": true,
	"web_commit_signoff_required": false,
	"permissions": {"admin": true, "push": true, "pull": true}
}`

//trace:checks FR-004
func TestDecodeRepository(t *testing.T) {
	const general = `"id": 1296269, "allow_auto_merge": true, "delete_branch_on_merge": false, "allow_squash_merge": true, "allow_merge_commit": false, "allow_rebase_merge": true, "allow_update_branch": false, ` +
		`"squash_merge_commit_title": "COMMIT_OR_PR_TITLE", "squash_merge_commit_message": "COMMIT_MESSAGES", "merge_commit_title": "MERGE_MESSAGE", "merge_commit_message": "PR_TITLE", ` +
		`"has_issues": true, "has_projects": false, "has_wiki": true, "web_commit_signoff_required": false, "private": false`
	successes := []struct {
		name string
		body string
		want repositoryAttributes
	}{
		{
			name: "public repository is read with all fields",
			body: repositoryBody,
			want: repositoryAttributes{admin: true, securityAndAnalysis: json.RawMessage(repositorySecurityAndAnalysis)},
		},
		{name: "private repository is read with all fields", body: privateRepositoryBody, want: repositoryAttributes{private: true, admin: true}},
		{name: "false permissions.admin is no admin access", body: `{` + general + `, "permissions": {"admin": false, "pull": true}}`, want: repositoryAttributes{}},
		{name: "missing permissions is no admin access", body: `{` + general + `}`, want: repositoryAttributes{}},
		{name: "permissions without admin is no admin access", body: `{` + general + `, "permissions": {"pull": true}}`, want: repositoryAttributes{}},
		{
			name: "non-object security_and_analysis is carried uninterpreted",
			body: `{` + general + `, "security_and_analysis": "text"}`,
			want: repositoryAttributes{securityAndAnalysis: json.RawMessage(`"text"`)},
		},
	}
	for _, tc := range successes {
		t.Run(tc.name, func(t *testing.T) {
			got, attributes, err := decodeRepository([]byte(tc.body))
			if err != nil {
				t.Fatalf("decodeRepository() error = %v", err)
			}
			want := ObservationResult{RepositoryID: 1296269, General: General{
				AllowAutoMerge: true, DeleteBranchOnMerge: false, AllowSquashMerge: true, AllowMergeCommit: false, AllowRebaseMerge: true, AllowUpdateBranch: false,
				SquashMergeCommitDefault: Observed(setting.SquashMergeCommitDefault{Title: setting.SquashMergeCommitTitleCommitOrPRTitle, Message: setting.SquashMergeCommitMessageCommitMessages}),
				MergeCommitDefault:       Observed(setting.MergeCommitDefault{Title: setting.MergeCommitTitleMergeMessage, Message: setting.MergeCommitMessagePRTitle}),
				HasIssues:                true, HasProjects: false, HasWiki: true, WebCommitSignoffRequired: Observed(false),
			}}
			if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(attributes, tc.want) {
				t.Errorf("decodeRepository() = (%+v, %+v), want (%+v, %+v)", got, attributes, want, tc.want)
			}
		})
	}

	// 受理される本文から、対象のフィールド1つだけを欠かすか null にする。ほかのフィールドの欠落で先に失敗しないようにするため。
	const accepted = `"id": 1, "full_name": "o/r", "allow_auto_merge": true, "delete_branch_on_merge": true, ` +
		`"allow_squash_merge": true, "allow_merge_commit": true, "allow_rebase_merge": true, "allow_update_branch": true, ` +
		`"squash_merge_commit_title": "PR_TITLE", "squash_merge_commit_message": "PR_BODY", "merge_commit_title": "PR_TITLE", "merge_commit_message": "PR_BODY", ` +
		`"has_issues": true, "has_projects": true, "has_wiki": true, "web_commit_signoff_required": true, "private": false`
	without := func(field string) string {
		return `{` + strings.Replace(accepted, fmt.Sprintf(`"%s": `, field), fmt.Sprintf(`"unrelated_%s": `, field), 1) + `}`
	}
	withNull := func(field string) string {
		return `{` + strings.Replace(accepted, fmt.Sprintf(`"%s": true`, field), fmt.Sprintf(`"%s": null`, field), 1) + `}`
	}
	failures := []struct {
		name        string
		body        string
		errContains string
	}{
		{name: "missing id is unexpected response", body: without("id"), errContains: "id"},
		{name: "missing allow_auto_merge is unexpected response", body: without("allow_auto_merge"), errContains: "allow_auto_merge"},
		{name: "missing delete_branch_on_merge is unexpected response", body: without("delete_branch_on_merge"), errContains: "delete_branch_on_merge"},
		{name: "missing allow_squash_merge is unexpected response", body: without("allow_squash_merge"), errContains: "allow_squash_merge"},
		{name: "missing allow_merge_commit is unexpected response", body: without("allow_merge_commit"), errContains: "allow_merge_commit"},
		{name: "missing allow_rebase_merge is unexpected response", body: without("allow_rebase_merge"), errContains: "allow_rebase_merge"},
		{name: "missing allow_update_branch is unexpected response", body: without("allow_update_branch"), errContains: "allow_update_branch"},
		{name: "missing squash_merge_commit_title is unexpected response", body: without("squash_merge_commit_title"), errContains: "missing fields: squash_merge_commit_title"},
		{name: "missing squash_merge_commit_message is unexpected response", body: without("squash_merge_commit_message"), errContains: "missing fields: squash_merge_commit_message"},
		{name: "missing merge_commit_title is unexpected response", body: without("merge_commit_title"), errContains: "missing fields: merge_commit_title"},
		{name: "missing merge_commit_message is unexpected response", body: without("merge_commit_message"), errContains: "missing fields: merge_commit_message"},
		{name: "missing has_issues is unexpected response", body: without("has_issues"), errContains: "missing fields: has_issues"},
		{name: "missing has_projects is unexpected response", body: without("has_projects"), errContains: "missing fields: has_projects"},
		{name: "missing has_wiki is unexpected response", body: without("has_wiki"), errContains: "missing fields: has_wiki"},
		{name: "null allow_auto_merge is unexpected response", body: withNull("allow_auto_merge"), errContains: "allow_auto_merge"},
		// 省略を許されるのは欠落だけで、null は省略ではない
		{name: "null web_commit_signoff_required is unexpected response", body: withNull("web_commit_signoff_required"), errContains: "web_commit_signoff_required"},
		{
			name:        "non-boolean web_commit_signoff_required is unexpected response",
			body:        `{` + strings.Replace(accepted, `"web_commit_signoff_required": true`, `"web_commit_signoff_required": "yes"`, 1) + `}`,
			errContains: "web_commit_signoff_required",
		},
		{name: "non-string merge message is unexpected response", body: `{` + strings.Replace(accepted, `"merge_commit_title": "PR_TITLE"`, `"merge_commit_title": 1`, 1) + `}`},
		{name: "missing private is unexpected response", body: without("private"), errContains: "private"},
		{name: "non-JSON body is unexpected response", body: `<html>`},
		{name: "non-boolean permissions.admin is unexpected response", body: `{` + accepted + `, "permissions": {"admin": "yes"}}`},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := decodeRepository([]byte(tc.body))
			if !errors.Is(err, ErrUnexpectedResponse) {
				t.Fatalf("decodeRepository() error = %v, want %v", err, ErrUnexpectedResponse)
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Errorf("decodeRepository() error = %q, want it to contain %q", err, tc.errContains)
			}
		})
	}
}

// 知らない列挙値と、省略を許された値の欠落は、その設定だけを観測不能にし、ほかの General の設定と観測全体は成り立つ。
//
//trace:checks FR-004 QR-002
func TestDecodeRepository_UnavailableGeneral(t *testing.T) {
	const accepted = `"id": 1, "allow_auto_merge": false, "delete_branch_on_merge": false, ` +
		`"allow_squash_merge": true, "allow_merge_commit": true, "allow_rebase_merge": true, "allow_update_branch": false, ` +
		`"squash_merge_commit_title": "PR_TITLE", "squash_merge_commit_message": "PR_BODY", "merge_commit_title": "MERGE_MESSAGE", "merge_commit_message": "PR_TITLE", ` +
		`"has_issues": true, "has_projects": true, "has_wiki": true, "web_commit_signoff_required": true, "private": false`
	// GitHub が将来足し得る値に見立てる。既知の値とどの位置でも一致しない文字列にする
	withUnknown := func(field, value string) string {
		return `{` + strings.Replace(accepted, fmt.Sprintf(`"%s": %q`, field, value), fmt.Sprintf(`"%s": "PR_BODY_AND_COMMITS"`, field), 1) + `}`
	}
	squash := Observed(setting.SquashMergeCommitDefault{Title: setting.SquashMergeCommitTitlePRTitle, Message: setting.SquashMergeCommitMessagePRBody})
	merge := Observed(setting.MergeCommitDefault{Title: setting.MergeCommitTitleMergeMessage, Message: setting.MergeCommitMessagePRTitle})
	unknownSquash := Unavailable[setting.SquashMergeCommitDefault](UnavailableUnknownValue)
	unknownMerge := Unavailable[setting.MergeCommitDefault](UnavailableUnknownValue)
	signoff := Observed(true)
	cases := []struct {
		name        string
		body        string
		wantSquash  SettingObservation[setting.SquashMergeCommitDefault]
		wantMerge   SettingObservation[setting.MergeCommitDefault]
		wantSignoff SettingObservation[bool]
	}{
		{name: "unknown squash merge title makes only the squash merge message unavailable", body: withUnknown("squash_merge_commit_title", "PR_TITLE"), wantSquash: unknownSquash, wantMerge: merge, wantSignoff: signoff},
		{name: "unknown squash merge message makes only the squash merge message unavailable", body: withUnknown("squash_merge_commit_message", "PR_BODY"), wantSquash: unknownSquash, wantMerge: merge, wantSignoff: signoff},
		{name: "unknown merge commit title makes only the merge commit message unavailable", body: withUnknown("merge_commit_title", "MERGE_MESSAGE"), wantSquash: squash, wantMerge: unknownMerge, wantSignoff: signoff},
		{name: "unknown merge commit message makes only the merge commit message unavailable", body: withUnknown("merge_commit_message", "PR_TITLE"), wantSquash: squash, wantMerge: unknownMerge, wantSignoff: signoff},
		{
			name:        "missing web_commit_signoff_required makes only it unavailable as not returned",
			body:        `{` + strings.Replace(accepted, `"web_commit_signoff_required": `, `"unrelated_web_commit_signoff_required": `, 1) + `}`,
			wantSquash:  squash,
			wantMerge:   merge,
			wantSignoff: Unavailable[bool](UnavailableNotReturned),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _, err := decodeRepository([]byte(tc.body))
			if err != nil {
				t.Fatalf("decodeRepository() error = %v, want the observation to succeed", err)
			}
			want := General{
				AllowSquashMerge: true, AllowMergeCommit: true, AllowRebaseMerge: true,
				SquashMergeCommitDefault: tc.wantSquash, MergeCommitDefault: tc.wantMerge,
				HasIssues: true, HasProjects: true, HasWiki: true, WebCommitSignoffRequired: tc.wantSignoff,
			}
			if got.RepositoryID != 1 || got.General != want {
				t.Errorf("decodeRepository() = %+v, want ID 1 and General %+v", got, want)
			}
		})
	}
}

// GitHub が更新で受け付けない組と、マージ方式が無効の方式のマージメッセージも、返った値のまま読む。
//
//trace:checks FR-004
func TestDecodeRepository_GeneralValues(t *testing.T) {
	cases := []struct {
		name string
		body string
		want General
	}{
		{
			name: "every value of the first merge message pair is read for its field",
			body: generalBody(true, true, "PR_TITLE", "PR_BODY", "PR_TITLE", "PR_BODY", `"has_issues": true, "has_projects": false, "has_wiki": false, "web_commit_signoff_required": false`),
			want: General{
				AllowSquashMerge: true, AllowMergeCommit: true, AllowRebaseMerge: true,
				SquashMergeCommitDefault: Observed(setting.SquashMergeCommitDefault{Title: setting.SquashMergeCommitTitlePRTitle, Message: setting.SquashMergeCommitMessagePRBody}),
				MergeCommitDefault:       Observed(setting.MergeCommitDefault{Title: setting.MergeCommitTitlePRTitle, Message: setting.MergeCommitMessagePRBody}),
				HasIssues:                true,
				WebCommitSignoffRequired: Observed(false),
			},
		},
		{
			name: "BLANK messages are read for their fields",
			body: generalBody(true, true, "PR_TITLE", "BLANK", "PR_TITLE", "BLANK", `"has_issues": false, "has_projects": true, "has_wiki": false, "web_commit_signoff_required": false`),
			want: General{
				AllowSquashMerge: true, AllowMergeCommit: true, AllowRebaseMerge: true,
				SquashMergeCommitDefault: Observed(setting.SquashMergeCommitDefault{Title: setting.SquashMergeCommitTitlePRTitle, Message: setting.SquashMergeCommitMessageBlank}),
				MergeCommitDefault:       Observed(setting.MergeCommitDefault{Title: setting.MergeCommitTitlePRTitle, Message: setting.MergeCommitMessageBlank}),
				HasProjects:              true,
				WebCommitSignoffRequired: Observed(false),
			},
		},
		{
			name: "pairs that GitHub does not accept in an update are read as returned",
			body: generalBody(true, true, "COMMIT_OR_PR_TITLE", "PR_BODY", "MERGE_MESSAGE", "BLANK", `"has_issues": false, "has_projects": false, "has_wiki": true, "web_commit_signoff_required": false`),
			want: General{
				AllowSquashMerge: true, AllowMergeCommit: true, AllowRebaseMerge: true,
				SquashMergeCommitDefault: Observed(setting.SquashMergeCommitDefault{Title: setting.SquashMergeCommitTitleCommitOrPRTitle, Message: setting.SquashMergeCommitMessagePRBody}),
				MergeCommitDefault:       Observed(setting.MergeCommitDefault{Title: setting.MergeCommitTitleMergeMessage, Message: setting.MergeCommitMessageBlank}),
				HasWiki:                  true,
				WebCommitSignoffRequired: Observed(false),
			},
		},
		{
			name: "merge messages of disabled merge methods are read",
			body: generalBody(false, false, "COMMIT_OR_PR_TITLE", "COMMIT_MESSAGES", "MERGE_MESSAGE", "PR_TITLE", `"has_issues": false, "has_projects": false, "has_wiki": false, "web_commit_signoff_required": true`),
			want: General{
				AllowRebaseMerge:         true,
				SquashMergeCommitDefault: Observed(setting.SquashMergeCommitDefault{Title: setting.SquashMergeCommitTitleCommitOrPRTitle, Message: setting.SquashMergeCommitMessageCommitMessages}),
				MergeCommitDefault:       Observed(setting.MergeCommitDefault{Title: setting.MergeCommitTitleMergeMessage, Message: setting.MergeCommitMessagePRTitle}),
				WebCommitSignoffRequired: Observed(true),
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _, err := decodeRepository([]byte(tc.body))
			if err != nil {
				t.Fatalf("decodeRepository() error = %v", err)
			}
			if got.General != tc.want {
				t.Errorf("decodeRepository().General = %+v, want %+v", got.General, tc.want)
			}
		})
	}
}

// generalBody は、rebase merge を常に有効にし、マージ方式がすべて無効という GitHub が取り得ない状態を避けた本文を返す。features はリポジトリの機能と Web コミットのサインオフ必須の項目の並びである。
func generalBody(allowSquashMerge, allowMergeCommit bool, squashTitle, squashMessage, mergeTitle, mergeMessage, features string) string {
	return fmt.Sprintf(`{"id": 1, "allow_auto_merge": false, "delete_branch_on_merge": false, `+
		`"allow_squash_merge": %t, "allow_merge_commit": %t, "allow_rebase_merge": true, "allow_update_branch": false, `+
		`"squash_merge_commit_title": %q, "squash_merge_commit_message": %q, "merge_commit_title": %q, "merge_commit_message": %q, %s, "private": false}`,
		allowSquashMerge, allowMergeCommit, squashTitle, squashMessage, mergeTitle, mergeMessage, features)
}
