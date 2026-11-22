package setting

import (
	"slices"
	"testing"
)

func TestRepositorySettingUnits_FollowsConfigurationOrder(t *testing.T) {
	want := []Key{DeleteBranchOnMerge, AllowAutoMerge, ActionsEnabled, AllowedActions, SelectedActions, SHAPinningRequired, DefaultWorkflowPermissions, CanApprovePullRequestReviews, ActionsAccessLevel, DependabotAlerts, DependabotSecurityUpdates, SecretScanningEnabled, RepositoryPushProtection, PrivateVulnerabilityReportingEnabled}
	if got := RepositorySettingUnits(); !slices.Equal(got, want) {
		t.Errorf("RepositorySettingUnits() = %v, want %v", got, want)
	}
}

func TestKeyArea(t *testing.T) {
	tests := []struct {
		key  Key
		want Area
	}{
		{key: DeleteBranchOnMerge, want: AreaGeneral},
		{key: AllowAutoMerge, want: AreaGeneral},
		{key: ActionsEnabled, want: AreaActionsPermissions},
		{key: AllowedActions, want: AreaActionsPermissions},
		{key: SHAPinningRequired, want: AreaActionsPermissions},
		{key: SelectedActions, want: AreaActionsPermissions},
		{key: DefaultWorkflowPermissions, want: AreaWorkflowPermissions},
		{key: CanApprovePullRequestReviews, want: AreaWorkflowPermissions},
		{key: ActionsAccessLevel, want: AreaActionsAccess},
		{key: RepositoryVariable, want: AreaRepositoryVariables},
		{key: EnvironmentResource, want: AreaEnvironments},
		{key: EnvironmentDeploymentBranchPolicy, want: AreaEnvironments},
		{key: EnvironmentVariable, want: AreaEnvironmentVariables},
		{key: DependabotAlerts, want: AreaDependabot},
		{key: DependabotSecurityUpdates, want: AreaDependabot},
		{key: SecretScanningEnabled, want: AreaSecretScanning},
		{key: RepositoryPushProtection, want: AreaSecretScanning},
		{key: PrivateVulnerabilityReportingEnabled, want: AreaPrivateVulnerabilityReporting},
	}
	for _, tt := range tests {
		t.Run(string(tt.key), func(t *testing.T) {
			if got := tt.key.Area(); got != tt.want {
				t.Errorf("Area() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKey_HasEnvironmentName(t *testing.T) {
	tests := []struct {
		key  Key
		want bool
	}{
		{key: EnvironmentResource, want: true},
		{key: EnvironmentDeploymentBranchPolicy, want: true},
		{key: EnvironmentVariable, want: true},
		{key: RepositoryVariable, want: false},
		{key: AllowAutoMerge, want: false},
	}
	for _, tt := range tests {
		t.Run(string(tt.key), func(t *testing.T) {
			if got := tt.key.HasEnvironmentName(); got != tt.want {
				t.Errorf("HasEnvironmentName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKeyArea_UnknownKeyPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Area() did not panic for an unknown key")
		}
	}()
	Key("general.unknown").Area()
}
