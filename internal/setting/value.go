package setting

import (
	"slices"
	"strconv"
)

// Value は、一つの設定の値。期待値と観測値のどちらも、同じ設定についてはこの値で表す。
// リポジトリリソースそのものでは、存在すること (値を持つ種類の Variable と Ruleset では値を伴う) と存在しないことも、この値で表す。
// GitHub が付ける ID は含めない (ドメインモデルのリポジトリリソースの同一性)。
// 種別の違う設定の値を一つの並びで扱うため、値の種別を持つ。
// ゼロ値は値がないことを表す。
// 種別ごとに使わないフィールドをゼロ値に保つため、フィールドを公開せず、コンストラクタでだけ作る。
// 二つの値が同じかどうかは Equal で判定する。許可パターンのスライスを持つため、== はコンパイルエラーになる。
type Value struct {
	kind            valueKind
	boolean         bool
	level           PermissionLevel
	policy          AllowedActionsPolicy
	criteria        SelectedActionsCriteria
	accessLevel     AccessLevel
	approvalPolicy  ForkPullRequestApprovalPolicy
	conditions      PrivateForkPullRequestWorkflowConditions
	days            int
	minutes         int
	customization   OIDCSubjectCustomization
	squashDefault   SquashMergeCommitDefault
	mergeDefault    MergeCommitDefault
	branchPolicy    DeploymentBranchPolicy
	reviewerSet     ReviewerSet
	securityUpdates DependabotSecurityUpdatesState
	variable        string
	ruleset         Ruleset
}

type valueKind int

const (
	kindNone valueKind = iota
	kindBool
	kindPermissionLevel
	kindAllowedActionsPolicy
	kindSelectedActionsCriteria
	kindAccessLevel
	kindForkPullRequestApprovalPolicy
	kindPrivateForkPullRequestWorkflowConditions
	kindRetentionDays
	kindWaitTimer
	kindOIDCSubjectCustomization
	kindSquashMergeCommitDefault
	kindMergeCommitDefault
	kindDeploymentBranchPolicy
	kindReviewerSet
	kindDependabotSecurityUpdates
	kindVariable
	kindRuleset
	kindPresent
	kindAbsent
)

// BoolValue は、真偽値の設定の値を作る。
func BoolValue(b bool) Value {
	return Value{kind: kindBool, boolean: b}
}

// PermissionLevelValue は、ワークフローの GITHUB_TOKEN の既定権限の値を作る。
func PermissionLevelValue(level PermissionLevel) Value {
	return Value{kind: kindPermissionLevel, level: level}
}

// AllowedActionsPolicyValue は、Actions 許可ポリシーの値を作る。
func AllowedActionsPolicyValue(policy AllowedActionsPolicy) Value {
	return Value{kind: kindAllowedActionsPolicy, policy: policy}
}

// SelectedActionsCriteriaValue は、選択方式の Actions 許可条件の値を作る。
// 呼び出し側があとで書き換えても値が変わらないように、許可パターンを複製して持つ。
func SelectedActionsCriteriaValue(criteria SelectedActionsCriteria) Value {
	criteria.PatternsAllowed = slices.Clone(criteria.PatternsAllowed)
	return Value{kind: kindSelectedActionsCriteria, criteria: criteria}
}

// AccessLevelValue は、リポジトリ外のワークフローからのアクセスの範囲の値を作る。
func AccessLevelValue(level AccessLevel) Value {
	return Value{kind: kindAccessLevel, accessLevel: level}
}

// ForkPullRequestApprovalPolicyValue は、Fork Pull Request ワークフローの承認ポリシーの値を作る。
func ForkPullRequestApprovalPolicyValue(policy ForkPullRequestApprovalPolicy) Value {
	return Value{kind: kindForkPullRequestApprovalPolicy, approvalPolicy: policy}
}

