package github

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/tmknom/reco/internal/setting"
)

// rulesetSummary は、Ruleset の一覧の要素のうち reco が読む ID と名前である。一覧の要素は値を持たないため、値は詳細で読む。
type rulesetSummary struct {
	id   RulesetID
	name string
}

// rulesetSummaryPayload は欠落と null を検出するため、各フィールドをポインタで受ける。
type rulesetSummaryPayload struct {
	ID   *int64  `json:"id"`
	Name *string `json:"name"`
}

// rulesetFields は、詳細の応答のうち取得契約に含めるトップレベルのフィールドを、欠落を報告する順に並べる。
var rulesetFields = []string{"id", "name", "target", "enforcement", "conditions", "bypass_actors", "rules"}

// decodeRulesetSummaries は Ruleset の一覧のすべての要素を、一覧の並びのまま読む。
// 詳細の要求を送る前に、すべての要素が取得契約に適合し、同じ名前が二つないことを確かめる（Rulesets の設計の読み取り要求）。
// 失敗の文言に Ruleset の名前を入れるのは、どの Ruleset で失敗したかを示す対象の識別子で、秘密情報ではないため。%q で制御文字をエスケープする。
func decodeRulesetSummaries(items []json.RawMessage) ([]rulesetSummary, error) {
	summaries := make([]rulesetSummary, len(items))
	seen := make(map[string]struct{}, len(items))
	for i, item := range items {
		var p rulesetSummaryPayload
		if err := json.Unmarshal(item, &p); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrUnexpectedResponse, err)
		}
		var missing []string
		if p.ID == nil {
			missing = append(missing, "id")
		}
		if p.Name == nil {
			missing = append(missing, "name")
		}
		if len(missing) > 0 {
			return nil, fmt.Errorf("%w: missing fields: %s", ErrUnexpectedResponse, strings.Join(missing, ", "))
		}
		if *p.Name == "" {
			return nil, fmt.Errorf("%w: unsupported value: name", ErrUnexpectedResponse)
		}
		key := simpleFoldKey(*p.Name)
		if _, duplicate := seen[key]; duplicate {
			return nil, fmt.Errorf("%w: duplicate ruleset name %q", ErrUnexpectedResponse, *p.Name)
		}
		seen[key] = struct{}{}
		summaries[i] = rulesetSummary{id: RulesetID(*p.ID), name: *p.Name}
	}
	return summaries, nil
}

// decodeRuleset は Ruleset の詳細の応答本文を、値か、解釈しなかったもののどちらかを持つ観測結果に変換する。
// どの形を取得契約に適合しない応答とし、何を解釈しない部分とし、どの順に集めるかは、Rulesets の設計の取得契約と解釈しない部分に従う。
// rulesetFields にないトップレベルのフィールドは読まない。取得契約に適合しない値は応答本文の一部のため、文言に入れない。
func decodeRuleset(data []byte) (Ruleset, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return Ruleset{}, fmt.Errorf("%w: %w", ErrUnexpectedResponse, err)
	}
	var missing []string
	for _, name := range rulesetFields {
		if isAbsent(fields[name]) {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return Ruleset{}, fmt.Errorf("%w: missing fields: %s", ErrUnexpectedResponse, strings.Join(missing, ", "))
	}
	id, err := decodeRequired[int64](fields["id"], "id")
	if err != nil {
		return Ruleset{}, err
	}
	name, err := decodeRequired[string](fields["name"], "name")
	if err != nil {
		return Ruleset{}, err
	}
	rawTarget, err := decodeRequired[string](fields["target"], "target")
	if err != nil {
		return Ruleset{}, err
	}
	rawEnforcement, err := decodeRequired[string](fields["enforcement"], "enforcement")
	if err != nil {
		return Ruleset{}, err
	}
	var parts []UninterpretedPart
	target, ok := setting.ParseRulesetTarget(rawTarget)
	if !ok {
		parts = append(parts, UninterpretedPart{Kind: UninterpretedTarget, Name: rawTarget})
	}
	enforcement, ok := setting.ParseRulesetEnforcement(rawEnforcement)
	if !ok {
		parts = append(parts, UninterpretedPart{Kind: UninterpretedEnforcement, Name: rawEnforcement})
	}
	conditions, conditionsParts, err := decodeRulesetConditions(fields["conditions"])
	if err != nil {
		return Ruleset{}, err
	}
	actors, actorsParts, err := decodeBypassActors(fields["bypass_actors"])
	if err != nil {
		return Ruleset{}, err
	}
	rules, rulesParts, err := decodeRulesetRules(fields["rules"])
	if err != nil {
		return Ruleset{}, err
	}
	parts = slices.Concat(parts, conditionsParts, actorsParts, rulesParts)
	if len(parts) > 0 {
		return UninterpretedRuleset(RulesetID(id), name, firstOfEachPart(parts)), nil
	}
	return InterpretedRuleset(RulesetID(id), name, setting.Ruleset{
		Target:       target,
		Enforcement:  enforcement,
		Conditions:   conditions,
		BypassActors: actors,
		Rules:        rules,
	}), nil
}

