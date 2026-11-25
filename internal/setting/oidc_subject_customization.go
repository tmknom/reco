package setting

// OIDCSubjectCustomization は、OIDC トークンの subject クレームのカスタマイズである。3項目の組を一つの値として扱う。
type OIDCSubjectCustomization struct {
	UseDefault bool
	// IncludeClaimKeys は、subject を組み立てるクレームの key を、順序と重複を保ったまま並べる。
	// UseDefault が偽のときだけ意味を持ち、そのとき nil と長さ 0 は、どちらも空の並びを表す。
	IncludeClaimKeys    []string
	UseImmutableSubject bool
}