// PrivateForkPullRequestWorkflowConditionsValue は、非公開リポジトリの Fork Pull Request ワークフローの実行条件の値を作る。
func PrivateForkPullRequestWorkflowConditionsValue(conditions PrivateForkPullRequestWorkflowConditions) Value {
	return Value{kind: kindPrivateForkPullRequestWorkflowConditions, conditions: conditions}
}

// RetentionDaysValue は、保持日数の値を作る。範囲の外の日数も観測値としてあり得るため、範囲を確かめない。
func RetentionDaysValue(days int) Value {
	return Value{kind: kindRetentionDays, days: days}
}

// WaitTimerValue は、Environment のデプロイの待機時間を分の数で表す値を作る。
func WaitTimerValue(minutes int) Value {
	return Value{kind: kindWaitTimer, minutes: minutes}
}

// OIDCSubjectCustomizationValue は、OIDC トークンの subject クレームのカスタマイズの値を作る。
// 呼び出し側があとで書き換えても値が変わらないように、key の並びと UseImmutableSubject の指す先を複製して持つ。並びは整列しない。
func OIDCSubjectCustomizationValue(c OIDCSubjectCustomization) Value {
	return Value{kind: kindOIDCSubjectCustomization, customization: c.clone()}
}

// SquashMergeCommitDefaultValue は、squash merge のマージメッセージの値を作る。観測値は GitHub が更新で受け付けない組にもなるため、組を確かめない。
func SquashMergeCommitDefaultValue(d SquashMergeCommitDefault) Value {
	return Value{kind: kindSquashMergeCommitDefault, squashDefault: d}
}

// MergeCommitDefaultValue は、merge commit のマージメッセージの値を作る。観測値は GitHub が更新で受け付けない組にもなるため、組を確かめない。
func MergeCommitDefaultValue(d MergeCommitDefault) Value {
	return Value{kind: kindMergeCommitDefault, mergeDefault: d}
}

// DeploymentBranchPolicyValue は、Environment のデプロイ元制限の値を作る。
// 呼び出し側があとで書き換えても値が変わらないように、個別パターンを複製して持つ。
func DeploymentBranchPolicyValue(policy DeploymentBranchPolicy) Value {
	policy.Patterns = slices.Clone(policy.Patterns)
	return Value{kind: kindDeploymentBranchPolicy, branchPolicy: policy}
}

// ReviewerSetValue は、Environment の担当者の集合の値を作る。
func ReviewerSetValue(s ReviewerSet) Value {
	return Value{kind: kindReviewerSet, reviewerSet: s}
}

// DependabotSecurityUpdatesValue は、Dependabot security updates の値を作る。
// 未定義の値は Enabled が偽を返して無効と同じ値に見えるため、確かめていない値を無効として通さないよう、不変条件の破れとして panic する。
func DependabotSecurityUpdatesValue(state DependabotSecurityUpdatesState) Value {
	if !state.valid() {
		panic("setting: undefined dependabot security updates value")
	}
	return Value{kind: kindDependabotSecurityUpdates, securityUpdates: state}
}

// VariableValue は、Variable が存在し、値が value であることを表す値を作る。
func VariableValue(value string) Value {
	return Value{kind: kindVariable, variable: value}
}

// RulesetValue は、Ruleset が存在し、値が r であることを表す値を作る。
// 呼び出し側があとで書き換えても値が変わらないように、スライスと ref_name の条件を複製して持つ。
func RulesetValue(r Ruleset) Value {
	return Value{kind: kindRuleset, ruleset: r.clone()}
}

// Present は、値を持たない種類のリポジトリリソース (Environment) が存在することを表す値を作る。
func Present() Value {
	return Value{kind: kindPresent}
}

// Absent は、リポジトリリソースが存在しないことを表す値を作る。値が空文字列で存在する VariableValue("") とは別の値である。
func Absent() Value {
	return Value{kind: kindAbsent}
}

