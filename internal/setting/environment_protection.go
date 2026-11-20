package setting

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

// ParseReviewerType は s が定義された値に完全一致するときだけ、その値と真を返す。
func ParseReviewerType(s string) (ReviewerType, bool) {
	switch t := ReviewerType(s); t {
	case ReviewerUser, ReviewerTeam:
		return t, true
	default:
		return "", false
	}
}