// decodeRulesetConditions は conditions を読む。解釈しなかったキーは名前の順に並べる。
func decodeRulesetConditions(raw json.RawMessage) (setting.RulesetConditions, []UninterpretedPart, error) {
	fields, err := decodeObject(raw, "conditions")
	if err != nil {
		return setting.RulesetConditions{}, nil, err
	}
	var conditions setting.RulesetConditions
	var parts []UninterpretedPart
	for _, key := range slices.Sorted(maps.Keys(fields)) {
		if key != "ref_name" {
			parts = append(parts, UninterpretedPart{Kind: UninterpretedConditionsKey, Name: key})
			continue
		}
		refName, refNameParts, err := decodeRefNameCondition(fields[key])
		if err != nil {
			return setting.RulesetConditions{}, nil, err
		}
		conditions.RefName = &refName
		parts = append(parts, refNameParts...)
	}
	return conditions, parts, nil
}

// decodeRefNameCondition は ref_name を読む。include と exclude 以外のキーは、ref_name の中のキーと分かる名前で持つ。
func decodeRefNameCondition(raw json.RawMessage) (setting.RefNameCondition, []UninterpretedPart, error) {
	fields, err := decodeObject(raw, "conditions.ref_name")
	if err != nil {
		return setting.RefNameCondition{}, nil, err
	}
	include, err := decodeRefNamePatterns(fields["include"], "conditions.ref_name.include")
	if err != nil {
		return setting.RefNameCondition{}, nil, err
	}
	exclude, err := decodeRefNamePatterns(fields["exclude"], "conditions.ref_name.exclude")
	if err != nil {
		return setting.RefNameCondition{}, nil, err
	}
	var parts []UninterpretedPart
	for _, key := range unreadKeys(fields, "include", "exclude") {
		parts = append(parts, UninterpretedPart{Kind: UninterpretedConditionsKey, Name: "ref_name." + key})
	}
	return setting.RefNameCondition{Include: include, Exclude: exclude}, parts, nil
}

// decodeRefNamePatterns は include か exclude のパターンを、受け取った順のまま重複も残して読む。
// 要素を *string で受けるのは、null の要素を空文字列のパターンと読まないため。
func decodeRefNamePatterns(raw json.RawMessage, path string) ([]string, error) {
	values, err := decodeRequired[[]*string](raw, path)
	if err != nil {
		return nil, err
	}
	patterns := make([]string, len(values))
	for i, v := range values {
		if v == nil {
			return nil, fmt.Errorf("%w: unsupported value: %s", ErrUnexpectedResponse, path)
		}
		patterns[i] = *v
	}
	return patterns, nil
}

// decodeBypassActors は bypass_actors の要素を応答の順に読む。解釈しない部分を持つ要素は値にしない。
func decodeBypassActors(raw json.RawMessage) ([]setting.BypassActor, []UninterpretedPart, error) {
	items, err := decodeRequired[[]json.RawMessage](raw, "bypass_actors")
	if err != nil {
		return nil, nil, err
	}
	actors := make([]setting.BypassActor, 0, len(items))
	var parts []UninterpretedPart
	for _, item := range items {
		actor, actorParts, err := decodeBypassActor(item)
		if err != nil {
			return nil, nil, err
		}
		if len(actorParts) > 0 {
			parts = append(parts, actorParts...)
			continue
		}
		actors = append(actors, actor)
	}
	return actors, parts, nil
}