// Equal は、二つの値が設定の意味で同じ値かどうかを返す。
// 値のないゼロ値は設定の値ではなく、比べようとするのは不変条件の破れのため panic する。
// 同じ値として扱うと、確かめていない値が一致や変更前提の成立として黙って通るため。
// 種別ごとに使わないフィールドはゼロ値のため、種別で分岐せずに全フィールドを比べる。
// 設定グループの省略できる項目は、両方の値が持つときだけ比べる。期待値が省略した項目の違いで照合、変更前提、期待状態の確認を止めないため。
// そのため項目を省略した値との比較は推移的でなく、すべての項目を比べるのは、項目を必ず持つ観測値どうしの比較に限る。
func (v Value) Equal(other Value) bool {
	if v.kind == kindNone || other.kind == kindNone {
		panic("setting: no value to compare")
	}
	return v.kind == other.kind &&
		v.boolean == other.boolean &&
		v.level == other.level &&
		v.policy == other.policy &&
		v.criteria.equal(other.criteria) &&
		v.accessLevel == other.accessLevel &&
		v.approvalPolicy == other.approvalPolicy &&
		v.conditions == other.conditions &&
		v.days == other.days &&
		v.minutes == other.minutes &&
		v.customization.equal(other.customization) &&
		v.squashDefault == other.squashDefault &&
		v.mergeDefault == other.mergeDefault &&
		v.branchPolicy.equal(other.branchPolicy) &&
		v.reviewerSet.equal(other.reviewerSet) &&
		v.securityUpdates.Enabled() == other.securityUpdates.Enabled() &&
		v.variable == other.variable &&
		v.ruleset.equal(other.ruleset)
}

// String は、表示に使う表記で値を返す。値のないゼロ値を表示しようとするのは不変条件の破れのため panic する。
// fmt は %s に渡した値の String の panic を回復して出力に埋めるため、呼び出し側は String を明示的に呼ぶ。
func (v Value) String() string {
	switch v.kind {
	case kindBool:
		return strconv.FormatBool(v.boolean)
	case kindPermissionLevel:
		return string(v.level)
	case kindAllowedActionsPolicy:
		return string(v.policy)
	case kindSelectedActionsCriteria:
		return "github_owned_allowed: " + strconv.FormatBool(v.criteria.GitHubOwnedAllowed) +
			", verified_allowed: " + strconv.FormatBool(v.criteria.VerifiedAllowed) +
			", patterns_allowed: " + v.criteria.PatternsAllowedString()
	case kindAccessLevel:
		return string(v.accessLevel)
	case kindForkPullRequestApprovalPolicy:
		return string(v.approvalPolicy)
	case kindPrivateForkPullRequestWorkflowConditions:
		return "run_workflows_from_fork_pull_requests: " + strconv.FormatBool(v.conditions.RunWorkflowsFromForkPullRequests) +
			", send_write_tokens_to_workflows: " + strconv.FormatBool(v.conditions.SendWriteTokensToWorkflows) +
			", send_secrets_and_variables: " + strconv.FormatBool(v.conditions.SendSecretsAndVariables) +
			", require_approval_for_fork_pr_workflows: " + strconv.FormatBool(v.conditions.RequireApprovalForForkPullRequestWorkflows)
	case kindRetentionDays:
		return strconv.Itoa(v.days)
	case kindWaitTimer:
		// 単位を添えて、分の数だと表示から分かるようにする
		if v.minutes == 1 {
			return "1 minute"
		}
		return strconv.Itoa(v.minutes) + " minutes"
	case kindOIDCSubjectCustomization:
		return v.customization.format()
	case kindSquashMergeCommitDefault:
		return "title: " + string(v.squashDefault.Title) + ", message: " + string(v.squashDefault.Message)
	case kindMergeCommitDefault:
		return "title: " + string(v.mergeDefault.Title) + ", message: " + string(v.mergeDefault.Message)
	case kindDeploymentBranchPolicy:
		if v.branchPolicy.Mode == DeploymentBranchPolicySelected {
			return string(v.branchPolicy.Mode) + " " + v.branchPolicy.PatternsString()
		}
		return string(v.branchPolicy.Mode)
	case kindReviewerSet:
		return v.reviewerSet.String()
	case kindDependabotSecurityUpdates:
		if v.securityUpdates == DependabotSecurityUpdatesPaused {
			return "true (paused by GitHub)"
		}
		return strconv.FormatBool(v.securityUpdates.Enabled())
	case kindVariable:
		// 値の改行や制御文字が変更計画の行を偽装しないよう、エスケープした1行で示す。引用符で、存在の present と absent と見分けられる
		return strconv.Quote(v.variable)
	case kindPresent:
		return "present"
	case kindAbsent:
		return "absent"
	case kindRuleset:
		// 1行の表記を持たない。変更計画の表示は値の差から行を組み立てる
		panic("setting: ruleset value has no one-line format")
	case kindNone:
	}
	panic("setting: no value to format")
}

