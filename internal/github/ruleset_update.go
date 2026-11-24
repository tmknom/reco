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

// rulesetCreationBody は、作成の POST の本文。トップレベルのフィールドは省略せずにすべて送る (Rulesets の設計の更新方式)。
type rulesetCreationBody struct {
	Name         string                `json:"name"`
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

// rulesetRuleBody の Parameters は、deletion ではキーごと送らないため omitempty のポインタで持つ。
type rulesetRuleBody struct {
	Type       string                     `json:"type"`
	Parameters *pullRequestParametersBody `json:"parameters,omitempty"`
}

type pullRequestParametersBody struct {
	RequiredApprovingReviewCount   int  `json:"required_approving_review_count"`
	DismissStaleReviewsOnPush      bool `json:"dismiss_stale_reviews_on_push"`
	RequireCodeOwnerReview         bool `json:"require_code_owner_review"`
	RequireLastPushApproval        bool `json:"require_last_push_approval"`
	RequiredReviewThreadResolution bool `json:"required_review_thread_resolution"`
}

// encodeRulesetCreation は、名前 name と値 value を作成の POST の本文にする。
// 空の集合は null ではなく [] で送る。null の扱いが公開資料にないため。
// ref_name を持たない値は宣言から作られず、その POST の挙動も確かめていないため、補わずに不変条件の破れとして panic する。
func encodeRulesetCreation(name setting.RulesetName, value setting.Ruleset) []byte {
	if value.Conditions.RefName == nil {
		panic(fmt.Sprintf("github: ruleset %q to create has no ref_name condition", name.String()))
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
	return mustMarshalJSON(rulesetCreationBody{
		Name:        name.String(),
		Target:      string(value.Target),
		Enforcement: string(value.Enforcement),
		Conditions: rulesetConditionsBody{RefName: refNameConditionBody{
			Include: nonNil(value.Conditions.RefName.Include),
			Exclude: nonNil(value.Conditions.RefName.Exclude),
		}},
		BypassActors: actors,
		Rules:        rules,
	})
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
	case setting.RulesetRuleDeletion:
		return rulesetRuleBody{Type: string(rule.Type())}
	}
	panic(fmt.Sprintf("github: no request body for ruleset rule type %q", rule.Type()))
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
