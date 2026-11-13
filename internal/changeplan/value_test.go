package changeplan

import (
	"testing"

	"github.com/tmknom/reco/internal/setting"
)

func TestSettingValue_String(t *testing.T) {
	cases := []struct {
		value SettingValue
		want  string
	}{
		{value: BoolValue(true), want: "true"},
		{value: BoolValue(false), want: "false"},
		{value: PermissionLevelValue(setting.PermissionRead), want: "read"},
		{value: PermissionLevelValue(setting.PermissionWrite), want: "write"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.value.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSettingValue_StringOfZeroValuePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("String() did not panic for the zero value")
		}
	}()
	_ = SettingValue{}.String()
}

func TestSettingValue_Bool(t *testing.T) {
	for _, want := range []bool{true, false} {
		if got := BoolValue(want).Bool(); got != want {
			t.Errorf("BoolValue(%v).Bool() = %v, want %v", want, got, want)
		}
	}
}

func TestSettingValue_BoolOfPermissionLevelPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Bool() did not panic for a permission level value")
		}
	}()
	_ = PermissionLevelValue(setting.PermissionWrite).Bool()
}

func TestSettingValue_PermissionLevel(t *testing.T) {
	for _, want := range []setting.PermissionLevel{setting.PermissionRead, setting.PermissionWrite} {
		if got := PermissionLevelValue(want).PermissionLevel(); got != want {
			t.Errorf("PermissionLevelValue(%q).PermissionLevel() = %q, want %q", want, got, want)
		}
	}
}

func TestSettingValue_PermissionLevelOfBoolPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("PermissionLevel() did not panic for a boolean value")
		}
	}()
	_ = BoolValue(true).PermissionLevel()
}
