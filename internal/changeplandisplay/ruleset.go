package changeplandisplay

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/setting"
)

// changedPair は、外れる要素と加わる要素のうち、同じものとして一行にまとめる組。
type changedPair[T any] struct {
	from, to T
}

// rulesetChangeText は、Ruleset の変更予定を表示仕様の形で返す。値は1行に収まらないため、期待値の位置に present または absent を置き、値を続く行に示す。
// 期待値が値なら、現在が値でない (作成と比較不能) なら宣言の値の全体を、値なら値の差だけを示す。
// 期待値が存在しないなら、削除は計画時の観測値の全体を示し、比較不能は現在の値が読めていないため値の行を示さない。
// 解釈しなかったものは、理由が解釈しない部分であることと一つ以上あることが一致しなければ、扱えない部分を示さずに承認を求め得るため、不変条件の破れとして panic する。
func rulesetChangeText(s changeplan.SettingPlan) string {
	var b strings.Builder
	switch s.Result {
	case changeplan.Match:
		return ""
	case changeplan.Mismatch:
		observed, _ := s.Observed.Value()
		switch {
		case s.Desired.Equal(setting.Absent()):
			fmt.Fprintf(&b, "  %s: present -> absent\n%s", s.Unit.String(), rulesetValueLines(observed.Ruleset()))
		case s.IsConfirmedCreation():
			fmt.Fprintf(&b, "  %s: absent -> present\n%s", s.Unit.String(), declaredRulesetValueLines(s.Desired.Ruleset()))
		default:
			fmt.Fprintf(&b, "  %s:\n%s", s.Unit.String(), rulesetDifferenceLines(observed.Ruleset(), s.Desired.Ruleset()))
		}
	case changeplan.NotComparable:
		if s.Desired.Equal(setting.Absent()) {
			fmt.Fprintf(&b, "  %s: %s -> absent\n", s.Unit.String(), repositoryStateText(s.Observed.UnavailableReason()))
		} else {
			fmt.Fprintf(&b, "  %s: %s -> present\n%s", s.Unit.String(), repositoryStateText(s.Observed.UnavailableReason()), declaredRulesetValueLines(s.Desired.Ruleset()))
		}
	}
	if s.Applicability.IsNotApplicable() {
		fmt.Fprintf(&b, "    Applicability: not applicable (%s)\n", notApplicableReasonText(s))
	}
	b.WriteString(uninterpretedPartsLines(s))
	return b.String()
}

// declaredRulesetValueLines は、宣言の値の全体を1行ずつ返す。
// 宣言の値は必ず ref_name を持ち、持たない値は conditions: {} と示すと承認した値と違う値を示すことになるため、不変条件の破れとして panic する。
func declaredRulesetValueLines(r setting.Ruleset) string {
	if r.Conditions.RefName == nil {
		panic("changeplandisplay: declared ruleset value has no ref_name")
	}
	return rulesetValueLines(r)
}

// rulesetValueLines は、Ruleset の値の全体を1行ずつ返す。
// 観測値は ref_name を持たないことがあり、その値は、include と exclude がどちらも空の値と異なるため、2行の代わりに conditions: {} の1行で示す。
func rulesetValueLines(r setting.Ruleset) string {
	conditions := "    conditions: {}\n"
	if ref := r.Conditions.RefName; ref != nil {
		conditions = fmt.Sprintf("    include: %s\n    exclude: %s\n", patternsText(ref.Include), patternsText(ref.Exclude))
	}
	return fmt.Sprintf("    target: %s\n    enforcement: %s\n%s    bypass_actors: %s\n    rules: %s\n",
		r.Target, r.Enforcement, conditions, bypassActorsText(r.BypassActors), rulesText(r.Rules))
}