// decodeBypassActor は Bypass の一つの要素を読む。
// 扱わない actor_type の要素は、ほかの項目の形が主体の種類ごとに違い得るため、ほかの項目を確かめない。
func decodeBypassActor(raw json.RawMessage) (setting.BypassActor, []UninterpretedPart, error) {
	fields, err := decodeObject(raw, "bypass_actors")
	if err != nil {
		return setting.BypassActor{}, nil, err
	}
	rawType, err := decodeRequired[string](fields["actor_type"], "bypass_actors.actor_type")
	if err != nil {
		return setting.BypassActor{}, nil, err
	}
	actorType, ok := setting.ParseBypassActorType(rawType)
	if !ok {
		return setting.BypassActor{}, []UninterpretedPart{{Kind: UninterpretedBypassActorType, Name: rawType}}, nil
	}
	actorID, err := decodeBypassActorID(actorType, fields["actor_id"])
	if err != nil {
		return setting.BypassActor{}, nil, err
	}
	rawMode, err := decodeRequired[string](fields["bypass_mode"], "bypass_actors.bypass_mode")
	if err != nil {
		return setting.BypassActor{}, nil, err
	}
	var parts []UninterpretedPart
	mode, ok := setting.ParseBypassMode(rawMode)
	if !ok {
		parts = append(parts, UninterpretedPart{Kind: UninterpretedBypassMode, Name: rawMode})
	}
	for _, key := range unreadKeys(fields, "actor_type", "actor_id", "bypass_mode") {
		parts = append(parts, UninterpretedPart{Kind: UninterpretedBypassActorKey, Name: key})
	}
	return setting.BypassActor{ActorType: actorType, ActorID: actorID, BypassMode: mode}, parts, nil
}

// decodeBypassActorID は主体の種類ごとに actor_id を読む。DeployKey は ID を持たないため、ID を 0 とする。
// 主体の種類を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
func decodeBypassActorID(actorType setting.BypassActorType, raw json.RawMessage) (int64, error) {
	switch actorType {
	case setting.BypassActorDeployKey:
		if !isAbsent(raw) {
			return 0, fmt.Errorf("%w: unsupported value: bypass_actors.actor_id", ErrUnexpectedResponse)
		}
		return 0, nil
	case setting.BypassActorRepositoryRole, setting.BypassActorUser, setting.BypassActorIntegration:
		return decodeRequired[int64](raw, "bypass_actors.actor_id")
	}
	panic(fmt.Sprintf("github: unknown bypass actor type %q", actorType))
}

// decodeRulesetRules は rules の要素を応答の順に読む。解釈しない部分を持つ要素は値にしない。
func decodeRulesetRules(raw json.RawMessage) ([]setting.RulesetRule, []UninterpretedPart, error) {
	items, err := decodeRequired[[]json.RawMessage](raw, "rules")
	if err != nil {
		return nil, nil, err
	}
	rules := make([]setting.RulesetRule, 0, len(items))
	var parts []UninterpretedPart
	for _, item := range items {
		rule, ruleParts, err := decodeRulesetRule(item)
		if err != nil {
			return nil, nil, err
		}
		if len(ruleParts) > 0 {
			parts = append(parts, ruleParts...)
			continue
		}
		rules = append(rules, rule)
	}
	return rules, parts, nil
}

// decodeRulesetRule はルールの一つの要素を読む。扱わないルール型の要素は、パラメータの形が型ごとに違うため、ほかの項目を確かめない。
func decodeRulesetRule(raw json.RawMessage) (setting.RulesetRule, []UninterpretedPart, error) {
	fields, err := decodeObject(raw, "rules")
	if err != nil {
		return setting.RulesetRule{}, nil, err
	}
	rawType, err := decodeRequired[string](fields["type"], "rules.type")
	if err != nil {
		return setting.RulesetRule{}, nil, err
	}
	ruleType, ok := setting.ParseRulesetRuleType(rawType)
	if !ok {
		return setting.RulesetRule{}, []UninterpretedPart{{Kind: UninterpretedRuleType, Name: rawType}}, nil
	}
	return decodeHandledRulesetRule(ruleType, fields)
}

