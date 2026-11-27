package setting

import (
	"fmt"
	"slices"
	"strings"
)

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

// clone は、スライスと ref_name の条件を共有しない写しを返す。Value が持つ値を、作った側と読んだ側の書き換えから切り離すため。
func (r Ruleset) clone() Ruleset {
	if r.Conditions.RefName != nil {
		refName := RefNameCondition{Include: slices.Clone(r.Conditions.RefName.Include), Exclude: slices.Clone(r.Conditions.RefName.Exclude)}
		r.Conditions.RefName = &refName
	}
	r.BypassActors = slices.Clone(r.BypassActors)
	r.Rules = slices.Clone(r.Rules)
	return r
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

// SameActor は、a と other が同じ主体か (actor_type と actor_id が同じか) を返す。方式は比べない。
func (a BypassActor) SameActor(other BypassActor) bool {
	return a.ActorType == other.ActorType && a.ActorID == other.ActorID
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
// パラメータのスライスとポインタは、コンストラクタとアクセサが複製し、作った後に誰も書き換えられない。そのため、ルールの並びの浅い写しで Ruleset を複製できる。
type RulesetRule struct {
	ruleType     RulesetRuleType
	pullRequest  PullRequestParameters
	statusChecks RequiredStatusChecksParameters
}

// RulesetRuleType は、reco が解釈するルール型。値は GitHub REST API の文字列で、表示にもそのまま使う。
type RulesetRuleType string

const (
	RulesetRulePullRequest          RulesetRuleType = "pull_request"
	RulesetRuleDeletion             RulesetRuleType = "deletion"
	RulesetRuleRequiredStatusChecks RulesetRuleType = "required_status_checks"
	RulesetRuleNonFastForward       RulesetRuleType = "non_fast_forward"
)

// ParseRulesetRuleType は s が reco の解釈するルール型に完全一致するときだけ、その型と真を返す。
func ParseRulesetRuleType(s string) (RulesetRuleType, bool) {
	switch r := RulesetRuleType(s); r {
	case RulesetRulePullRequest, RulesetRuleDeletion, RulesetRuleRequiredStatusChecks, RulesetRuleNonFastForward:
		return r, true
	default:
		return "", false
	}
}

// HasParameters は、ルール型が parameters を持つかを返す。
// ルール型を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かず、定義にない型は panic する。
func (t RulesetRuleType) HasParameters() bool {
	switch t {
	case RulesetRulePullRequest, RulesetRuleRequiredStatusChecks:
		return true
	case RulesetRuleDeletion, RulesetRuleNonFastForward:
		return false
	}
	panic(fmt.Sprintf("setting: undefined ruleset rule type %q", t))
}

// PullRequestParameters は、pull_request のルールのパラメータのうち、値に含めるもの。
// 宣言できない required_reviewers は、既定値のときだけ解釈できたとし、値に含めない。
type PullRequestParameters struct {
	RequiredApprovingReviewCount   int
	DismissStaleReviewsOnPush      bool
	RequireCodeOwnerReview         bool
	RequireLastPushApproval        bool
	RequiredReviewThreadResolution bool
	// AllowedMergeMethods は省略できるパラメータで、nil は宣言が省略したことを表す。持つなら1件以上で、受け取った順のまま重複も残す。観測した値は必ず持つ。
	AllowedMergeMethods []AllowedMergeMethod
	// RequireExtraApprovalForUnattributedChanges は省略できるパラメータで、nil は宣言が省略したことを表す。観測した値は必ず持つ。
	RequireExtraApprovalForUnattributedChanges *bool
}

// clone は、スライスとポインタを共有しない写しを返す。省略を表す nil は nil のまま残す。
func (p PullRequestParameters) clone() PullRequestParameters {
	p.AllowedMergeMethods = slices.Clone(p.AllowedMergeMethods)
	if p.RequireExtraApprovalForUnattributedChanges != nil {
		p.RequireExtraApprovalForUnattributedChanges = new(*p.RequireExtraApprovalForUnattributedChanges)
	}
	return p
}

// AllowedMergeMethod は、pull_request のルールが許すマージ方式。値は GitHub REST API の文字列で、表示にもそのまま使う。
// General のマージ方式の設定（真偽値の管理の単位）とは別の概念である。
type AllowedMergeMethod string

const (
	AllowedMergeMethodMerge  AllowedMergeMethod = "merge"
	AllowedMergeMethodSquash AllowedMergeMethod = "squash"
	AllowedMergeMethodRebase AllowedMergeMethod = "rebase"
)

// ParseAllowedMergeMethod は s が定義された値に完全一致するときだけ、その値と真を返す。
func ParseAllowedMergeMethod(s string) (AllowedMergeMethod, bool) {
	switch m := AllowedMergeMethod(s); m {
	case AllowedMergeMethodMerge, AllowedMergeMethodSquash, AllowedMergeMethodRebase:
		return m, true
	default:
		return "", false
	}
}

// AllowedMergeMethodsText は、許すマージ方式の並びを、受け取った順のまま重複も残した表示の表記で返す。
// 要素は定義された値だけで、行を偽装する文字を含み得ないため、引用符で囲まない。
func AllowedMergeMethodsText(methods []AllowedMergeMethod) string {
	texts := make([]string, len(methods))
	for i, m := range methods {
		texts[i] = string(m)
	}
	return "[" + strings.Join(texts, ", ") + "]"
}

// PullRequestRule は、パラメータ parameters を持つ pull_request のルールを作る。
// nil でない空の AllowedMergeMethods と定義にない方式は、宣言も観測も作らないため、不変条件の破れとして panic する。
// 定義にない方式を通すと、引用符で囲まずに示す表示が変更計画の行を偽装し得る。
func PullRequestRule(parameters PullRequestParameters) RulesetRule {
	if parameters.AllowedMergeMethods != nil && len(parameters.AllowedMergeMethods) == 0 {
		panic("setting: pull_request rule with empty allowed_merge_methods")
	}
	for _, m := range parameters.AllowedMergeMethods {
		if _, ok := ParseAllowedMergeMethod(string(m)); !ok {
			panic(fmt.Sprintf("setting: pull_request rule with undefined merge method %q", m))
		}
	}
	return RulesetRule{ruleType: RulesetRulePullRequest, pullRequest: parameters.clone()}
}

// RequiredStatusChecksParameters は、required_status_checks のルールのパラメータ。
type RequiredStatusChecksParameters struct {
	// RequiredStatusChecks は受け取った順のまま、重複も残して持つ。順は意味を持たないが、表示に使う。
	RequiredStatusChecks             []StatusCheck
	StrictRequiredStatusChecksPolicy bool
	// DoNotEnforceOnCreate は省略できるパラメータで、nil は宣言が省略したことを表す。観測した値は必ず持つ。
	DoNotEnforceOnCreate *bool
}

// clone は、スライスとポインタを共有しない写しを返す。
func (p RequiredStatusChecksParameters) clone() RequiredStatusChecksParameters {
	p.RequiredStatusChecks = slices.Clone(p.RequiredStatusChecks)
	if p.DoNotEnforceOnCreate != nil {
		p.DoNotEnforceOnCreate = new(*p.DoNotEnforceOnCreate)
	}
	return p
}

// StatusCheck は、必須のステータスチェック。
type StatusCheck struct {
	Context string
	// IntegrationID は、発行元を限らないチェックでは 0 になる。GitHub は 0 を ID として受け付けないため、ID と取り違えない。
	IntegrationID int64
}

// DeletionRule は、パラメータを持たない deletion のルールを作る。
func DeletionRule() RulesetRule {
	return RulesetRule{ruleType: RulesetRuleDeletion}
}

// RequiredStatusChecksRule は、パラメータ parameters を持つ required_status_checks のルールを作る。
func RequiredStatusChecksRule(parameters RequiredStatusChecksParameters) RulesetRule {
	return RulesetRule{ruleType: RulesetRuleRequiredStatusChecks, statusChecks: parameters.clone()}
}

// NonFastForwardRule は、パラメータを持たない non_fast_forward のルールを作る。
func NonFastForwardRule() RulesetRule {
	return RulesetRule{ruleType: RulesetRuleNonFastForward}
}

// Type はルール型を返す。
func (r RulesetRule) Type() RulesetRuleType {
	return r.ruleType
}

// PullRequest は pull_request のルールのパラメータの写しを返す。ほかの型のルールでは、ゼロ値をパラメータとして読ませないよう panic する。
func (r RulesetRule) PullRequest() PullRequestParameters {
	if r.ruleType != RulesetRulePullRequest {
		panic(fmt.Sprintf("setting: rule %q has no pull_request parameters", r.ruleType))
	}
	return r.pullRequest.clone()
}

// RequiredStatusChecks は required_status_checks のルールのパラメータの写しを返す。ほかの型のルールでは、ゼロ値をパラメータとして読ませないよう panic する。
func (r RulesetRule) RequiredStatusChecks() RequiredStatusChecksParameters {
	if r.ruleType != RulesetRuleRequiredStatusChecks {
		panic(fmt.Sprintf("setting: rule %q has no required_status_checks parameters", r.ruleType))
	}
	return r.statusChecks.clone()
}