// rulesetDifferenceLines は、現在の値 observed を宣言の値 desired に変えるときの値の差を、変わる部分だけの行で返す。
// 差のない不一致は示す行がなく、変更の中身を示さずに承認を求め得るため、不変条件の破れとして panic する。
// 宣言の値は必ず ref_name を持つため、ref_name がなくなる向きの差も不変条件の破れとして panic する。
func rulesetDifferenceLines(observed, desired setting.Ruleset) string {
	d := observed.DifferenceTo(desired)
	if d.Empty() {
		panic("changeplandisplay: mismatched ruleset has no difference to show")
	}
	var b strings.Builder
	if d.TargetChanged {
		fmt.Fprintf(&b, "    target: %s -> %s\n", observed.Target, desired.Target)
	}
	if d.EnforcementChanged {
		fmt.Fprintf(&b, "    enforcement: %s -> %s\n", observed.Enforcement, desired.Enforcement)
	}
	if d.RefNameChanged {
		if desired.Conditions.RefName == nil {
			panic("changeplandisplay: declared ruleset value has no ref_name")
		}
		b.WriteString("    conditions: {} -> ref_name\n")
	}
	b.WriteString(elementsLine("Include patterns to remove", d.IncludeRemoved, patternsText))
	b.WriteString(elementsLine("Include patterns to add", d.IncludeAdded, patternsText))
	b.WriteString(elementsLine("Exclude patterns to remove", d.ExcludeRemoved, patternsText))
	b.WriteString(elementsLine("Exclude patterns to add", d.ExcludeAdded, patternsText))
	actorsRemoved, actorsAdded, modeChanges := pairChanged(d.BypassActorsRemoved, d.BypassActorsAdded, setting.BypassActor.SameActor)
	b.WriteString(elementsLine("Bypass actors to remove", actorsRemoved, bypassActorsText))
	b.WriteString(elementsLine("Bypass actors to add", actorsAdded, bypassActorsText))
	for _, change := range modeChanges {
		fmt.Fprintf(&b, "    Bypass actor %s: %s -> %s\n", bypassActorIdentityText(change.to), change.from.BypassMode, change.to.BypassMode)
	}
	sameType := func(x, y setting.RulesetRule) bool { return x.Type() == y.Type() }
	rulesRemoved, rulesAdded, parameterChanges := pairChanged(d.RulesRemoved, d.RulesAdded, sameType)
	b.WriteString(elementsLine("Rules to remove", rulesRemoved, rulesText))
	b.WriteString(elementsLine("Rules to add", rulesAdded, rulesText))
	for _, change := range parameterChanges {
		b.WriteString(ruleParameterChangeLines(change.from, change.to))
	}
	return b.String()
}

// ruleParameterChangeLines は、同じ型で値の違う二つのルール from、to のパラメータの差の行を返す。
// 同じ型で値の違う二つのルールはパラメータを持つ型に限るため、パラメータのない型の組と定義にない型は、不変条件の破れとして panic する。
// ルール型を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
func ruleParameterChangeLines(from, to setting.RulesetRule) string {
	switch to.Type() {
	case setting.RulesetRulePullRequest:
		return fmt.Sprintf("    Rule %s: %s\n", to.Type(), pullRequestParameterChangesText(from.PullRequest(), to.PullRequest()))
	case setting.RulesetRuleRequiredStatusChecks:
		return requiredStatusChecksChangeLines(from.RequiredStatusChecks(), to.RequiredStatusChecks())
	case setting.RulesetRuleDeletion, setting.RulesetRuleNonFastForward:
	}
	panic(fmt.Sprintf("changeplandisplay: rule %q has no parameters to change", to.Type()))
}

// requiredStatusChecksChangeLines は、required_status_checks のパラメータの差を、なくなるチェック、加わるチェック、変わる真偽値の行の順で返す。
func requiredStatusChecksChangeLines(from, to setting.RequiredStatusChecksParameters) string {
	d := from.DifferenceTo(to)
	var b strings.Builder
	b.WriteString(elementsLine("Required status checks to remove", d.ChecksRemoved, statusChecksText))
	b.WriteString(elementsLine("Required status checks to add", d.ChecksAdded, statusChecksText))
	var changes []string
	if d.StrictPolicyChanged {
		changes = append(changes, fmt.Sprintf("strict_required_status_checks_policy: %t -> %t", from.StrictRequiredStatusChecksPolicy, to.StrictRequiredStatusChecksPolicy))
	}
	if d.DoNotEnforceOnCreateChanged {
		changes = append(changes, fmt.Sprintf("do_not_enforce_on_create: %t -> %t", from.DoNotEnforceOnCreate, to.DoNotEnforceOnCreate))
	}
	if len(changes) > 0 {
		fmt.Fprintf(&b, "    Rule %s: %s\n", setting.RulesetRuleRequiredStatusChecks, strings.Join(changes, ", "))
	}
	return b.String()
}