// decodeHandledRulesetRule は、reco が解釈するルール型 ruleType の要素 fields を読む。
// ルール型を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
func decodeHandledRulesetRule(ruleType setting.RulesetRuleType, fields map[string]json.RawMessage) (setting.RulesetRule, []UninterpretedPart, error) {
	switch ruleType {
	case setting.RulesetRulePullRequest:
		return decodePullRequestRule(fields)
	case setting.RulesetRuleDeletion:
		return setting.DeletionRule(), ruleKeyParts(fields, ruleType, "type"), nil
	case setting.RulesetRuleNonFastForward:
		return setting.NonFastForwardRule(), ruleKeyParts(fields, ruleType, "type"), nil
	case setting.RulesetRuleRequiredStatusChecks:
		return decodeRequiredStatusChecksRule(fields)
	}
	panic(fmt.Sprintf("github: unknown ruleset rule type %q", ruleType))
}

// decodePullRequestRule は pull_request のルールを読む。GitHub の API で必須の5項目は、欠けると値を作れないため、欠落を契約外の応答とする。
// 宣言できる allowed_merge_methods と require_extra_approval_for_unattributed_changes は、読めないときだけ解釈しない部分とする（Rulesets の設計の解釈しない部分）。
func decodePullRequestRule(fields map[string]json.RawMessage) (setting.RulesetRule, []UninterpretedPart, error) {
	if isAbsent(fields["parameters"]) {
		return setting.RulesetRule{}, nil, fmt.Errorf("%w: missing fields: rules.parameters", ErrUnexpectedResponse)
	}
	parameters, err := decodeObject(fields["parameters"], "rules.parameters")
	if err != nil {
		return setting.RulesetRule{}, nil, err
	}
	count, err := decodeRequired[int](parameters["required_approving_review_count"], "rules.parameters.required_approving_review_count")
	if err != nil {
		return setting.RulesetRule{}, nil, err
	}
	var flags [4]bool
	for i, name := range []string{"dismiss_stale_reviews_on_push", "require_code_owner_review", "require_last_push_approval", "required_review_thread_resolution"} {
		if flags[i], err = decodeRequired[bool](parameters[name], "rules.parameters."+name); err != nil {
			return setting.RulesetRule{}, nil, err
		}
	}
	var unread []string
	methods, ok := decodeAllowedMergeMethods(parameters["allowed_merge_methods"])
	if !ok {
		unread = append(unread, "allowed_merge_methods")
	}
	extraApproval, ok := decodeOptional[bool](parameters["require_extra_approval_for_unattributed_changes"])
	if !ok {
		unread = append(unread, "require_extra_approval_for_unattributed_changes")
	}
	// 読めなかった allowed_merge_methods は方式を持たず、ルールを作ると PullRequestRule の不変条件を破るため、解釈しない部分があればルールを作らない
	parts := slices.Concat(ruleKeyParts(fields, setting.RulesetRulePullRequest, "type", "parameters"), pullRequestParameterParts(parameters, unread))
	if len(parts) > 0 {
		return setting.RulesetRule{}, parts, nil
	}
	return setting.PullRequestRule(setting.PullRequestParameters{
		RequiredApprovingReviewCount:               count,
		DismissStaleReviewsOnPush:                  flags[0],
		RequireCodeOwnerReview:                     flags[1],
		RequireLastPushApproval:                    flags[2],
		RequiredReviewThreadResolution:             flags[3],
		AllowedMergeMethods:                        methods,
		RequireExtraApprovalForUnattributedChanges: extraApproval,
	}), nil, nil
}

