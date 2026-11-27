package configuration

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/tmknom/reco/internal/setting"
)

// rulesets は、Ruleset の宣言の並びを宣言の順のまま読む。
// 同じ Ruleset とみなす名前の宣言は一つに限る。GitHub は名前の英字の大小を区別しないため、大小だけが違う名前も重複にする。
func (d document) rulesets(n *yaml.Node, path string) ([]RulesetDeclaration, error) {
	items, err := d.sequenceItems(n, path)
	if err != nil {
		return nil, err
	}
	declarations := make([]RulesetDeclaration, 0, len(items))
	for i, item := range items {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		declaration, err := d.ruleset(item, itemPath)
		if err != nil {
			return nil, err
		}
		if j := slices.IndexFunc(declarations, func(seen RulesetDeclaration) bool { return seen.Name.EqualFold(declaration.Name) }); j >= 0 {
			return nil, invalid(item, itemPath, "duplicate name %q; same as %s[%d] (compared ignoring letter case)", declaration.Name.String(), path, j)
		}
		declarations = append(declarations, declaration)
	}
	return declarations, nil
}

// ruleset は、存在し値を伴うことを宣言する一つの Ruleset を読む。
// 6 つのキーはどれも必須で、省略を補わない。
// 判定は、キーの集合、name、target、enforcement、conditions、bypass_actors、rules の順に行う。
func (d document) ruleset(n *yaml.Node, path string) (RulesetDeclaration, error) {
	entries, err := d.mappingEntries(n, path, []string{"name", "target", "enforcement", "conditions", "bypass_actors", "rules"}, nil)
	if err != nil {
		return RulesetDeclaration{}, err
	}
	namePath := path + ".name"
	name, err := d.stringValue(entries["name"], namePath)
	if err != nil {
		return RulesetDeclaration{}, err
	}
	rulesetName, err := setting.ParseRulesetName(name)
	if err != nil {
		return RulesetDeclaration{}, invalid(entries["name"], namePath, "%v; got %q", err, name)
	}
	var ruleset setting.Ruleset
	if ruleset.Target, err = d.rulesetTarget(entries["target"], path+".target"); err != nil {
		return RulesetDeclaration{}, err
	}
	if ruleset.Enforcement, err = d.rulesetEnforcement(entries["enforcement"], path+".enforcement"); err != nil {
		return RulesetDeclaration{}, err
	}
	if ruleset.Conditions, err = d.rulesetConditions(entries["conditions"], path+".conditions"); err != nil {
		return RulesetDeclaration{}, err
	}
	if ruleset.BypassActors, err = d.bypassActors(entries["bypass_actors"], path+".bypass_actors"); err != nil {
		return RulesetDeclaration{}, err
	}
	if ruleset.Rules, err = d.rulesetRules(entries["rules"], path+".rules"); err != nil {
		return RulesetDeclaration{}, err
	}
	return RulesetDeclaration{Name: rulesetName, Desired: setting.RulesetValue(ruleset)}, nil
}

// rulesetTarget は、文字列のうち宣言で受け付ける branch だけを受理する。
func (d document) rulesetTarget(n *yaml.Node, path string) (setting.RulesetTarget, error) {
	s, err := d.stringValue(n, path)
	if err != nil {
		return "", err
	}
	target, ok := setting.ParseRulesetTarget(s)
	if !ok {
		return "", invalid(n, path, "must be branch; got %q", s)
	}
	return target, nil
}

// rulesetEnforcement は、取り得る値のうち宣言で受け付ける active と disabled だけを受理する。
// evaluate は show が表示し得る値のため、そのまま書いた利用者に拒否の理由を添える。
func (d document) rulesetEnforcement(n *yaml.Node, path string) (setting.RulesetEnforcement, error) {
	s, err := d.stringValue(n, path)
	if err != nil {
		return "", err
	}
	switch enforcement, _ := setting.ParseRulesetEnforcement(s); enforcement {
	case setting.RulesetEnforcementActive, setting.RulesetEnforcementDisabled:
		return enforcement, nil
	case setting.RulesetEnforcementEvaluate:
		return "", invalid(n, path, "must be active or disabled; got %q; GitHub rejects evaluate for repositories owned by personal accounts", s)
	}
	return "", invalid(n, path, "must be active or disabled; got %q", s)
}

