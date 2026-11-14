package setting

import (
	"reflect"
	"slices"
	"testing"
)

func TestValue_String(t *testing.T) {
	cases := []struct {
		value Value
		want  string
	}{
		{value: BoolValue(true), want: "true"},
		{value: PermissionLevelValue(PermissionRead), want: "read"},
		{value: AllowedActionsPolicyValue(AllowedActionsSelected), want: "selected"},
		{
			value: SelectedActionsCriteriaValue(SelectedActionsCriteria{GitHubOwnedAllowed: true, VerifiedAllowed: false, PatternsAllowed: []string{"z/*", "a/*"}}),
			want:  `github_owned_allowed: true, verified_allowed: false, patterns_allowed: ["z/*", "a/*"]`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.value.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}

//trace:checks FR-005
func TestValue_Equal(t *testing.T) {
	criteria := func(patterns ...string) Value {
		return SelectedActionsCriteriaValue(SelectedActionsCriteria{GitHubOwnedAllowed: true, VerifiedAllowed: true, PatternsAllowed: patterns})
	}
	cases := []struct {
		name  string
		a, b  Value
		equal bool
	}{
		{name: "same booleans are equal", a: BoolValue(true), b: BoolValue(true), equal: true},
		{name: "different booleans differ", a: BoolValue(true), b: BoolValue(false), equal: false},
		{name: "same permission levels are equal", a: PermissionLevelValue(PermissionRead), b: PermissionLevelValue(PermissionRead), equal: true},
		{name: "different permission levels differ", a: PermissionLevelValue(PermissionRead), b: PermissionLevelValue(PermissionWrite), equal: false},
		{name: "same policies are equal", a: AllowedActionsPolicyValue(AllowedActionsAll), b: AllowedActionsPolicyValue(AllowedActionsAll), equal: true},
		{name: "different policies differ", a: AllowedActionsPolicyValue(AllowedActionsAll), b: AllowedActionsPolicyValue(AllowedActionsSelected), equal: false},
		{name: "criteria differing only in pattern order are equal", a: criteria("a/*", "b/*"), b: criteria("b/*", "a/*"), equal: true},
		{name: "criteria with a different pattern differ", a: criteria("a/*"), b: criteria("b/*"), equal: false},
		{name: "values of different kinds differ", a: BoolValue(false), b: SelectedActionsCriteriaValue(SelectedActionsCriteria{}), equal: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.Equal(tc.b); got != tc.equal {
				t.Errorf("Equal() = %v, want %v", got, tc.equal)
			}
		})
	}
}

func TestValue_EqualWithZeroValuePanics(t *testing.T) {
	cases := []struct {
		name string
		a, b Value
	}{
		{name: "zero value on the receiver side", a: Value{}, b: SelectedActionsCriteriaValue(SelectedActionsCriteria{})},
		{name: "zero value on the argument side", a: SelectedActionsCriteriaValue(SelectedActionsCriteria{}), b: Value{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("Equal() did not panic for the zero value")
				}
			}()
			_ = tc.a.Equal(tc.b)
		})
	}
}

// Value は許可パターンのスライスを持つため == で比べられない。比較可能な表現に戻ると、
// 順序を無視しない == が Equal の代わりに黙って使えるようになる。
func TestValue_IsNotComparable(t *testing.T) {
	if reflect.TypeFor[Value]().Comparable() {
		t.Error("Value is comparable, want == to be a compile error")
	}
}

func TestValue_StringOfZeroValuePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("String() did not panic for the zero value")
		}
	}()
	_ = Value{}.String()
}

func TestValue_Bool(t *testing.T) {
	for _, want := range []bool{true, false} {
		if got := BoolValue(want).Bool(); got != want {
			t.Errorf("BoolValue(%v).Bool() = %v, want %v", want, got, want)
		}
	}
}

func TestValue_BoolOfPermissionLevelPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Bool() did not panic for a permission level value")
		}
	}()
	_ = PermissionLevelValue(PermissionWrite).Bool()
}

func TestValue_PermissionLevel(t *testing.T) {
	for _, want := range []PermissionLevel{PermissionRead, PermissionWrite} {
		if got := PermissionLevelValue(want).PermissionLevel(); got != want {
			t.Errorf("PermissionLevelValue(%q).PermissionLevel() = %q, want %q", want, got, want)
		}
	}
}

func TestValue_PermissionLevelOfBoolPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("PermissionLevel() did not panic for a boolean value")
		}
	}()
	_ = BoolValue(true).PermissionLevel()
}

func TestValue_AllowedActionsPolicy(t *testing.T) {
	for _, want := range []AllowedActionsPolicy{AllowedActionsAll, AllowedActionsSelected} {
		if got := AllowedActionsPolicyValue(want).AllowedActionsPolicy(); got != want {
			t.Errorf("AllowedActionsPolicyValue(%q).AllowedActionsPolicy() = %q, want %q", want, got, want)
		}
	}
}

func TestValue_AllowedActionsPolicyOfBoolPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("AllowedActionsPolicy() did not panic for a boolean value")
		}
	}()
	_ = BoolValue(true).AllowedActionsPolicy()
}

func TestValue_SelectedActionsCriteria(t *testing.T) {
	want := SelectedActionsCriteria{GitHubOwnedAllowed: true, VerifiedAllowed: false, PatternsAllowed: []string{"z/*", "a/*"}}
	if got := SelectedActionsCriteriaValue(want).SelectedActionsCriteria(); !reflect.DeepEqual(got, want) {
		t.Errorf("SelectedActionsCriteria() = %+v, want %+v", got, want)
	}
}

func TestValue_SelectedActionsCriteriaOfBoolPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("SelectedActionsCriteria() did not panic for a boolean value")
		}
	}()
	_ = BoolValue(true).SelectedActionsCriteria()
}

func TestValue_SelectedActionsCriteriaIsolatesPatterns(t *testing.T) {
	want := []string{"a/*", "b/*"}

	t.Run("writing to the slice given to the constructor does not change the value", func(t *testing.T) {
		given := slices.Clone(want)
		v := SelectedActionsCriteriaValue(SelectedActionsCriteria{PatternsAllowed: given})
		given[0] = "changed"
		if got := v.SelectedActionsCriteria().PatternsAllowed; !slices.Equal(got, want) {
			t.Errorf("PatternsAllowed = %q, want %q", got, want)
		}
	})

	t.Run("writing to the slice returned by the accessor does not change the value", func(t *testing.T) {
		v := SelectedActionsCriteriaValue(SelectedActionsCriteria{PatternsAllowed: slices.Clone(want)})
		v.SelectedActionsCriteria().PatternsAllowed[0] = "changed"
		if got := v.SelectedActionsCriteria().PatternsAllowed; !slices.Equal(got, want) {
			t.Errorf("PatternsAllowed = %q, want %q", got, want)
		}
	})
}