// pullRequestParameterParts は、pull_request のパラメータのうち解釈しないものを、読めなかったパラメータ unread、required_reviewers、ほかのキーの名前の順で返す。
// 宣言できない required_reviewers を既定値のときだけ解釈できたとする理由は、Rulesets の設計の解釈しない部分にある。
func pullRequestParameterParts(parameters map[string]json.RawMessage, unread []string) []UninterpretedPart {
	// 読めない値はゼロ値の空の配列になり、既定値の [] と区別できないため、読めたかも確かめる
	if reviewers, ok := decodeOptional[[]json.RawMessage](parameters["required_reviewers"]); !ok || len(reviewers) > 0 {
		unread = append(unread, "required_reviewers")
	}
	unread = append(unread, unreadKeys(parameters,
		"required_approving_review_count", "dismiss_stale_reviews_on_push", "require_code_owner_review", "require_last_push_approval", "required_review_thread_resolution",
		"allowed_merge_methods", "require_extra_approval_for_unattributed_changes", "required_reviewers")...)
	parts := make([]UninterpretedPart, len(unread))
	for i, name := range unread {
		parts[i] = UninterpretedPart{Kind: UninterpretedRuleParameter, Rule: setting.RulesetRulePullRequest, Name: name}
	}
	return parts
}

// decodeRequiredStatusChecksRule は required_status_checks のルールを読む。GitHub の API で必須の項目は、欠けると値を作れないため、欠落を契約外の応答とする。
// 宣言できる do_not_enforce_on_create は、どちらの値も値に含め、読めないときだけ解釈しない部分とする（Rulesets の設計の解釈しない部分）。
func decodeRequiredStatusChecksRule(fields map[string]json.RawMessage) (setting.RulesetRule, []UninterpretedPart, error) {
	if isAbsent(fields["parameters"]) {
		return setting.RulesetRule{}, nil, fmt.Errorf("%w: missing fields: rules.parameters", ErrUnexpectedResponse)
	}
	parameters, err := decodeObject(fields["parameters"], "rules.parameters")
	if err != nil {
		return setting.RulesetRule{}, nil, err
	}
	items, err := decodeRequired[[]json.RawMessage](parameters["required_status_checks"], "rules.parameters.required_status_checks")
	if err != nil {
		return setting.RulesetRule{}, nil, err
	}
	strict, err := decodeRequired[bool](parameters["strict_required_status_checks_policy"], "rules.parameters.strict_required_status_checks_policy")
	if err != nil {
		return setting.RulesetRule{}, nil, err
	}
	checks := make([]setting.StatusCheck, len(items))
	var unread []string
	enforce, ok := decodeOptional[bool](parameters["do_not_enforce_on_create"])
	if !ok {
		unread = append(unread, "do_not_enforce_on_create")
	}
	for i, item := range items {
		check, checkUnread, err := decodeStatusCheck(item)
		if err != nil {
			return setting.RulesetRule{}, nil, err
		}
		checks[i] = check
		unread = append(unread, checkUnread...)
	}
	unread = append(unread, unreadKeys(parameters, "required_status_checks", "strict_required_status_checks_policy", "do_not_enforce_on_create")...)
	parts := ruleKeyParts(fields, setting.RulesetRuleRequiredStatusChecks, "type", "parameters")
	for _, name := range unread {
		parts = append(parts, UninterpretedPart{Kind: UninterpretedRuleParameter, Rule: setting.RulesetRuleRequiredStatusChecks, Name: name})
	}
	return setting.RequiredStatusChecksRule(setting.RequiredStatusChecksParameters{
		RequiredStatusChecks:             checks,
		StrictRequiredStatusChecksPolicy: strict,
		DoNotEnforceOnCreate:             enforce,
	}), parts, nil
}

// decodeStatusCheck は、チェックの一つの要素を読み、解釈しなかったパラメータの名前を、integration_id、読まないキーの名前の順で返す。
// 解釈しなかったものの名前は、ref_name の中のキーと同じく、required_status_checks の中と分かる名前にする。
// integration_id は、キーがないことと null を区別する。GitHub は送らなかった integration_id を返さず、null と 0 を拒否したため、null と正の整数でない値の意味を定める根拠がない。
func decodeStatusCheck(raw json.RawMessage) (setting.StatusCheck, []string, error) {
	fields, err := decodeObject(raw, "rules.parameters.required_status_checks")
	if err != nil {
		return setting.StatusCheck{}, nil, err
	}
	checkContext, err := decodeRequired[string](fields["context"], "rules.parameters.required_status_checks.context")
	if err != nil {
		return setting.StatusCheck{}, nil, err
	}
	check := setting.StatusCheck{Context: checkContext}
	var unread []string
	if rawID, ok := fields["integration_id"]; ok {
		if id, ok := decodeOptional[int64](rawID); ok && id > 0 {
			check.IntegrationID = id
		} else {
			unread = append(unread, "required_status_checks.integration_id")
		}
	}
	for _, key := range unreadKeys(fields, "context", "integration_id") {
		unread = append(unread, "required_status_checks."+key)
	}
	return check, unread, nil
}