// Bool は、真偽値の設定の値を返す。真偽値でない値は不変条件の破れのため panic する。
func (v Value) Bool() bool {
	if v.kind != kindBool {
		panic("setting: value is not a boolean")
	}
	return v.boolean
}

// PermissionLevel は、ワークフローの GITHUB_TOKEN の既定権限の値を返す。それ以外の値は不変条件の破れのため panic する。
func (v Value) PermissionLevel() PermissionLevel {
	if v.kind != kindPermissionLevel {
		panic("setting: value is not a permission level")
	}
	return v.level
}

// AllowedActionsPolicy は、Actions 許可ポリシーの値を返す。それ以外の値は不変条件の破れのため panic する。
func (v Value) AllowedActionsPolicy() AllowedActionsPolicy {
	if v.kind != kindAllowedActionsPolicy {
		panic("setting: value is not an allowed actions policy")
	}
	return v.policy
}

// SelectedActionsCriteria は、選択方式の Actions 許可条件の値を返す。それ以外の値は不変条件の破れのため panic する。
// 戻り値の書き換えが値に及ばないように、許可パターンは複製を返す。
func (v Value) SelectedActionsCriteria() SelectedActionsCriteria {
	if v.kind != kindSelectedActionsCriteria {
		panic("setting: value is not selected actions criteria")
	}
	criteria := v.criteria
	criteria.PatternsAllowed = slices.Clone(criteria.PatternsAllowed)
	return criteria
}

// AccessLevel は、リポジトリ外のワークフローからのアクセスの範囲の値を返す。それ以外の値は不変条件の破れのため panic する。
func (v Value) AccessLevel() AccessLevel {
	if v.kind != kindAccessLevel {
		panic("setting: value is not an access level")
	}
	return v.accessLevel
}

// ForkPullRequestApprovalPolicy は、Fork Pull Request ワークフローの承認ポリシーの値を返す。それ以外の値は不変条件の破れのため panic する。
func (v Value) ForkPullRequestApprovalPolicy() ForkPullRequestApprovalPolicy {
	if v.kind != kindForkPullRequestApprovalPolicy {
		panic("setting: value is not a fork pull request approval policy")
	}
	return v.approvalPolicy
}

// PrivateForkPullRequestWorkflowConditions は、非公開リポジトリの Fork Pull Request ワークフローの実行条件の値を返す。それ以外の値は不変条件の破れのため panic する。
func (v Value) PrivateForkPullRequestWorkflowConditions() PrivateForkPullRequestWorkflowConditions {
	if v.kind != kindPrivateForkPullRequestWorkflowConditions {
		panic("setting: value is not private fork pull request workflow conditions")
	}
	return v.conditions
}

// RetentionDays は、保持日数の値を返す。それ以外の値は不変条件の破れのため panic する。
func (v Value) RetentionDays() int {
	if v.kind != kindRetentionDays {
		panic("setting: value is not retention days")
	}
	return v.days
}

