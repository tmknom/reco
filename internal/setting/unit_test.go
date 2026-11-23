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
		{name: "repository variable follows the variables path", unit: Unit{Key: RepositoryVariable, Variable: mustVariableName(t, "GO_VERSION")}, want: "actions.variables.GO_VERSION"},
		{name: "environment variable follows the environment path", unit: Unit{Key: EnvironmentVariable, Environment: production, Variable: mustVariableName(t, "REGION")}, want: "environments.production.variables.REGION"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.unit.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestUnit_Same(t *testing.T) {
	production := mustEnvironmentName(t, "production")
	region := mustVariableName(t, "REGION")
	base := Unit{Key: EnvironmentVariable, Environment: production, Variable: region}
	cases := []struct {
		name  string
		other Unit
		want  bool
	}{
		{name: "identical unit is the same", other: base, want: true},
		{name: "environment name differing only in case is the same", other: Unit{Key: EnvironmentVariable, Environment: mustEnvironmentName(t, "Production"), Variable: region}, want: true},
		{name: "different environment name is another unit", other: Unit{Key: EnvironmentVariable, Environment: mustEnvironmentName(t, "staging"), Variable: region}, want: false},
		{name: "different variable name is another unit", other: Unit{Key: EnvironmentVariable, Environment: production, Variable: mustVariableName(t, "ZONE")}, want: false},
		{name: "different key is another unit", other: Unit{Key: RepositoryVariable, Variable: region}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := base.Same(tc.other); got != tc.want {
				t.Errorf("Same(%v) = %t, want %t", tc.other, got, tc.want)
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
		{name: "repository variable key without a name", unit: Unit{Key: RepositoryVariable}},
		{name: "repository setting with a variable name", unit: Unit{Key: AllowAutoMerge, Variable: mustVariableName(t, "X")}},
		{name: "environment variable key without an environment name", unit: Unit{Key: EnvironmentVariable, Variable: mustVariableName(t, "X")}},
		{name: "environment variable key without a variable name", unit: Unit{Key: EnvironmentVariable, Environment: mustEnvironmentName(t, "production")}},
		{name: "environment key with a variable name", unit: Unit{Key: EnvironmentResource, Environment: mustEnvironmentName(t, "production"), Variable: mustVariableName(t, "X")}},
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