// decodeAllowedMergeMethods は、allowed_merge_methods を応答の順のまま、重複も残して読む。
// 空の配列、定義にない値、null の要素を含む値、配列でない値は偽を返す（Rulesets の設計の解釈しない部分）。
// 要素を *string で受けるのは、null の要素を空文字列と読まないため。
func decodeAllowedMergeMethods(raw json.RawMessage) ([]setting.AllowedMergeMethod, bool) {
	texts, ok := decodeOptional[[]*string](raw)
	if !ok || len(texts) == 0 {
		return nil, false
	}
	methods := make([]setting.AllowedMergeMethod, len(texts))
	for i, text := range texts {
		if text == nil {
			return nil, false
		}
		if methods[i], ok = setting.ParseAllowedMergeMethod(*text); !ok {
			return nil, false
		}
	}
	return methods, true
}

// ruleKeyParts は、ルールの要素の known 以外のキーを、ルール型 ruleType の解釈しないキーとして返す。
func ruleKeyParts(fields map[string]json.RawMessage, ruleType setting.RulesetRuleType, known ...string) []UninterpretedPart {
	var parts []UninterpretedPart
	for _, key := range unreadKeys(fields, known...) {
		parts = append(parts, UninterpretedPart{Kind: UninterpretedRuleKey, Rule: ruleType, Name: key})
	}
	return parts
}

// unreadKeys は、fields の known 以外のキーを名前の順に返す。オブジェクトのキーの並びは JSON では意味を持たないため。
func unreadKeys(fields map[string]json.RawMessage, known ...string) []string {
	var keys []string
	for _, key := range slices.Sorted(maps.Keys(fields)) {
		if !slices.Contains(known, key) {
			keys = append(keys, key)
		}
	}
	return keys
}

// firstOfEachPart は、同じ種類、ルール型、名前の解釈しなかったものを、最初の一つだけ残して並びのまま返す。
func firstOfEachPart(parts []UninterpretedPart) []UninterpretedPart {
	seen := make(map[UninterpretedPart]struct{}, len(parts))
	var unique []UninterpretedPart
	for _, p := range parts {
		if _, duplicate := seen[p]; duplicate {
			continue
		}
		seen[p] = struct{}{}
		unique = append(unique, p)
	}
	return unique
}

// decodeObject は raw をオブジェクトとして読む。null も、キーを持たないオブジェクトと読まず、契約外の応答とする。
func decodeObject(raw json.RawMessage, path string) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrUnexpectedResponse, path, err)
	}
	if fields == nil {
		return nil, fmt.Errorf("%w: unsupported value: %s", ErrUnexpectedResponse, path)
	}
	return fields, nil
}

// decodeRequired は、取得契約が求める項目 raw を T として読む。欠落と null は、既定値で補わず契約外の応答とする。
func decodeRequired[T any](raw json.RawMessage, path string) (T, error) {
	var v T
	if isAbsent(raw) {
		return v, fmt.Errorf("%w: missing fields: %s", ErrUnexpectedResponse, path)
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return v, fmt.Errorf("%w: %s: %w", ErrUnexpectedResponse, path, err)
	}
	return v, nil
}

// decodeOptional は、取得契約が求めない項目 raw を T として読み、読めたかを返す。欠落、null、型違いは、契約外の応答にせず読めなかったとする。
func decodeOptional[T any](raw json.RawMessage) (T, bool) {
	var v T
	if isAbsent(raw) {
		return v, false
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return v, false
	}
	return v, true
}
