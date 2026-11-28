package github

import "testing"

//trace:checks FR-004 QR-002
func TestSettingObservation(t *testing.T) {
	t.Run("observed false is an observed value", func(t *testing.T) {
		got, ok := Observed(false).Value()
		if got || !ok {
			t.Errorf("Observed(false).Value() = (%t, %t), want (false, true)", got, ok)
		}
	})

	t.Run("unavailable has no value and keeps its reason", func(t *testing.T) {
		o := Unavailable[bool](UnavailableNotSelected)
		if got, ok := o.Value(); got || ok {
			t.Errorf("Value() = (%t, %t), want (false, false)", got, ok)
		}
		if got := o.UnavailableReason(); got != UnavailableNotSelected {
			t.Errorf("UnavailableReason() = %v, want %v", got, UnavailableNotSelected)
		}
	})

	t.Run("reason of an observed value panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("UnavailableReason() did not panic for an observed value")
			}
		}()
		_ = Observed(true).UnavailableReason()
	})

	t.Run("zero value is neither observed nor unavailable", func(t *testing.T) {
		var o SettingObservation[bool]
		if _, ok := o.Value(); ok {
			t.Error("Value() of the zero value reports an observed value")
		}
		defer func() {
			if recover() == nil {
				t.Error("UnavailableReason() did not panic for the zero value")
			}
		}()
		_ = o.UnavailableReason()
	})
}

func TestSettingObservation_HasUnknownValue(t *testing.T) {
	cases := []struct {
		name string
		in   SettingObservation[bool]
		want bool
	}{
		{name: "observed value is not an unknown value", in: Observed(false), want: false},
		{name: "unavailable because of an unknown value is an unknown value", in: Unavailable[bool](UnavailableUnknownValue), want: true},
		{name: "unavailable for another reason is not an unknown value", in: Unavailable[bool](UnavailableActionsDisabled), want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.in.HasUnknownValue(); got != tc.want {
				t.Errorf("HasUnknownValue() = %t, want %t", got, tc.want)
			}
		})
	}

	t.Run("zero value panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("HasUnknownValue() did not panic for the zero value")
			}
		}()
		var o SettingObservation[bool]
		_ = o.HasUnknownValue()
	})
}