// rulesetConditions は、ref_name だけを持つ対象の条件を読む。ref_name は include と exclude の両方を持つ。
func (d document) rulesetConditions(n *yaml.Node, path string) (setting.RulesetConditions, error) {
	entries, err := d.mappingEntries(n, path, []string{"ref_name"}, nil)
	if err != nil {
		return setting.RulesetConditions{}, err
	}
	refNamePath := path + ".ref_name"
	refName, err := d.mappingEntries(entries["ref_name"], refNamePath, []string{"include", "exclude"}, nil)
	if err != nil {
		return setting.RulesetConditions{}, err
	}
	include, err := d.refNamePatterns(refName["include"], refNamePath+".include")
	if err != nil {
		return setting.RulesetConditions{}, err
	}
	exclude, err := d.refNamePatterns(refName["exclude"], refNamePath+".exclude")
	if err != nil {
		return setting.RulesetConditions{}, err
	}
	return setting.RulesetConditions{RefName: &setting.RefNameCondition{Include: include, Exclude: exclude}}, nil
}

// refNamePatterns は、ブランチのパターンの並びを宣言の順のまま返す。
// 空や前後の空白、重複の規則は patterns_allowed と同じで、パターンの形の誤りは空や前後の空白より後、重複より先に示す。
// パターンは大文字小文字を区別して比べる。
func (d document) refNamePatterns(n *yaml.Node, path string) ([]string, error) {
	return d.uniquePatterns(n, path, func(item *yaml.Node, itemPath, pattern string) error {
		if pattern != "~DEFAULT_BRANCH" && pattern != "~ALL" && !strings.HasPrefix(pattern, "refs/heads/") {
			return invalid(item, itemPath, "must be ~DEFAULT_BRANCH, ~ALL, or start with refs/heads/; got %q", pattern)
		}
		return nil
	})
}

// bypassActors は、Bypass の主体の並びを宣言の順のまま読む。空のシーケンスは「主体なし」の期待値で、空でも nil にしない。
// actor_type と actor_id が同じ主体は、bypass_mode が違っても一つに限る。DeployKey は actor_id を持たないため、二つ書けない。
func (d document) bypassActors(n *yaml.Node, path string) ([]setting.BypassActor, error) {
	items, err := d.sequenceContent(n, path)
	if err != nil {
		return nil, err
	}
	actors := make([]setting.BypassActor, 0, len(items))
	for i, item := range items {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		actor, err := d.bypassActor(item, itemPath)
		if err != nil {
			return nil, err
		}
		if j := slices.IndexFunc(actors, actor.SameActor); j >= 0 {
			return nil, invalid(item, itemPath, "duplicate bypass actor %s; same as %s[%d]", bypassActorLabel(actor), path, j)
		}
		actors = append(actors, actor)
	}
	return actors, nil
}

