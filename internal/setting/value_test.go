package setting

import "testing"

func TestValue_String(t *testing.T) {
	cases := []struct {
		value Value
		want  string
	}{
		{value: BoolValue(true), want: "true"},
		{value: PermissionLevelValue(PermissionRead), want: "read"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.value.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
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
