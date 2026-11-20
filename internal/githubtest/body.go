package githubtest

import (
	"fmt"
	"strings"
)

// 応答本文は、各取得エンドポイントの応答のうち reco が読む項目だけを持つ。
// 値の組み合わせはテストごとに違うため、項目名だけをここに閉じ、値は各テストが構造体で書く。
// 文字列の値は %q で埋め込む。Go の引用は ASCII の印字可能な文字では JSON の文字列と同じ形になり、テストはその範囲の値だけを使う。

// RepositoryBody は、リポジトリの取得の応答本文である。
// Private のゼロ値は公開のリポジトリを表す。
type RepositoryBody struct {
	ID                  int64
	AllowAutoMerge      bool
	DeleteBranchOnMerge bool
	Private             bool
}

// ActionsPermissionsBody は、Actions が有効なリポジトリの Actions permissions の取得の応答本文である。
type ActionsPermissionsBody struct {
	AllowedActions     string
	SHAPinningRequired bool
}

// ActionsDisabledBody は、Actions が無効なリポジトリの Actions permissions の取得の応答本文である。
// GitHub は無効のときも sha_pinning_required を返すが、allowed_actions は返さない。
const ActionsDisabledBody = `{"enabled": false, "sha_pinning_required": false}`

// SelectedActionsBody は、Selected actions の取得の応答本文である。
type SelectedActionsBody struct {
	GitHubOwnedAllowed bool
	VerifiedAllowed    bool
	PatternsAllowed    []string
}

// WorkflowPermissionsBody は、Workflow permissions の取得の応答本文である。
type WorkflowPermissionsBody struct {
	DefaultWorkflowPermissions   string
	CanApprovePullRequestReviews bool
}

// ActionsAccessBody は、リポジトリ外のワークフローからのアクセスの取得の応答本文である。
type ActionsAccessBody struct {
	AccessLevel string
}

// JSON は応答本文を返す。
func (b RepositoryBody) JSON() string {
	return fmt.Sprintf(`{"id": %d, "allow_auto_merge": %t, "delete_branch_on_merge": %t, "private": %t}`,
		b.ID, b.AllowAutoMerge, b.DeleteBranchOnMerge, b.Private)
}

// JSON は応答本文を返す。
func (b ActionsPermissionsBody) JSON() string {
	return fmt.Sprintf(`{"enabled": true, "allowed_actions": %q, "sha_pinning_required": %t}`, b.AllowedActions, b.SHAPinningRequired)
}

// JSON は応答本文を返す。
func (b SelectedActionsBody) JSON() string {
	patterns := make([]string, len(b.PatternsAllowed))
	for i, p := range b.PatternsAllowed {
		patterns[i] = fmt.Sprintf("%q", p)
	}
	return fmt.Sprintf(`{"github_owned_allowed": %t, "verified_allowed": %t, "patterns_allowed": [%s]}`,
		b.GitHubOwnedAllowed, b.VerifiedAllowed, strings.Join(patterns, ", "))
}

// JSON は応答本文を返す。
func (b WorkflowPermissionsBody) JSON() string {
	return fmt.Sprintf(`{"default_workflow_permissions": %q, "can_approve_pull_request_reviews": %t}`, b.DefaultWorkflowPermissions, b.CanApprovePullRequestReviews)
}

// JSON は応答本文を返す。
func (b ActionsAccessBody) JSON() string {
	return fmt.Sprintf(`{"access_level": %q}`, b.AccessLevel)
}