// WaitTimer は、デプロイの待機時間の分の数を返す。それ以外の値は不変条件の破れのため panic する。
func (v Value) WaitTimer() int {
	if v.kind != kindWaitTimer {
		panic("setting: value is not a wait timer")
	}
	return v.minutes
}

// OIDCSubjectCustomization は、OIDC トークンの subject クレームのカスタマイズの値を返す。それ以外の値は不変条件の破れのため panic する。
// 戻り値の書き換えが値に及ばないように、複製を返す。
func (v Value) OIDCSubjectCustomization() OIDCSubjectCustomization {
	if v.kind != kindOIDCSubjectCustomization {
		panic("setting: value is not an oidc subject customization")
	}
	return v.customization.clone()
}

// SquashMergeCommitDefault は、squash merge のマージメッセージの値を返す。それ以外の値は不変条件の破れのため panic する。
func (v Value) SquashMergeCommitDefault() SquashMergeCommitDefault {
	if v.kind != kindSquashMergeCommitDefault {
		panic("setting: value is not a squash merge commit default")
	}
	return v.squashDefault
}

// MergeCommitDefault は、merge commit のマージメッセージの値を返す。それ以外の値は不変条件の破れのため panic する。
func (v Value) MergeCommitDefault() MergeCommitDefault {
	if v.kind != kindMergeCommitDefault {
		panic("setting: value is not a merge commit default")
	}
	return v.mergeDefault
}

// WithItemsDeclaredIn は、declared が省略した設定グループの項目を v から除いた値を返す。省略した項目を変更計画に示さず、変更前提でも比べないようにするため。
// 種別ごとに使わないフィールドはゼロ値のため、種別で分岐しない。省略しても GitHub が値を保持する項目だけを除き、ほかの種別の値はそのまま返る。
func (v Value) WithItemsDeclaredIn(declared Value) Value {
	if declared.customization.UseImmutableSubject == nil {
		v.customization.UseImmutableSubject = nil
	}
	return v
}

// DeploymentBranchPolicy は、Environment のデプロイ元制限の値を返す。それ以外の値は不変条件の破れのため panic する。
// 戻り値の書き換えが値に及ばないように、個別パターンは複製を返す。
func (v Value) DeploymentBranchPolicy() DeploymentBranchPolicy {
	if v.kind != kindDeploymentBranchPolicy {
		panic("setting: value is not a deployment branch policy")
	}
	policy := v.branchPolicy
	policy.Patterns = slices.Clone(policy.Patterns)
	return policy
}

// ReviewerSet は、Environment の担当者の集合の値を返す。それ以外の値は不変条件の破れのため panic する。
func (v Value) ReviewerSet() ReviewerSet {
	if v.kind != kindReviewerSet {
		panic("setting: value is not a reviewer set")
	}
	return v.reviewerSet
}

// DependabotSecurityUpdates は、Dependabot security updates の値を返す。それ以外の値は不変条件の破れのため panic する。
func (v Value) DependabotSecurityUpdates() DependabotSecurityUpdatesState {
	if v.kind != kindDependabotSecurityUpdates {
		panic("setting: value is not dependabot security updates")
	}
	return v.securityUpdates
}

// VariableValue は、存在する Variable の値を返す。それ以外の値は不変条件の破れのため panic する。
// 存在しない Variable で空文字列を返すと、空の値を送る更新要求に化けるため。
func (v Value) VariableValue() string {
	if v.kind != kindVariable {
		panic("setting: value is not an existing variable")
	}
	return v.variable
}

// Ruleset は、存在する Ruleset の値を返す。それ以外の値は不変条件の破れのため panic する。
// 戻り値の書き換えが値に及ばないように、複製を返す。
func (v Value) Ruleset() Ruleset {
	if v.kind != kindRuleset {
		panic("setting: value is not an existing ruleset")
	}
	return v.ruleset.clone()
}