// elementsLine は、要素が一つ以上あるときだけ、要素の一覧の行を返す。表示仕様は該当する要素のない行を示さない。
func elementsLine[T any](label string, elements []T, text func([]T) string) string {
	if len(elements) == 0 {
		return ""
	}
	return fmt.Sprintf("    %s: %s\n", label, text(elements))
}

// pairChanged は、外れる要素 removed と加わる要素 added のうち、same で同じものとする要素がどちらにもちょうど一つずつある組を、加わる要素の並びで返す。
// 残りは外れる要素と加わる要素のまま、それぞれの並びで返す。
// 二つ以上あるときにまとめると、どれがどれに変わるかを reco が決めて示すことになるため、まとめない。
// 同じものの判定を same 一つに保つため map で引かずに探す。時間は外れる要素の数と加わる要素の数の積で、reco が大きさを制御しない観測値 (外れる要素) には比例するだけになる。
func pairChanged[T any](removed, added []T, same func(T, T) bool) (restRemoved, restAdded []T, pairs []changedPair[T]) {
	paired := make([]bool, len(removed))
	for _, to := range added {
		i := onlySameIndex(removed, to, same)
		if i < 0 || onlySameIndex(added, to, same) < 0 {
			restAdded = append(restAdded, to)
			continue
		}
		paired[i] = true
		pairs = append(pairs, changedPair[T]{from: removed[i], to: to})
	}
	for i, from := range removed {
		if !paired[i] {
			restRemoved = append(restRemoved, from)
		}
	}
	return restRemoved, restAdded, pairs
}

// onlySameIndex は、elements のうち same で e と同じものがちょうど一つなら、その位置を返す。なければ、または二つ以上あれば -1 を返す。
func onlySameIndex[T any](elements []T, e T, same func(T, T) bool) int {
	found := -1
	for i, other := range elements {
		if !same(e, other) {
			continue
		}
		if found >= 0 {
			return -1
		}
		found = i
	}
	return found
}

