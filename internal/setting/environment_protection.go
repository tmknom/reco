package setting

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
)

// EnvironmentProtection は、Environment の保護設定の値。reco は観測するが、宣言を受け付けない。
type EnvironmentProtection struct {
	// WaitTimer はデプロイを待たせる分数。wait_timer の保護ルールがなければ nil。
	WaitTimer *int
	// RequiredReviewers は、required_reviewers の保護ルールがなければ nil。
	RequiredReviewers *RequiredReviewers
	CanAdminsBypass   bool
}

// RequiredReviewers は、承認を求めるレビュアーと自己承認の禁止からなる保護ルールの値。
type RequiredReviewers struct {
	PreventSelfReview bool
	// Reviewers は受け取った順のまま持つ。
	Reviewers []Reviewer
}

// Reviewer は、承認を求めるレビュアー。GitHub の利用者かチームの ID で指す。
type Reviewer struct {
	Type ReviewerType
	ID   int64
}

// ReviewerType は、レビュアーの種類。値は GitHub REST API の文字列で、表示にもそのまま使う。
type ReviewerType string

const (
	ReviewerUser ReviewerType = "User"
	ReviewerTeam ReviewerType = "Team"
)

// EnvironmentProtectionDifference は、保護設定の一つの項目の、二つの値のあいだの差。
// 保護設定の表記は CLI 仕様が一か所で定めるため、RulesetDifference と違い項目の名前と値を表示の表記で持つ。
type EnvironmentProtectionDifference struct {
	Item, Before, After string
}

// ParseReviewerType は s が定義された値に完全一致するときだけ、その値と真を返す。
func ParseReviewerType(s string) (ReviewerType, bool) {
	switch t := ReviewerType(s); t {
	case ReviewerUser, ReviewerTeam:
		return t, true
	default:
		return "", false
	}
}

// DifferencesTo は、p を other に変えるときの値の異なる項目を返す。項目の名前、値の表記、並びは CLI 仕様の変更対象外の設定の示し方に従う。
// 保護設定は宣言を受け付けず管理の単位の識別を持たないため、項目は名前で示す。
func (p EnvironmentProtection) DifferencesTo(other EnvironmentProtection) []EnvironmentProtectionDifference {
	var differences []EnvironmentProtectionDifference
	if !p.sameWaitTimer(other) {
		differences = append(differences, EnvironmentProtectionDifference{Item: "wait_timer", Before: p.waitTimerString(), After: other.waitTimerString()})
	}
	if !p.sameRequiredReviewers(other) {
		differences = append(differences, EnvironmentProtectionDifference{Item: "required_reviewers", Before: p.requiredReviewersString(), After: other.requiredReviewersString()})
	}
	if p.CanAdminsBypass != other.CanAdminsBypass {
		differences = append(differences, EnvironmentProtectionDifference{
			Item: "can_admins_bypass", Before: strconv.FormatBool(p.CanAdminsBypass), After: strconv.FormatBool(other.CanAdminsBypass),
		})
	}
	return differences
}

// sameWaitTimer は、wait_timer の有無と分数が同じかを返す。
func (p EnvironmentProtection) sameWaitTimer(other EnvironmentProtection) bool {
	if p.WaitTimer == nil || other.WaitTimer == nil {
		return p.WaitTimer == other.WaitTimer
	}
	return *p.WaitTimer == *other.WaitTimer
}

// sameRequiredReviewers は、required_reviewers の有無、自己承認の禁止、レビュアーの種類と ID の組の集合が同じかを返す。
// レビュアーの並びは区別しない。
func (p EnvironmentProtection) sameRequiredReviewers(other EnvironmentProtection) bool {
	if p.RequiredReviewers == nil || other.RequiredReviewers == nil {
		return p.RequiredReviewers == other.RequiredReviewers
	}
	return p.RequiredReviewers.PreventSelfReview == other.RequiredReviewers.PreventSelfReview &&
		slices.Equal(sortedReviewers(p.RequiredReviewers.Reviewers), sortedReviewers(other.RequiredReviewers.Reviewers))
}

// waitTimerString は、wait_timer を表示の表記で返す。保護ルールがなければ none。
func (p EnvironmentProtection) waitTimerString() string {
	if p.WaitTimer == nil {
		return "none"
	}
	return strconv.Itoa(*p.WaitTimer)
}

// requiredReviewersString は、required_reviewers を一行の表示の表記で返す。保護ルールがなければ none。
func (p EnvironmentProtection) requiredReviewersString() string {
	if p.RequiredReviewers == nil {
		return "none"
	}
	return "reviewers: " + p.RequiredReviewers.ReviewersString() +
		", prevent_self_review: " + strconv.FormatBool(p.RequiredReviewers.PreventSelfReview)
}

// ReviewersString は、レビュアーを [User 1, Team 2] の表記で、受け取った順のまま返す。
func (r RequiredReviewers) ReviewersString() string {
	formatted := make([]string, len(r.Reviewers))
	for i, reviewer := range r.Reviewers {
		formatted[i] = string(reviewer.Type) + " " + strconv.FormatInt(reviewer.ID, 10)
	}
	return "[" + strings.Join(formatted, ", ") + "]"
}

// sortedReviewers は、保持する並びを表示に使うため、引数を書き換えずに写しを整列する。重複は残す。
func sortedReviewers(reviewers []Reviewer) []Reviewer {
	return slices.SortedFunc(slices.Values(reviewers), func(a, b Reviewer) int {
		return cmp.Or(cmp.Compare(a.Type, b.Type), cmp.Compare(a.ID, b.ID))
	})
}
