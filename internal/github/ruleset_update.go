package github

import (
	"context"
	"fmt"
	"net/http"

	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

// CreateRuleset は、reference のリポジトリに、名前が name で値が value の Ruleset を作る POST を一度だけ送る。再送はしない。
// 応答の本文の扱いは Rulesets の設計の更新方式に従い、更新の契約を確かめるためだけに読む。
func (c *Client) CreateRuleset(ctx context.Context, reference repository.Reference, name setting.RulesetName, value setting.Ruleset) (UpdateOutcome, error) {
	status, data, err := c.send(ctx, http.MethodPost, c.rulesetsURL(reference), encodeRulesetCreation(name, value), http.StatusCreated)
	if err == nil {
		_, err = decodeRuleset(data)
	}
	return describeUpdateOutcome(fmt.Sprintf("create ruleset %q", name.String()), reference, status, err)
}

// UpdateRulesetValue は、reference のリポジトリの、ID が id の Ruleset の値を value にする PUT を一度だけ送る。再送はしない。
// 宛先は呼び出し側が承認時に予定した ID で、name は本文に入れず、エラーの文言にだけ使う。
// 応答の本文の扱いは CreateRuleset と同じで、更新の契約を確かめるためだけに読む。
func (c *Client) UpdateRulesetValue(ctx context.Context, reference repository.Reference, id RulesetID, name setting.RulesetName, value setting.Ruleset) (UpdateOutcome, error) {
	status, data, err := c.send(ctx, http.MethodPut, c.rulesetURL(reference, id), encodeRulesetValueUpdate(name, value), http.StatusOK)
	if err == nil {
		_, err = decodeRuleset(data)
	}
	return describeUpdateOutcome(fmt.Sprintf("update ruleset %q", name.String()), reference, status, err)
}

// rulesetCreationBody は、作成の POST の本文。名前と、値の全体を送る。
type rulesetCreationBody struct {
	Name string `json:"name"`
	rulesetValueBody
}

// rulesetValueBody は、Ruleset の値の全体の本文。トップレベルのフィールドは省略せずにすべて送る (Rulesets の設計の更新方式)。
type rulesetValueBody struct {
	Target       string                `json:"target"`
	Enforcement  string                `json:"enforcement"`
	Conditions   rulesetConditionsBody `json:"conditions"`
	BypassActors []bypassActorBody     `json:"bypass_actors"`
	Rules        []rulesetRuleBody     `json:"rules"`
}

type rulesetConditionsBody struct {
	RefName refNameConditionBody `json:"ref_name"`
}

type refNameConditionBody struct {
	Include []string `json:"include"`
	Exclude []string `json:"exclude"`
}

// bypassActorBody の ActorID は、DeployKey では null を送るためポインタで持つ。
type bypassActorBody struct {
	ActorType  string `json:"actor_type"`
	ActorID    *int64 `json:"actor_id"`
	BypassMode string `json:"bypass_mode"`
}

// rulesetRuleBody の Parameters は、型ごとの本文の struct を持つ。パラメータを持たない型ではキーごと送らないため、omitempty で nil のままにする。
// nil のポインタを入れると、interface の値が nil でなくなり omitempty が効かないため、代入しない。
type rulesetRuleBody struct {
	Type       string `json:"type"`
	Parameters any    `json:"parameters,omitempty"`
}

type pullRequestParametersBody struct {
	RequiredApprovingReviewCount   int  `json:"required_approving_review_count"`
	DismissStaleReviewsOnPush      bool `json:"dismiss_stale_reviews_on_push"`
	RequireCodeOwnerReview         bool `json:"require_code_owner_review"`
	RequireLastPushApproval        bool `json:"require_last_push_approval"`
	RequiredReviewThreadResolution bool `json:"required_review_thread_resolution"`
}

// requiredStatusChecksParametersBody の DoNotEnforceOnCreate は、宣言したときだけ送るため omitempty のポインタで持つ。false を指すポインタは省かれない。
type requiredStatusChecksParametersBody struct {
	RequiredStatusChecks             []statusCheckBody `json:"required_status_checks"`
	StrictRequiredStatusChecksPolicy bool              `json:"strict_required_status_checks_policy"`
	DoNotEnforceOnCreate             *bool             `json:"do_not_enforce_on_create,omitempty"`
}

// statusCheckBody の IntegrationID は、発行元を限らないチェック (0) では送らない。GitHub は 0 を拒否するため。
type statusCheckBody struct {
	Context       string `json:"context"`
	IntegrationID int64  `json:"integration_id,omitempty"`
}

// encodeRulesetCreation は、名前 name と値 value を作成の POST の本文にする。
func encodeRulesetCreation(name setting.RulesetName, value setting.Ruleset) []byte {
	return mustMarshalJSON(rulesetCreationBody{Name: name.String(), rulesetValueBody: newRulesetValueBody(name, value)})
}

// encodeRulesetValueUpdate は、値 value を値の変更の PUT の本文にする。名前 name は本文に入れず、panic の文言にだけ使う。
func encodeRulesetValueUpdate(name setting.RulesetName, value setting.Ruleset) []byte {
	return mustMarshalJSON(newRulesetValueBody(name, value))
}

// newRulesetValueBody は、値 value を本文の値の全体にする。
// 空の集合は null ではなく [] で送る。null の扱いが公開資料にないため。
// ref_name を持たない値は宣言から作られず、その送信の挙動も確かめていないため、補わずに不変条件の破れとして panic する。
func newRulesetValueBody(name setting.RulesetName, value setting.Ruleset) rulesetValueBody {
	if value.Conditions.RefName == nil {
		panic(fmt.Sprintf("github: ruleset %q to send has no ref_name condition", name.String()))
	}
	actors := make([]bypassActorBody, len(value.BypassActors))
	for i, a := range value.BypassActors {
		actors[i] = bypassActorBody{ActorType: string(a.ActorType), BypassMode: string(a.BypassMode)}
		if a.ActorType != setting.BypassActorDeployKey {
			actors[i].ActorID = &a.ActorID
		}
	}
	rules := make([]rulesetRuleBody, len(value.Rules))
	for i, r := range value.Rules {
		rules[i] = encodeRulesetRule(r)
	}
	return rulesetValueBody{
		Target:      string(value.Target),
		Enforcement: string(value.Enforcement),
		Conditions: rulesetConditionsBody{RefName: refNameConditionBody{
			Include: nonNil(value.Conditions.RefName.Include),
			Exclude: nonNil(value.Conditions.RefName.Exclude),
		}},
		BypassActors: actors,
		Rules:        rules,
	}
}

// encodeRulesetRule は、ルール rule を本文の要素にする。
// 定数を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
func encodeRulesetRule(rule setting.RulesetRule) rulesetRuleBody {
	switch rule.Type() {
	case setting.RulesetRulePullRequest:
		p := rule.PullRequest()
		return rulesetRuleBody{Type: string(rule.Type()), Parameters: &pullRequestParametersBody{
			RequiredApprovingReviewCount:   p.RequiredApprovingReviewCount,
			DismissStaleReviewsOnPush:      p.DismissStaleReviewsOnPush,
			RequireCodeOwnerReview:         p.RequireCodeOwnerReview,
			RequireLastPushApproval:        p.RequireLastPushApproval,
			RequiredReviewThreadResolution: p.RequiredReviewThreadResolution,
		}}
	case setting.RulesetRuleDeletion, setting.RulesetRuleNonFastForward:
		return rulesetRuleBody{Type: string(rule.Type())}
	case setting.RulesetRuleRequiredStatusChecks:
		p := rule.RequiredStatusChecks()
		checks := make([]statusCheckBody, len(p.RequiredStatusChecks))
		for i, c := range p.RequiredStatusChecks {
			checks[i] = statusCheckBody{Context: c.Context, IntegrationID: c.IntegrationID}
		}
		return rulesetRuleBody{Type: string(rule.Type()), Parameters: &requiredStatusChecksParametersBody{
			RequiredStatusChecks:             checks,
			StrictRequiredStatusChecksPolicy: p.StrictRequiredStatusChecksPolicy,
			DoNotEnforceOnCreate:             p.DoNotEnforceOnCreate,
		}}
	}
	panic(fmt.Sprintf("github: no request body for ruleset rule type %q", rule.Type()))
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
