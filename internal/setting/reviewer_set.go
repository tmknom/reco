package setting

import (
	"cmp"
	"maps"
	"slices"
	"strconv"
	"strings"
)

// ReviewerSet は、Environment の担当者の集合の値。担当者は種類と ID の組で一つに定まり、login は識別に含めない。
// 担当者のいない保護ルールを、保護ルールがないこと (空集合) と別の値として持つ。GitHub はこの保護ルールを残すことがあり、
// 宣言した「担当者がいない」状態と同じとは確かめられないため。
// 担当者のいない保護ルールの印を立てるのは、担当者が空のときの ReviewerSetOfRule だけにし、印のある値が担当者を持たないことを保つ。
// フィールドを非公開にし、メソッドはどれも受け取った値を書き換えないため、値を複製せずに渡してよい。
type ReviewerSet struct {
	// reviewers は受け取った順のまま持つ。
	reviewers []Reviewer
	// logins は、観測した担当者の login。表示にだけ使う。
	logins               map[Reviewer]string
	ruleWithoutReviewers bool
}

// ReviewerSetOf は、担当者 reviewers の集合を並びのまま持つ値を作る。空なら、保護ルールがないことを表す。
// 呼び出し側があとで書き換えても値が変わらないように、複製して持つ。
func ReviewerSetOf(reviewers []Reviewer) ReviewerSet {
	return ReviewerSet{reviewers: slices.Clone(reviewers)}
}

// ReviewerSetOfRule は、required_reviewers の保護ルール rule の担当者の集合を作る。nil は保護ルールがないこと、担当者が空の保護ルールは担当者のいない保護ルールになる。
// 呼び出し側があとで書き換えても値が変わらないように、担当者と login を複製して持つ。
func ReviewerSetOfRule(rule *RequiredReviewers) ReviewerSet {
	if rule == nil {
		return ReviewerSet{}
	}
	return ReviewerSet{reviewers: slices.Clone(rule.Reviewers), logins: maps.Clone(rule.Logins), ruleWithoutReviewers: len(rule.Reviewers) == 0}
}

// Reviewers は、担当者を保った並びで返す。戻り値の書き換えが値に及ばないように、複製を返す。
func (s ReviewerSet) Reviewers() []Reviewer {
	return slices.Clone(s.reviewers)
}

// IsRuleWithoutReviewers は、s が担当者のいない保護ルールかを返す。
func (s ReviewerSet) IsRuleWithoutReviewers() bool {
	return s.ruleWithoutReviewers
}

// Login は、担当者 reviewer の観測した login を返す。持たなければ偽を返す。
func (s ReviewerSet) Login(reviewer Reviewer) (string, bool) {
	login, ok := s.logins[reviewer]
	return login, ok
}

// NotIn は、s の担当者のうち other にいない者を、s の並びと login を保った集合で返す。equal と同じく種類と ID の組で比べる。
// 外す担当者や加える担当者の一覧として使うため、担当者のいない保護ルールの印は引き継がない。
func (s ReviewerSet) NotIn(other ReviewerSet) ReviewerSet {
	var reviewers []Reviewer
	for _, reviewer := range s.reviewers {
		if !slices.Contains(other.reviewers, reviewer) {
			reviewers = append(reviewers, reviewer)
		}
	}
	return ReviewerSet{reviewers: reviewers, logins: s.logins}
}

// WithLoginsFrom は、s の担当者のうち other が login を持つ者に、その login を添えた値を返す。並びと担当者は s のまま変えない。
// 期待値は login を持たないため、表示で現在の担当者と同じ者を見分けられるよう、観測値の login を写すのに使う。
func (s ReviewerSet) WithLoginsFrom(other ReviewerSet) ReviewerSet {
	logins := maps.Clone(s.logins)
	for _, reviewer := range s.reviewers {
		if login, ok := other.logins[reviewer]; ok {
			if logins == nil {
				logins = map[Reviewer]string{}
			}
			logins[reviewer] = login
		}
	}
	s.logins = logins
	return s
}

// String は、担当者を [User 583231 (octocat), Team 42] の表記で、並びのまま返す。login は持つ担当者にだけ添える。
// 担当者のいない保護ルールは、空集合の [] と見分けられるよう注記を添える。
func (s ReviewerSet) String() string {
	formatted := make([]string, len(s.reviewers))
	for i, reviewer := range s.reviewers {
		formatted[i] = string(reviewer.Type) + " " + strconv.FormatInt(reviewer.ID, 10)
		if login, ok := s.logins[reviewer]; ok {
			formatted[i] += " (" + login + ")"
		}
	}
	text := "[" + strings.Join(formatted, ", ") + "]"
	if s.ruleWithoutReviewers {
		return text + " (protection rule without reviewers)"
	}
	return text
}

// equal は、担当者のいない保護ルールかどうかが同じで、担当者が種類と ID の組の集合として同じかを返す。並び、重複、login は比べない。
func (s ReviewerSet) equal(other ReviewerSet) bool {
	return s.ruleWithoutReviewers == other.ruleWithoutReviewers && slices.Equal(sortedDistinctReviewers(s.reviewers), sortedDistinctReviewers(other.reviewers))
}

// sortedDistinctReviewers は、保持する並びを表示に使うため、引数を書き換えずに写しを整列し、重複を除く。
func sortedDistinctReviewers(reviewers []Reviewer) []Reviewer {
	return slices.Compact(slices.SortedFunc(slices.Values(reviewers), func(a, b Reviewer) int {
		return cmp.Or(cmp.Compare(a.Type, b.Type), cmp.Compare(a.ID, b.ID))
	}))
}