// bypassActor は、Bypass の主体 1 件を読む。
// 判定は、キーの集合、actor_type、actor_id の有無の組み合わせ、actor_id の値、bypass_mode の順に行う。
func (d document) bypassActor(n *yaml.Node, path string) (setting.BypassActor, error) {
	entries, err := d.mappingEntries(n, path, []string{"actor_type", "bypass_mode"}, []string{"actor_id"})
	if err != nil {
		return setting.BypassActor{}, err
	}
	typePath := path + ".actor_type"
	typeString, err := d.stringValue(entries["actor_type"], typePath)
	if err != nil {
		return setting.BypassActor{}, err
	}
	actorType, ok := setting.ParseBypassActorType(typeString)
	if !ok {
		return setting.BypassActor{}, invalid(entries["actor_type"], typePath, "must be RepositoryRole, User, Integration, or DeployKey; got %q", typeString)
	}
	actorIDNode, hasActorID := entries["actor_id"]
	switch {
	case actorType == setting.BypassActorDeployKey && hasActorID:
		return setting.BypassActor{}, invalid(actorIDNode, path+".actor_id", `actor_id is not allowed when actor_type is "DeployKey"`)
	case actorType != setting.BypassActorDeployKey && !hasActorID:
		return setting.BypassActor{}, invalid(n, path, "actor_type %q requires actor_id", actorType)
	}
	var actorID int64
	if hasActorID {
		if actorID, err = d.actorID(actorIDNode, path+".actor_id", actorType); err != nil {
			return setting.BypassActor{}, err
		}
	}
	modePath := path + ".bypass_mode"
	modeString, err := d.stringValue(entries["bypass_mode"], modePath)
	if err != nil {
		return setting.BypassActor{}, err
	}
	mode, ok := setting.ParseBypassMode(modeString)
	if !ok {
		return setting.BypassActor{}, invalid(entries["bypass_mode"], modePath, "must be always, pull_request, or exempt; got %q", modeString)
	}
	if actorType == setting.BypassActorDeployKey && mode != setting.BypassModeAlways {
		return setting.BypassActor{}, invalid(entries["bypass_mode"], modePath, `must be always when actor_type is "DeployKey"; got %q`, modeString)
	}
	return setting.BypassActor{ActorType: actorType, ActorID: actorID, BypassMode: mode}, nil
}

// actorID は、DeployKey 以外の主体の actor_id を読む。
// RepositoryRole は GitHub が受け付ける 2、4、5 に限り、User と Integration は正の整数に限る。
func (d document) actorID(n *yaml.Node, path string, actorType setting.BypassActorType) (int64, error) {
	id, err := d.intValue(n, path)
	if err != nil {
		return 0, err
	}
	switch {
	case actorType == setting.BypassActorRepositoryRole && !slices.Contains([]int{2, 4, 5}, id):
		return 0, invalid(n, path, "must be 2, 4, or 5 when actor_type is %q; got %d", actorType, id)
	case id < 1:
		return 0, invalid(n, path, "must be a positive integer; got %d", id)
	}
	return int64(id), nil
}

// bypassActorLabel は、重複の誤りが示す主体の表記。DeployKey は actor_id を持たないため、種類だけを示す。
func bypassActorLabel(actor setting.BypassActor) string {
	if actor.ActorType == setting.BypassActorDeployKey {
		return string(actor.ActorType)
	}
	return fmt.Sprintf("%s %d", actor.ActorType, actor.ActorID)
}

