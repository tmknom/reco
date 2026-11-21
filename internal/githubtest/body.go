package githubtest

import (
	"fmt"
	"strings"
)

// 応答本文は、各取得エンドポイントの応答のうち reco が読む項目だけを持つ。
// 値の組み合わせはテストごとに違うため、項目名だけをここに閉じ、値は各テストが構造体で書く。
// 文字列の値は %q で埋め込む。Go の引用は ASCII の印字可能な文字では JSON の文字列と同じ形になり、テストはその範囲の値だけを使う。

// RepositoryBody は、リポジトリの取得の応答本文である。
// Private のゼロ値は公開のリポジトリを、Admin のゼロ値は admin のアクセスを持たないトークンの所有者を表す。
// SecretScanning と SecretScanningPushProtection は、GitHub と同じく公開で admin のアクセスを持つときだけ security_and_analysis に含め、ゼロ値は disabled を表す。
type RepositoryBody struct {
	ID                           int64
	AllowAutoMerge               bool
	DeleteBranchOnMerge          bool
	Private                      bool
	Admin                        bool
	SecretScanning               bool
	SecretScanningPushProtection bool
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

// AutomatedSecurityFixesBody は、Dependabot security updates の取得の応答本文である。
type AutomatedSecurityFixesBody struct {
	Enabled bool
	Paused  bool
}

// PrivateVulnerabilityReportingBody は、Private vulnerability reporting の取得の応答本文である。
type PrivateVulnerabilityReportingBody struct {
	Enabled bool
}

// EnvironmentsBody は、Environment の一覧の1ページの応答本文である。
// TotalCount は GitHub が返す全件数で、このページの要素数とは限らない。
type EnvironmentsBody struct {
	TotalCount   int
	Environments []EnvironmentBody
}

// EnvironmentBody は、Environment の一覧の要素である。
// DeploymentBranchPolicy が nil の要素は deployment_branch_policy を null で返す。nil でなければ、GitHub と同じく branch_policy の保護ルールを含める。
// WaitTimer と RequiredReviewers は、nil ならその保護ルールを含めない。
type EnvironmentBody struct {
	ID                     int64
	Name                   string
	DeploymentBranchPolicy *DeploymentBranchPolicyBody
	WaitTimer              *int
	RequiredReviewers      *RequiredReviewersBody
	CanAdminsBypass        bool
}

// DeploymentBranchPolicyBody は、Environment の deployment_branch_policy である。
type DeploymentBranchPolicyBody struct {
	ProtectedBranches    bool
	CustomBranchPolicies bool
}

// RequiredReviewersBody は、required_reviewers の保護ルールである。
type RequiredReviewersBody struct {
	PreventSelfReview bool
	Reviewers         []ReviewerBody
}

// ReviewerBody は、required_reviewers の保護ルールのレビュアーである。Type は User か Team。
type ReviewerBody struct {
	Type string
	ID   int64
}

// DeploymentBranchPoliciesBody は、デプロイ元パターンの一覧の1ページの応答本文である。
// TotalCount は GitHub が返す全件数で、このページの要素数とは限らない。
type DeploymentBranchPoliciesBody struct {
	TotalCount int
	Patterns   []DeploymentPatternBody
}

// DeploymentPatternBody は、デプロイ元パターンの一覧の要素と、作成の正常応答の本文である。Type は branch か tag。
type DeploymentPatternBody struct {
	ID   int64
	Name string
	Type string
}

// JSON は応答本文を返す。
func (b RepositoryBody) JSON() string {
	securityAndAnalysis := ""
	if !b.Private && b.Admin {
		securityAndAnalysis = fmt.Sprintf(`, "security_and_analysis": {"secret_scanning": {"status": %q}, "secret_scanning_push_protection": {"status": %q}}`,
			securityStatus(b.SecretScanning), securityStatus(b.SecretScanningPushProtection))
	}
	return fmt.Sprintf(`{"id": %d, "allow_auto_merge": %t, "delete_branch_on_merge": %t, "private": %t, "permissions": {"admin": %t}%s}`,
		b.ID, b.AllowAutoMerge, b.DeleteBranchOnMerge, b.Private, b.Admin, securityAndAnalysis)
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

// JSON は応答本文を返す。
func (b AutomatedSecurityFixesBody) JSON() string {
	return fmt.Sprintf(`{"enabled": %t, "paused": %t}`, b.Enabled, b.Paused)
}

// JSON は応答本文を返す。
func (b PrivateVulnerabilityReportingBody) JSON() string {
	return fmt.Sprintf(`{"enabled": %t}`, b.Enabled)
}

// JSON は応答本文を返す。
func (b EnvironmentsBody) JSON() string {
	environments := make([]string, len(b.Environments))
	for i, e := range b.Environments {
		environments[i] = e.JSON()
	}
	return fmt.Sprintf(`{"total_count": %d, "environments": [%s]}`, b.TotalCount, strings.Join(environments, ", "))
}

// JSON は Environment の1件の応答本文を返す。一覧の要素と同じ形で、保護ルールは GitHub が返した並び（required_reviewers、wait_timer、branch_policy）で並べる。
func (b EnvironmentBody) JSON() string {
	var rules []string
	if r := b.RequiredReviewers; r != nil {
		reviewers := make([]string, len(r.Reviewers))
		for i, reviewer := range r.Reviewers {
			reviewers[i] = fmt.Sprintf(`{"type": %q, "reviewer": {"id": %d}}`, reviewer.Type, reviewer.ID)
		}
		rules = append(rules, fmt.Sprintf(`{"type": "required_reviewers", "prevent_self_review": %t, "reviewers": [%s]}`,
			r.PreventSelfReview, strings.Join(reviewers, ", ")))
	}
	if b.WaitTimer != nil {
		rules = append(rules, fmt.Sprintf(`{"type": "wait_timer", "wait_timer": %d}`, *b.WaitTimer))
	}
	policy := "null"
	if p := b.DeploymentBranchPolicy; p != nil {
		rules = append(rules, `{"type": "branch_policy"}`)
		policy = fmt.Sprintf(`{"protected_branches": %t, "custom_branch_policies": %t}`, p.ProtectedBranches, p.CustomBranchPolicies)
	}
	return fmt.Sprintf(`{"id": %d, "name": %q, "can_admins_bypass": %t, "protection_rules": [%s], "deployment_branch_policy": %s}`,
		b.ID, b.Name, b.CanAdminsBypass, strings.Join(rules, ", "), policy)
}

// JSON は応答本文を返す。
func (b DeploymentBranchPoliciesBody) JSON() string {
	patterns := make([]string, len(b.Patterns))
	for i, p := range b.Patterns {
		patterns[i] = p.JSON()
	}
	return fmt.Sprintf(`{"total_count": %d, "branch_policies": [%s]}`, b.TotalCount, strings.Join(patterns, ", "))
}

// JSON は応答本文を返す。
func (b DeploymentPatternBody) JSON() string {
	return fmt.Sprintf(`{"id": %d, "name": %q, "type": %q}`, b.ID, b.Name, b.Type)
}

// securityStatus は security_and_analysis の項目の status を返す。
func securityStatus(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}
