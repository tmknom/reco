package setting

import "testing"

//trace:checks FR-005
func TestOIDCSubjectCustomization_Equal(t *testing.T) {
	custom := func(keys ...string) OIDCSubjectCustomization {
		return OIDCSubjectCustomization{UseDefault: false, IncludeClaimKeys: keys}
	}
	withImmutable := func(c OIDCSubjectCustomization, immutable bool) OIDCSubjectCustomization {
		c.UseImmutableSubject = &immutable
		return c
	}
	defaultForm := OIDCSubjectCustomization{UseDefault: true}
	cases := []struct {
		name  string
		a, b  OIDCSubjectCustomization
		equal bool
	}{
		{name: "default forms are equal", a: defaultForm, b: defaultForm, equal: true},
		{name: "default forms are equal even if one holds claim keys", a: defaultForm, b: OIDCSubjectCustomization{UseDefault: true, IncludeClaimKeys: []string{"repo"}}, equal: true},
		{name: "default form differs from a customized form", a: defaultForm, b: custom("repo"), equal: false},
		{name: "same claim keys are equal", a: custom("repo", "context"), b: custom("repo", "context"), equal: true},
		{name: "claim keys differing only in order differ", a: custom("repo", "context"), b: custom("context", "repo"), equal: false},
		{name: "claim keys differing only in duplication differ", a: custom("repo"), b: custom("repo", "repo"), equal: false},
		{name: "nil and empty claim keys are equal", a: OIDCSubjectCustomization{IncludeClaimKeys: nil}, b: OIDCSubjectCustomization{IncludeClaimKeys: []string{}}, equal: true},
		{name: "claim keys differing only in letter case differ", a: custom("repo"), b: custom("Repo"), equal: false},
		{name: "same immutable subjects held by different pointers are equal", a: withImmutable(defaultForm, true), b: withImmutable(defaultForm, true), equal: true},
		{name: "different immutable subjects differ", a: withImmutable(defaultForm, true), b: withImmutable(defaultForm, false), equal: false},
		{name: "immutable subject held by only the receiver is not compared", a: withImmutable(defaultForm, true), b: defaultForm, equal: true},
		{name: "immutable subject held by only the argument is not compared", a: defaultForm, b: withImmutable(defaultForm, false), equal: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.equal(tc.b); got != tc.equal {
				t.Errorf("equal() = %v, want %v", got, tc.equal)
			}
		})
	}
}

// subject のカスタマイズの値を1行で示す表記を確かめる。
func TestOIDCSubjectCustomization_Format(t *testing.T) {
	immutable := func(b bool) *bool { return &b }
	cases := []struct {
		name  string
		value OIDCSubjectCustomization
		want  string
	}{
		{name: "default form shows no claim keys", value: OIDCSubjectCustomization{UseDefault: true, IncludeClaimKeys: []string{"repo"}}, want: "use_default: true"},
		{name: "customized form without claim keys shows an empty sequence", value: OIDCSubjectCustomization{UseDefault: false}, want: "use_default: false, include_claim_keys: []"},
		{name: "claim keys keep their order and are quoted", value: OIDCSubjectCustomization{UseDefault: false, IncludeClaimKeys: []string{"repo", "a\nb"}}, want: `use_default: false, include_claim_keys: ["repo", "a\nb"]`},
		{name: "false immutable subject is shown", value: OIDCSubjectCustomization{UseDefault: true, UseImmutableSubject: immutable(false)}, want: "use_default: true, use_immutable_subject: false"},
		{name: "immutable subject follows claim keys", value: OIDCSubjectCustomization{UseDefault: false, IncludeClaimKeys: []string{"context"}, UseImmutableSubject: immutable(true)}, want: `use_default: false, include_claim_keys: ["context"], use_immutable_subject: true`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.value.format(); got != tc.want {
				t.Errorf("format() = %q, want %q", got, tc.want)
			}
		})
	}
}
