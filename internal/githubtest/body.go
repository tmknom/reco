package githubtest

import (
	"encoding/json"
	"fmt"
	"strings"
)

// 応答本文は、各取得エンドポイントの応答のうち reco が読む項目だけを持つ。
// 値の組み合わせはテストごとに違うため、項目名だけをここに閉じ、値は各テストが構造体で書く。
// 文字列の値は %q で埋め込む。Go の引用は ASCII の印字可能な文字では JSON の文字列と同じ形になり、テストはその範囲の値だけを使う。
// 変数の名前と値だけは、制御文字や ASCII 以外の文字を含む値をテストに使うため、encoding/json で書く。

// RepositoryBody は、リポジトリの取得の応答本文である。
// Private のゼロ値は公開のリポジトリを、Admin のゼロ値は admin のアクセスを持たないトークンの所有者を表す。
// 3つのマージ方式のゼロ値はどれも無効で、GitHub が取り得ない組み合わせである。マージ方式の値を読む判定や表示を確かめるテストは、値を明示する。
// SecretScanning と SecretScanningPushProtection は、GitHub と同じく公開で admin のアクセスを持つときだけ security_and_analysis に含め、ゼロ値は disabled を表す。
type RepositoryBody struct {
	ID                           int64
	AllowAutoMerge               bool
	DeleteBranchOnMerge          bool
	AllowSquashMerge             bool
	AllowMergeCommit             bool
	AllowRebaseMerge             bool
	AllowUpdateBranch            bool
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

// VariablesBody は、変数の一覧の1ページの応答本文である。
// TotalCount は GitHub が返す全件数で、このページの要素数とは限らない。
type VariablesBody struct {
	TotalCount int
	Variables  []VariableBody
}

// VariableBody は、変数の一覧の要素である。
type VariableBody struct {
	Name  string
	Value string
}

// RulesetsBody は、Ruleset の一覧の応答本文である。GitHub と同じく total_count を持たない配列で返す。
type RulesetsBody []RulesetSummaryBody

// RulesetSummaryBody は、Ruleset の一覧の要素のうち、reco が読む項目である。
type RulesetSummaryBody struct {
	ID   int64
	Name string
}

// RulesetBody は、Ruleset の取得の応答本文である。conditions は ref_name に Include と Exclude を持つ形で返す。
// Rules はルールの要素の JSON で、型ごとに形が違い、解釈しない部分を持つ要素もテストに使うため、各テストが書く。
type RulesetBody struct {
	ID           int64
	Name         string
	Target       string
	Enforcement  string
	Include      []string
	Exclude      []string
	BypassActors []BypassActorBody
	Rules        []string
}

// BypassActorBody は、Ruleset の Bypass の要素である。ActorType が DeployKey の要素は、GitHub と同じく actor_id を null で返す。
type BypassActorBody struct {
	ActorType  string
	ActorID    int64
	BypassMode string
}

// PullRequestRuleBody は、pull_request のルールの要素である。GitHub と同じく、任意の3項目を既定値で返す。
type PullRequestRuleBody struct {
	RequiredApprovingReviewCount   int
	DismissStaleReviewsOnPush      bool
	RequireCodeOwnerReview         bool
	RequireLastPushApproval        bool
	RequiredReviewThreadResolution bool
}

// DeletionRuleBody は、deletion のルールの要素である。
const DeletionRuleBody = `{"type": "deletion"}`

// JSON は応答本文を返す。
func (b RepositoryBody) JSON() string {
	securityAndAnalysis := ""
	if !b.Private && b.Admin {
		securityAndAnalysis = fmt.Sprintf(`, "security_and_analysis": {"secret_scanning": {"status": %q}, "secret_scanning_push_protection": {"status": %q}}`,
			securityStatus(b.SecretScanning), securityStatus(b.SecretScanningPushProtection))
	}
	return fmt.Sprintf(`{"id": %d, "allow_auto_merge": %t, "delete_branch_on_merge": %t, `+
		`"allow_squash_merge": %t, "allow_merge_commit": %t, "allow_rebase_merge": %t, "allow_update_branch": %t, `+
		`"private": %t, "permissions": {"admin": %t}%s}`,
		b.ID, b.AllowAutoMerge, b.DeleteBranchOnMerge, b.AllowSquashMerge, b.AllowMergeCommit, b.AllowRebaseMerge, b.AllowUpdateBranch,
		b.Private, b.Admin, securityAndAnalysis)
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

// JSON は応答本文を返す。
func (b VariablesBody) JSON() string {
	variables := make([]string, len(b.Variables))
	for i, v := range b.Variables {
		variables[i] = v.JSON()
	}
	return fmt.Sprintf(`{"total_count": %d, "variables": [%s]}`, b.TotalCount, strings.Join(variables, ", "))
}

// JSON は変数の1件の応答本文を返す。
func (b VariableBody) JSON() string {
	return fmt.Sprintf(`{"name": %s, "value": %s}`, jsonString(b.Name), jsonString(b.Value))
}

// JSON は応答本文を返す。
func (b RulesetsBody) JSON() string {
	rulesets := make([]string, len(b))
	for i, r := range b {
		rulesets[i] = fmt.Sprintf(`{"id": %d, "name": %s}`, r.ID, jsonString(r.Name))
	}
	return "[" + strings.Join(rulesets, ", ") + "]"
}

// JSON は応答本文を返す。
func (b RulesetBody) JSON() string {
	actors := make([]string, len(b.BypassActors))
	for i, a := range b.BypassActors {
		actors[i] = a.JSON()
	}
	return fmt.Sprintf(`{"id": %d, "name": %s, "target": %q, "enforcement": %q, "conditions": {"ref_name": {"include": %s, "exclude": %s}}, "bypass_actors": [%s], "rules": [%s]}`,
		b.ID, jsonString(b.Name), b.Target, b.Enforcement, jsonStrings(b.Include), jsonStrings(b.Exclude), strings.Join(actors, ", "), strings.Join(b.Rules, ", "))
}

// JSON は Bypass の要素を返す。
func (b BypassActorBody) JSON() string {
	id := "null"
	if b.ActorType != "DeployKey" {
		id = fmt.Sprint(b.ActorID)
	}
	return fmt.Sprintf(`{"actor_type": %q, "actor_id": %s, "bypass_mode": %q}`, b.ActorType, id, b.BypassMode)
}

// JSON はルールの要素を返す。
func (b PullRequestRuleBody) JSON() string {
	return fmt.Sprintf(`{"type": "pull_request", "parameters": {"required_approving_review_count": %d, "dismiss_stale_reviews_on_push": %t, `+
		`"require_code_owner_review": %t, "require_last_push_approval": %t, "required_review_thread_resolution": %t, `+
		`"allowed_merge_methods": ["merge", "squash", "rebase"], "require_extra_approval_for_unattributed_changes": true, "required_reviewers": []}}`,
		b.RequiredApprovingReviewCount, b.DismissStaleReviewsOnPush, b.RequireCodeOwnerReview, b.RequireLastPushApproval, b.RequiredReviewThreadResolution)
}

// jsonStrings は ss を JSON の文字列の配列として返す。nil も空の配列にする。
func jsonStrings(ss []string) string {
	quoted := make([]string, len(ss))
	for i, s := range ss {
		quoted[i] = jsonString(s)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// jsonString は s を JSON の文字列として返す。
func jsonString(s string) string {
	// string の Marshal は失敗しない
	data, _ := json.Marshal(s)
	return string(data)
}

// securityStatus は security_and_analysis の項目の status を返す。
func securityStatus(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}
