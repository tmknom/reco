package setting

import "testing"

func TestDependabotSecurityUpdatesState_Enabled(t *testing.T) {
	tests := []struct {
		state DependabotSecurityUpdatesState
		want  bool
	}{
		{state: DependabotSecurityUpdatesDisabled, want: false},
		{state: DependabotSecurityUpdatesEnabled, want: true},
		{state: DependabotSecurityUpdatesPaused, want: true},
	}
	for _, tt := range tests {
		if got := tt.state.Enabled(); got != tt.want {
			t.Errorf("%d.Enabled() = %t, want %t", tt.state, got, tt.want)
		}
	}
}

func TestDependabotSecurityUpdatesStateOf(t *testing.T) {
	tests := []struct {
		enabled bool
		want    DependabotSecurityUpdatesState
	}{
		{enabled: true, want: DependabotSecurityUpdatesEnabled},
		{enabled: false, want: DependabotSecurityUpdatesDisabled},
	}
	for _, tt := range tests {
		if got := DependabotSecurityUpdatesStateOf(tt.enabled); got != tt.want {
			t.Errorf("DependabotSecurityUpdatesStateOf(%t) = %d, want %d", tt.enabled, got, tt.want)
		}
	}
}