// patternsText は、パターンの一覧を、二重引用符で囲んでエスケープした要素の並びで返す。パターンの改行や制御文字が変更計画の行を偽装しないようにするため。
func patternsText(patterns []string) string {
	quoted := make([]string, len(patterns))
	for i, p := range patterns {
		quoted[i] = strconv.Quote(p)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// bypassActorsText は、Bypass の一覧を、主体と方式の並びで返す。
func bypassActorsText(actors []setting.BypassActor) string {
	texts := make([]string, len(actors))
	for i, a := range actors {
		texts[i] = bypassActorIdentityText(a) + " " + string(a.BypassMode)
	}
	return "[" + strings.Join(texts, ", ") + "]"
}

// bypassActorIdentityText は、Bypass の主体を返す。DeployKey は ID を持たず、0 を ID として示さない。
func bypassActorIdentityText(a setting.BypassActor) string {
	if a.ActorType == setting.BypassActorDeployKey {
		return string(a.ActorType)
	}
	return string(a.ActorType) + " " + strconv.FormatInt(a.ActorID, 10)
}

// rulesText は、ルールの一覧を、パラメータを含めた要素の並びで返す。
func rulesText(rules []setting.RulesetRule) string {
	texts := make([]string, len(rules))
	for i, r := range rules {
		texts[i] = ruleText(r)
	}
	return "[" + strings.Join(texts, ", ") + "]"
}

// ruleText は、ルール一つを返す。
// パラメータを持つルール型を足して case を足し忘れ、パラメータを黙って省かないよう、lint の exhaustive が検出する形にし、default を置かず、定義にない型は panic する。
func ruleText(r setting.RulesetRule) string {
	switch r.Type() {
	case setting.RulesetRulePullRequest:
		p := r.PullRequest()
		return fmt.Sprintf("%s (required_approving_review_count: %d, dismiss_stale_reviews_on_push: %t, require_code_owner_review: %t, "+
			"require_last_push_approval: %t, required_review_thread_resolution: %t, allowed_merge_methods: %s, require_extra_approval_for_unattributed_changes: %t)",
			r.Type(), p.RequiredApprovingReviewCount, p.DismissStaleReviewsOnPush, p.RequireCodeOwnerReview, p.RequireLastPushApproval, p.RequiredReviewThreadResolution,
			setting.AllowedMergeMethodsText(p.AllowedMergeMethods), p.RequireExtraApprovalForUnattributedChanges)
	case setting.RulesetRuleDeletion, setting.RulesetRuleNonFastForward:
		return string(r.Type())
	case setting.RulesetRuleRequiredStatusChecks:
		p := r.RequiredStatusChecks()
		return fmt.Sprintf("%s (required_status_checks: %s, strict_required_status_checks_policy: %t, do_not_enforce_on_create: %t)",
			r.Type(), statusChecksText(p.RequiredStatusChecks), p.StrictRequiredStatusChecksPolicy, p.DoNotEnforceOnCreate)
	}
	panic(fmt.Sprintf("changeplandisplay: undefined ruleset rule type %q", r.Type()))
}

// statusChecksText は、チェックの一覧を、二重引用符で囲んでエスケープした context と、発行元を限るなら integration_id の並びで返す。
// context は GitHub が返した任意の文字列のため、パターンと同じく変更計画の行を偽装しないようにする。
func statusChecksText(checks []setting.StatusCheck) string {
	texts := make([]string, len(checks))
	for i, c := range checks {
		texts[i] = strconv.Quote(c.Context)
		if c.IntegrationID != 0 {
			texts[i] += " (integration_id: " + strconv.FormatInt(c.IntegrationID, 10) + ")"
		}
	}
	return "[" + strings.Join(texts, ", ") + "]"
}

// pullRequestParameterChangesText は、pull_request のパラメータのうち変わるものだけを、パラメータの順に現在値 -> 期待値で返す。
func pullRequestParameterChangesText(from, to setting.PullRequestParameters) string {
	d := from.DifferenceTo(to)
	var changes []string
	if d.RequiredApprovingReviewCountChanged {
		changes = append(changes, fmt.Sprintf("required_approving_review_count: %d -> %d", from.RequiredApprovingReviewCount, to.RequiredApprovingReviewCount))
	}
	flags := []struct {
		name     string
		changed  bool
		from, to bool
	}{
		{"dismiss_stale_reviews_on_push", d.DismissStaleReviewsOnPushChanged, from.DismissStaleReviewsOnPush, to.DismissStaleReviewsOnPush},
		{"require_code_owner_review", d.RequireCodeOwnerReviewChanged, from.RequireCodeOwnerReview, to.RequireCodeOwnerReview},
		{"require_last_push_approval", d.RequireLastPushApprovalChanged, from.RequireLastPushApproval, to.RequireLastPushApproval},
		{"required_review_thread_resolution", d.RequiredReviewThreadResolutionChanged, from.RequiredReviewThreadResolution, to.RequiredReviewThreadResolution},
	}
	for _, f := range flags {
		if f.changed {
			changes = append(changes, fmt.Sprintf("%s: %t -> %t", f.name, f.from, f.to))
		}
	}
	if d.AllowedMergeMethodsChanged {
		changes = append(changes, fmt.Sprintf("allowed_merge_methods: %s -> %s",
			setting.AllowedMergeMethodsText(from.AllowedMergeMethods), setting.AllowedMergeMethodsText(to.AllowedMergeMethods)))
	}
	if d.RequireExtraApprovalForUnattributedChangesChanged {
		changes = append(changes, fmt.Sprintf("require_extra_approval_for_unattributed_changes: %t -> %t",
			from.RequireExtraApprovalForUnattributedChanges, to.RequireExtraApprovalForUnattributedChanges))
	}
	return strings.Join(changes, ", ")
}
