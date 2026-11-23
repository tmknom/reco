package setting

import "fmt"

// Ruleset は、Ruleset の値。名前は同一性の基準で、値に含めない。
// 値は、Ruleset の値を変える更新が置き換える範囲の全体で、その全体を解釈できた Ruleset にだけ作る。
type Ruleset struct {
	Target      RulesetTarget
	Enforcement RulesetEnforcement
	Conditions  RulesetConditions
	// BypassActors と Rules は受け取った順のまま持つ。順は意味を持たないが、表示に使う。
	BypassActors []BypassActor
	Rules        []RulesetRule
}

// RulesetTarget は、Ruleset が効く対象の種類。値は GitHub REST API の文字列で、表示にもそのまま使う。
// reco が解釈するのは branch だけで、ほかの種類は値にしない。
type RulesetTarget string

const RulesetTargetBranch RulesetTarget = "branch"

// ParseRulesetTarget は s が定義された値に完全一致するときだけ、その値と真を返す。
func ParseRulesetTarget(s string) (RulesetTarget, bool) {
	if t := RulesetTarget(s); t == RulesetTargetBranch {
		return t, true
	}
	return "", false
}

// RulesetEnforcement は、Ruleset の強制の状態。値は GitHub REST API の文字列で、表示にもそのまま使う。
type RulesetEnforcement string

const (
	RulesetEnforcementDisabled RulesetEnforcement = "disabled"
	RulesetEnforcementActive   RulesetEnforcement = "active"
	// RulesetEnforcementEvaluate は宣言では書けないが、観測した値として意味が定まるため値にする。
	RulesetEnforcementEvaluate RulesetEnforcement = "evaluate"
)

// ParseRulesetEnforcement は s が定義された値に完全一致するときだけ、その値と真を返す。
func ParseRulesetEnforcement(s string) (RulesetEnforcement, bool) {
	switch e := RulesetEnforcement(s); e {
	case RulesetEnforcementDisabled, RulesetEnforcementActive, RulesetEnforcementEvaluate:
		return e, true
	default:
		return "", false
	}
}

// RulesetConditions は、Ruleset の対象の条件。
type RulesetConditions struct {
	// RefName は、ref_name を持たない条件（conditions: {}）では nil になる。空の include と exclude を持つ条件とは別の値である。
	RefName *RefNameCondition
}

// RefNameCondition は、Ruleset が効く ref を、含めるパターンと除くパターンで表す。
// パターンは受け取った順のまま、重複も残して持つ。
type RefNameCondition struct {
	Include []string
	Exclude []string
}

// BypassActor は、Ruleset のルールを迂回できる主体。
type BypassActor struct {
	ActorType BypassActorType
	// ActorID は、DeployKey では ID を持たないため 0 になる。
	ActorID    int64
	BypassMode BypassMode
}

// BypassActorType は、Bypass の主体の種類。値は GitHub REST API の文字列で、表示にもそのまま使う。
type BypassActorType string

const (
	BypassActorRepositoryRole BypassActorType = "RepositoryRole"
	BypassActorUser           BypassActorType = "User"
	BypassActorIntegration    BypassActorType = "Integration"
	BypassActorDeployKey      BypassActorType = "DeployKey"
)

// ParseBypassActorType は s が定義された値に完全一致するときだけ、その値と真を返す。
func ParseBypassActorType(s string) (BypassActorType, bool) {
	switch a := BypassActorType(s); a {
	case BypassActorRepositoryRole, BypassActorUser, BypassActorIntegration, BypassActorDeployKey:
		return a, true
	default:
		return "", false
	}
}

// BypassMode は、Bypass の方式。値は GitHub REST API の文字列で、表示にもそのまま使う。
type BypassMode string

const (
	BypassModeAlways      BypassMode = "always"
	BypassModePullRequest BypassMode = "pull_request"
	BypassModeExempt      BypassMode = "exempt"
)

// ParseBypassMode は s が定義された値に完全一致するときだけ、その値と真を返す。
func ParseBypassMode(s string) (BypassMode, bool) {
	switch m := BypassMode(s); m {
	case BypassModeAlways, BypassModePullRequest, BypassModeExempt:
		return m, true
	default:
		return "", false
	}
}

// RulesetRule は、Ruleset のルール。型に合わないパラメータを持たないよう、フィールドを公開せずコンストラクタでだけ作る。
// 比較演算子で比べられる値のまま保つ。
type RulesetRule struct {
	ruleType    RulesetRuleType
	pullRequest PullRequestParameters
}

// RulesetRuleType は、reco が解釈するルール型。値は GitHub REST API の文字列で、表示にもそのまま使う。
type RulesetRuleType string

const (
	RulesetRulePullRequest RulesetRuleType = "pull_request"
	RulesetRuleDeletion    RulesetRuleType = "deletion"
)

// ParseRulesetRuleType は s が reco の解釈するルール型に完全一致するときだけ、その型と真を返す。
func ParseRulesetRuleType(s string) (RulesetRuleType, bool) {
	switch r := RulesetRuleType(s); r {
	case RulesetRulePullRequest, RulesetRuleDeletion:
		return r, true
	default:
		return "", false
	}
}

// PullRequestParameters は、pull_request のルールのパラメータのうち、値に含める5項目。
// GitHub が既定値で返す任意の項目は、既定値のときだけ解釈できたとし、値に含めない。
type PullRequestParameters struct {
	RequiredApprovingReviewCount   int
	DismissStaleReviewsOnPush      bool
	RequireCodeOwnerReview         bool
	RequireLastPushApproval        bool
	RequiredReviewThreadResolution bool
}

// PullRequestRule は、パラメータ parameters を持つ pull_request のルールを作る。
func PullRequestRule(parameters PullRequestParameters) RulesetRule {
	return RulesetRule{ruleType: RulesetRulePullRequest, pullRequest: parameters}
}

// DeletionRule は、パラメータを持たない deletion のルールを作る。
func DeletionRule() RulesetRule {
	return RulesetRule{ruleType: RulesetRuleDeletion}
}

// Type はルール型を返す。
func (r RulesetRule) Type() RulesetRuleType {
	return r.ruleType
}

// PullRequest は pull_request のルールのパラメータを返す。ほかの型のルールでは、ゼロ値をパラメータとして読ませないよう panic する。
func (r RulesetRule) PullRequest() PullRequestParameters {
	if r.ruleType != RulesetRulePullRequest {
		panic(fmt.Sprintf("setting: rule %q has no pull_request parameters", r.ruleType))
	}
	return r.pullRequest
}