// rulesetRules は、ルールの並びを宣言の順のまま読む。空のシーケンスは「ルールなし」の期待値で、空でも nil にしない。
// 同じ type のルールは、parameters が違っても一つに限る。
func (d document) rulesetRules(n *yaml.Node, path string) ([]setting.RulesetRule, error) {
	items, err := d.sequenceContent(n, path)
	if err != nil {
		return nil, err
	}
	rules := make([]setting.RulesetRule, 0, len(items))
	for i, item := range items {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		rule, err := d.rulesetRule(item, itemPath)
		if err != nil {
			return nil, err
		}
		if j := slices.IndexFunc(rules, func(seen setting.RulesetRule) bool { return seen.Type() == rule.Type() }); j >= 0 {
			return nil, invalid(item, itemPath, "duplicate rule type %q; same as %s[%d]", rule.Type(), path, j)
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

// rulesetRule は、ルール 1 件を読む。deletion と non_fast_forward は parameters を持てず、pull_request と required_status_checks は parameters を必須とする。
// 判定は、type、parameters の有無の組み合わせ、parameters の中身の順に行う。
func (d document) rulesetRule(n *yaml.Node, path string) (setting.RulesetRule, error) {
	entries, err := d.mappingEntries(n, path, []string{"type"}, []string{"parameters"})
	if err != nil {
		return setting.RulesetRule{}, err
	}
	typePath := path + ".type"
	typeString, err := d.stringValue(entries["type"], typePath)
	if err != nil {
		return setting.RulesetRule{}, err
	}
	ruleType, ok := setting.ParseRulesetRuleType(typeString)
	if !ok {
		return setting.RulesetRule{}, invalid(entries["type"], typePath, "must be pull_request, deletion, required_status_checks, or non_fast_forward; got %q", typeString)
	}
	parametersNode, hasParameters := entries["parameters"]
	parametersPath := path + ".parameters"
	takesParameters := ruleType.HasParameters()
	switch {
	case hasParameters && !takesParameters:
		return setting.RulesetRule{}, invalid(parametersNode, parametersPath, "parameters is not allowed when type is %q", ruleType)
	case !hasParameters && takesParameters:
		return setting.RulesetRule{}, invalid(n, path, "type %q requires parameters", ruleType)
	}
	return d.typedRulesetRule(ruleType, parametersNode, parametersPath)
}

// typedRulesetRule は、parameters の有無を確かめたルール型 ruleType のルールを、型が持つなら parametersNode を読んで作る。
// ルール型を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
func (d document) typedRulesetRule(ruleType setting.RulesetRuleType, parametersNode *yaml.Node, parametersPath string) (setting.RulesetRule, error) {
	switch ruleType {
	case setting.RulesetRuleDeletion:
		return setting.DeletionRule(), nil
	case setting.RulesetRuleNonFastForward:
		return setting.NonFastForwardRule(), nil
	case setting.RulesetRulePullRequest:
		parameters, err := d.pullRequestParameters(parametersNode, parametersPath)
		if err != nil {
			return setting.RulesetRule{}, err
		}
		return setting.PullRequestRule(parameters), nil
	case setting.RulesetRuleRequiredStatusChecks:
		parameters, err := d.requiredStatusChecksParameters(parametersNode, parametersPath)
		if err != nil {
			return setting.RulesetRule{}, err
		}
		return setting.RequiredStatusChecksRule(parameters), nil
	}
	panic(fmt.Sprintf("configuration: undefined ruleset rule type %q", ruleType))
}

// pullRequestParameters は、pull_request のルールの 5 項目をすべて読む。
func (d document) pullRequestParameters(n *yaml.Node, path string) (setting.PullRequestParameters, error) {
	entries, err := d.mappingEntries(n, path, []string{
		"required_approving_review_count",
		"dismiss_stale_reviews_on_push",
		"require_code_owner_review",
		"require_last_push_approval",
		"required_review_thread_resolution",
	}, nil)
	if err != nil {
		return setting.PullRequestParameters{}, err
	}
	var parameters setting.PullRequestParameters
	countPath := path + ".required_approving_review_count"
	if parameters.RequiredApprovingReviewCount, err = d.intValue(entries["required_approving_review_count"], countPath); err != nil {
		return setting.PullRequestParameters{}, err
	}
	if count := parameters.RequiredApprovingReviewCount; count > 10 {
		return setting.PullRequestParameters{}, invalid(entries["required_approving_review_count"], countPath, "must be an integer from 0 to 10; got %d", count)
	}
	for _, field := range []struct {
		key string
		dst *bool
	}{
		{"dismiss_stale_reviews_on_push", &parameters.DismissStaleReviewsOnPush},
		{"require_code_owner_review", &parameters.RequireCodeOwnerReview},
		{"require_last_push_approval", &parameters.RequireLastPushApproval},
		{"required_review_thread_resolution", &parameters.RequiredReviewThreadResolution},
	} {
		if *field.dst, err = d.boolValue(entries[field.key], path+"."+field.key); err != nil {
			return setting.PullRequestParameters{}, err
		}
	}
	return parameters, nil
}

// requiredStatusChecksParameters は、required_status_checks のルールのパラメータを読む。do_not_enforce_on_create は省略でき、省略したら nil のまま残す。
// 判定は、キーの集合、チェック、strict_required_status_checks_policy、do_not_enforce_on_create の順に行う。
func (d document) requiredStatusChecksParameters(n *yaml.Node, path string) (setting.RequiredStatusChecksParameters, error) {
	entries, err := d.mappingEntries(n, path, []string{"required_status_checks", "strict_required_status_checks_policy"}, []string{"do_not_enforce_on_create"})
	if err != nil {
		return setting.RequiredStatusChecksParameters{}, err
	}
	var parameters setting.RequiredStatusChecksParameters
	if parameters.RequiredStatusChecks, err = d.statusChecks(entries["required_status_checks"], path+".required_status_checks"); err != nil {
		return setting.RequiredStatusChecksParameters{}, err
	}
	strictPath := path + ".strict_required_status_checks_policy"
	if parameters.StrictRequiredStatusChecksPolicy, err = d.boolValue(entries["strict_required_status_checks_policy"], strictPath); err != nil {
		return setting.RequiredStatusChecksParameters{}, err
	}
	if node, ok := entries["do_not_enforce_on_create"]; ok {
		enforce, err := d.boolValue(node, path+".do_not_enforce_on_create")
		if err != nil {
			return setting.RequiredStatusChecksParameters{}, err
		}
		parameters.DoNotEnforceOnCreate = &enforce
	}
	return parameters, nil
}

// statusChecks は、チェックの並びを宣言の順のまま読む。空のシーケンスは「チェックなし」の期待値で、空でも nil にしない。
// context と integration_id の組が同じチェックは一つに限る。integration_id の省略も一つの値として比べる。
func (d document) statusChecks(n *yaml.Node, path string) ([]setting.StatusCheck, error) {
	items, err := d.sequenceContent(n, path)
	if err != nil {
		return nil, err
	}
	checks := make([]setting.StatusCheck, 0, len(items))
	for i, item := range items {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		check, err := d.statusCheck(item, itemPath)
		if err != nil {
			return nil, err
		}
		if j := slices.Index(checks, check); j >= 0 {
			return nil, invalid(item, itemPath, "duplicate status check %s; same as %s[%d]", statusCheckLabel(check), path, j)
		}
		checks = append(checks, check)
	}
	return checks, nil
}

// statusCheck は、チェック 1 件を読む。context は空文字列だけを拒否し、前後の空白も大文字小文字も書いたとおりに持つ。
// GitHub が送ったとおりに保存するため、正規化すると宣言と観測が食い違う。
func (d document) statusCheck(n *yaml.Node, path string) (setting.StatusCheck, error) {
	entries, err := d.mappingEntries(n, path, []string{"context"}, []string{"integration_id"})
	if err != nil {
		return setting.StatusCheck{}, err
	}
	contextPath := path + ".context"
	context, err := d.stringValue(entries["context"], contextPath)
	if err != nil {
		return setting.StatusCheck{}, err
	}
	if context == "" {
		return setting.StatusCheck{}, invalid(entries["context"], contextPath, "must not be empty")
	}
	check := setting.StatusCheck{Context: context}
	if node, ok := entries["integration_id"]; ok {
		idPath := path + ".integration_id"
		id, err := d.intValue(node, idPath)
		if err != nil {
			return setting.StatusCheck{}, err
		}
		if id < 1 {
			return setting.StatusCheck{}, invalid(node, idPath, "must be a positive integer; got %d", id)
		}
		check.IntegrationID = int64(id)
	}
	return check, nil
}

// statusCheckLabel は、重複の誤りが示すチェックの表記。context は任意の文字を含み得るため、引用符で囲んでエスケープする。
func statusCheckLabel(check setting.StatusCheck) string {
	if check.IntegrationID == 0 {
		return strconv.Quote(check.Context)
	}
	return fmt.Sprintf("%q with integration_id %d", check.Context, check.IntegrationID)
}
