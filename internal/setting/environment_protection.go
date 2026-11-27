package setting

import "strconv"

// EnvironmentProtection は、Environment の保護設定の値。reco は観測し、can_admins_bypass のほかは宣言を受け付ける。
type EnvironmentProtection struct {
	// WaitTimer はデプロイを待たせる分数。wait_timer の保護ルールがなければ 0。
	WaitTimer int
	// RequiredReviewers は、required_reviewers の保護ルールがなければ nil。
	RequiredReviewers *RequiredReviewers
	CanAdminsBypass   bool
}

// RequiredReviewers は、担当者と自己承認の禁止からなる保護ルールの値。
type RequiredReviewers struct {
	PreventSelfReview bool
	// Reviewers は受け取った順のまま持つ。
	Reviewers []Reviewer
	// Logins は、観測した担当者の login。login を読めなかった担当者とチームは持たない。表示にだけ使い、同じ値かの判定には使わない。
	Logins map[Reviewer]string
}

// Reviewer は、担当者。GitHub の利用者かチームの ID で指す。
type Reviewer struct {
	Type ReviewerType
	ID   int64
}

// ReviewerType は、担当者の種類。値は GitHub REST API の文字列で、表示にもそのまま使う。
type ReviewerType string

const (
	ReviewerUser ReviewerType = "User"
	ReviewerTeam ReviewerType = "Team"
)

// EnvironmentProtectionDifference は、保護設定の一つの項目の、二つの値のあいだの差。
// 保護設定の表記は CLI 仕様が一か所で定めるため、RulesetDifference と違い項目の名前と値を表示の表記で持つ。
type EnvironmentProtectionDifference struct {
	Item, Before, After string
	// Key は、項目が Environment の中の管理の単位に当たるときの、その形。当たらない項目はゼロ値。
	Key Key
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
// 項目は名前で示し、管理の単位に当たる項目はその形を Key にも持つ。
func (p EnvironmentProtection) DifferencesTo(other EnvironmentProtection) []EnvironmentProtectionDifference {
	var differences []EnvironmentProtectionDifference
	if p.WaitTimer != other.WaitTimer {
		differences = append(differences, EnvironmentProtectionDifference{
			Item: EnvironmentWaitTimer.DeclaredKeyName(), Before: WaitTimerValue(p.WaitTimer).String(), After: WaitTimerValue(other.WaitTimer).String(), Key: EnvironmentWaitTimer,
		})
	}
	if !p.sameReviewerSet(other) {
		differences = append(differences, EnvironmentProtectionDifference{
			Item: EnvironmentRequiredReviewers.DeclaredKeyName(), Before: p.reviewerSetString(), After: other.reviewerSetString(), Key: EnvironmentRequiredReviewers,
		})
	}
	if p.PreventSelfReview() != other.PreventSelfReview() {
		differences = append(differences, EnvironmentProtectionDifference{
			Item: EnvironmentPreventSelfReview.DeclaredKeyName(), Before: strconv.FormatBool(p.PreventSelfReview()), After: strconv.FormatBool(other.PreventSelfReview()), Key: EnvironmentPreventSelfReview,
		})
	}
	if p.CanAdminsBypass != other.CanAdminsBypass {
		differences = append(differences, EnvironmentProtectionDifference{
			Item: "can_admins_bypass", Before: strconv.FormatBool(p.CanAdminsBypass), After: strconv.FormatBool(other.CanAdminsBypass),
		})
	}
	return differences
}

// PreventSelfReview は、自己承認の禁止の値を返す。
// required_reviewers の保護ルールがなければ偽とする。GitHub は担当者をすべて外すと保護ルールごと値をなくし、担当者だけを設定し直すと偽で作るため。
// 担当者のいない保護ルールは、偽とみなさず保護ルールが持つ値を返す。GitHub はこの保護ルールに真を残し、担当者を戻すと真のまま戻すため。
func (p EnvironmentProtection) PreventSelfReview() bool {
	return p.RequiredReviewers != nil && p.RequiredReviewers.PreventSelfReview
}

// sameReviewerSet は、担当者の集合が同じかを返す。管理設定の照合と同じく ReviewerSet の判定で比べ、保護ルールがないことは空集合として扱う。
func (p EnvironmentProtection) sameReviewerSet(other EnvironmentProtection) bool {
	return p.reviewerSet().equal(other.reviewerSet())
}

// reviewerSet は、担当者の集合を login を持たない値で返す。CLI 仕様が変更対象外の設定に login を示さないため。
func (p EnvironmentProtection) reviewerSet() ReviewerSet {
	if p.RequiredReviewers == nil {
		return ReviewerSetOfRule(nil)
	}
	return ReviewerSetOfRule(&RequiredReviewers{Reviewers: p.RequiredReviewers.Reviewers})
}

// reviewerSetString は、required_reviewers の値を ReviewerSet と同じ表記で返す。保護ルールがなければ空集合の表記。
func (p EnvironmentProtection) reviewerSetString() string {
	return p.reviewerSet().String()
}
