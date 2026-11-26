package setting

import (
	"slices"
	"strconv"
	"strings"
)

// OIDCSubjectCustomization は、OIDC トークンの subject クレームのカスタマイズである。3項目の組を一つの値として扱う。
type OIDCSubjectCustomization struct {
	UseDefault bool
	// IncludeClaimKeys は、subject を組み立てるクレームの key を、順序と重複を保ったまま並べる。
	// UseDefault が偽のときだけ意味を持ち、そのとき nil と長さ 0 は、どちらも空の並びを表す。
	IncludeClaimKeys []string
	// UseImmutableSubject は省略できる項目で、nil は期待値がこの項目を宣言していないことを表す。観測値は必ず値を持つ。
	UseImmutableSubject *bool
}

// clone は、呼び出し側の書き換えが値に及ばないよう、key の並びと UseImmutableSubject の指す先を複製した値を返す。
func (c OIDCSubjectCustomization) clone() OIDCSubjectCustomization {
	c.IncludeClaimKeys = slices.Clone(c.IncludeClaimKeys)
	if c.UseImmutableSubject != nil {
		immutable := *c.UseImmutableSubject
		c.UseImmutableSubject = &immutable
	}
	return c
}

// equal は、二つの値が同じ値かどうかを返す。key の並びは順序と重複まで比べ、UseDefault が真どうしでは GitHub が key を保存しないため比べない。
// UseImmutableSubject は、省略した項目を比べないよう、両方が持つときだけ比べる。
func (c OIDCSubjectCustomization) equal(other OIDCSubjectCustomization) bool {
	if c.UseDefault != other.UseDefault {
		return false
	}
	if !c.UseDefault && !slices.Equal(c.IncludeClaimKeys, other.IncludeClaimKeys) {
		return false
	}
	if c.UseImmutableSubject == nil || other.UseImmutableSubject == nil {
		return true
	}
	return *c.UseImmutableSubject == *other.UseImmutableSubject
}

// format は、変更計画に示す1行の表記を返す。観測した key は形式を確かめていないため、改行などが行を偽装しないよう、エスケープして引用符で囲む。
func (c OIDCSubjectCustomization) format() string {
	var b strings.Builder
	b.WriteString("use_default: " + strconv.FormatBool(c.UseDefault))
	if !c.UseDefault {
		quoted := make([]string, len(c.IncludeClaimKeys))
		for i, key := range c.IncludeClaimKeys {
			quoted[i] = strconv.Quote(key)
		}
		b.WriteString(", include_claim_keys: [" + strings.Join(quoted, ", ") + "]")
	}
	if c.UseImmutableSubject != nil {
		b.WriteString(", use_immutable_subject: " + strconv.FormatBool(*c.UseImmutableSubject))
	}
	return b.String()
}
