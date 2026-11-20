package setting

import "testing"

func TestUnit_String(t *testing.T) {
	production := mustEnvironmentName(t, "production")
	cases := []struct {
		name string
		unit Unit
		want string
	}{
		{name: "repository setting is its key", unit: Unit{Key: AllowAutoMerge}, want: "general.allow_auto_merge"},
		{name: "environment existence is the environment path", unit: Unit{Key: EnvironmentResource, Environment: production}, want: "environments.production"},
		{name: "setting in an environment follows the environment path", unit: Unit{Key: EnvironmentDeploymentBranchPolicy, Environment: production}, want: "environments.production.deployment_branch_policy"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.unit.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestUnit_StringPanicsOnMismatchedName(t *testing.T) {
	cases := []struct {
		name string
		unit Unit
	}{
		{name: "environment key without a name", unit: Unit{Key: EnvironmentResource}},
		{name: "repository setting with a name", unit: Unit{Key: AllowAutoMerge, Environment: mustEnvironmentName(t, "production")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("String() did not panic")
				}
			}()
			_ = tc.unit.String()
		})
	}
}
