package configuration

import (
	"reflect"
	"testing"

	"github.com/tmknom/reco/internal/setting"
)

//trace:checks FR-001
func TestDesiredState_Value(t *testing.T) {
	cases := []struct {
		name    string
		desired DesiredState
		key     setting.Key
		want    setting.Value
		wantOK  bool
	}{
		{name: "declared delete_branch_on_merge returns its desired value", desired: DesiredState{General: General{DeleteBranchOnMerge: new(false)}}, key: setting.DeleteBranchOnMerge, want: setting.BoolValue(false), wantOK: true},
		{name: "declared allow_auto_merge returns its desired value", desired: DesiredState{General: General{AllowAutoMerge: new(true)}}, key: setting.AllowAutoMerge, want: setting.BoolValue(true), wantOK: true},
		{name: "declared default_workflow_permissions returns its desired value", desired: DesiredState{WorkflowPermissions: WorkflowPermissions{DefaultWorkflowPermissions: new(setting.PermissionRead)}}, key: setting.DefaultWorkflowPermissions, want: setting.PermissionLevelValue(setting.PermissionRead), wantOK: true},
		{name: "declared can_approve_pull_request_reviews returns its desired value", desired: DesiredState{WorkflowPermissions: WorkflowPermissions{CanApprovePullRequestReviews: new(false)}}, key: setting.CanApprovePullRequestReviews, want: setting.BoolValue(false), wantOK: true},
		{name: "declared allowed_actions returns its desired value", desired: DesiredState{ActionsPermissions: ActionsPermissions{AllowedActions: new(setting.AllowedActionsLocalOnly)}}, key: setting.AllowedActions, want: setting.AllowedActionsPolicyValue(setting.AllowedActionsLocalOnly), wantOK: true},
		{name: "declared selected_actions returns its desired value with empty patterns kept non-nil", desired: DesiredState{ActionsPermissions: ActionsPermissions{AllowedActions: new(setting.AllowedActionsSelected), SelectedActions: &setting.SelectedActionsCriteria{VerifiedAllowed: true, PatternsAllowed: []string{}}}}, key: setting.SelectedActions, want: setting.SelectedActionsCriteriaValue(setting.SelectedActionsCriteria{VerifiedAllowed: true, PatternsAllowed: []string{}}), wantOK: true},
		{name: "declared enabled returns its desired value", desired: DesiredState{ActionsPermissions: ActionsPermissions{Enabled: new(false)}}, key: setting.ActionsEnabled, want: setting.BoolValue(false), wantOK: true},
		{name: "declared sha_pinning_required returns its desired value", desired: DesiredState{ActionsPermissions: ActionsPermissions{SHAPinningRequired: new(false)}}, key: setting.SHAPinningRequired, want: setting.BoolValue(false), wantOK: true},
		{name: "declared access_level returns its desired value", desired: DesiredState{ActionsAccess: ActionsAccess{AccessLevel: new(setting.AccessNone)}}, key: setting.ActionsAccessLevel, want: setting.AccessLevelValue(setting.AccessNone), wantOK: true},
		{name: "omitted access_level is outside the management scope", desired: DesiredState{WorkflowPermissions: WorkflowPermissions{CanApprovePullRequestReviews: new(true)}}, key: setting.ActionsAccessLevel, wantOK: false},
		{name: "omitted sha_pinning_required is outside the management scope", desired: DesiredState{ActionsPermissions: ActionsPermissions{AllowedActions: new(setting.AllowedActionsAll)}}, key: setting.SHAPinningRequired, wantOK: false},
		{name: "omitted allowed_actions is outside the management scope", desired: DesiredState{WorkflowPermissions: WorkflowPermissions{CanApprovePullRequestReviews: new(true)}}, key: setting.AllowedActions, wantOK: false},
		{name: "omitted selected_actions is outside the management scope", desired: DesiredState{ActionsPermissions: ActionsPermissions{AllowedActions: new(setting.AllowedActionsAll)}}, key: setting.SelectedActions, wantOK: false},
		{name: "omitted delete_branch_on_merge is outside the management scope", desired: DesiredState{General: General{AllowAutoMerge: new(true)}}, key: setting.DeleteBranchOnMerge, wantOK: false},
		{name: "omitted allow_auto_merge is outside the management scope", desired: DesiredState{General: General{DeleteBranchOnMerge: new(true)}}, key: setting.AllowAutoMerge, wantOK: false},
		{name: "omitted default_workflow_permissions is outside the management scope", desired: DesiredState{WorkflowPermissions: WorkflowPermissions{CanApprovePullRequestReviews: new(true)}}, key: setting.DefaultWorkflowPermissions, wantOK: false},
		{name: "declared dependabot_alerts returns its desired value", desired: DesiredState{Dependabot: Dependabot{Alerts: new(false)}}, key: setting.DependabotAlerts, want: setting.BoolValue(false), wantOK: true},
		{name: "declared dependabot_security_updates true returns enabled security updates", desired: DesiredState{Dependabot: Dependabot{SecurityUpdates: new(true)}}, key: setting.DependabotSecurityUpdates, want: setting.DependabotSecurityUpdatesValue(setting.DependabotSecurityUpdatesEnabled), wantOK: true},
		{name: "omitted dependabot_alerts is outside the management scope", desired: DesiredState{Dependabot: Dependabot{SecurityUpdates: new(true)}}, key: setting.DependabotAlerts, wantOK: false},
		{name: "omitted dependabot_security_updates is outside the management scope", desired: DesiredState{Dependabot: Dependabot{Alerts: new(true)}}, key: setting.DependabotSecurityUpdates, wantOK: false},
		{name: "declared secret_scanning returns its desired value", desired: DesiredState{SecretScanning: SecretScanning{Enabled: new(false)}}, key: setting.SecretScanningEnabled, want: setting.BoolValue(false), wantOK: true},
		{name: "declared push protection returns its desired value", desired: DesiredState{SecretScanning: SecretScanning{PushProtection: new(true)}}, key: setting.RepositoryPushProtection, want: setting.BoolValue(true), wantOK: true},
		{name: "omitted secret_scanning is outside the management scope", desired: DesiredState{SecretScanning: SecretScanning{PushProtection: new(true)}}, key: setting.SecretScanningEnabled, wantOK: false},
		{name: "omitted push protection is outside the management scope", desired: DesiredState{SecretScanning: SecretScanning{Enabled: new(true)}}, key: setting.RepositoryPushProtection, wantOK: false},
		{name: "declared private_vulnerability_reporting returns its desired value", desired: DesiredState{PrivateVulnerabilityReporting: PrivateVulnerabilityReporting{Enabled: new(false)}}, key: setting.PrivateVulnerabilityReportingEnabled, want: setting.BoolValue(false), wantOK: true},
		{name: "omitted private_vulnerability_reporting is outside the management scope", desired: DesiredState{SecretScanning: SecretScanning{Enabled: new(true)}}, key: setting.PrivateVulnerabilityReportingEnabled, wantOK: false},
		{name: "omitted can_approve_pull_request_reviews is outside the management scope", desired: DesiredState{WorkflowPermissions: WorkflowPermissions{DefaultWorkflowPermissions: new(setting.PermissionRead)}}, key: setting.CanApprovePullRequestReviews, wantOK: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := tc.desired.Value(tc.key)
			if !reflect.DeepEqual(got, tc.want) || ok != tc.wantOK {
				t.Errorf("Value(%q) = %v, %v, want %v, %v", tc.key, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

// TestDesiredState_ValueCoversEveryDesiredValue は、期待状態が持つ期待値の数と、Value が返す期待値の数が食い違わないことを確かめる。
// 期待状態に設定を足して setting.RepositorySettingUnits や Value に足し忘れると、宣言を受理した設定が照合されないまま
// コンパイルもほかのテストも通るため、期待状態の全フィールドを埋めて数える。
func TestDesiredState_ValueCoversEveryDesiredValue(t *testing.T) {
	var desired DesiredState
	declared := fillDesiredValues(reflect.ValueOf(&desired).Elem())

	n := 0
	for _, key := range setting.RepositorySettingUnits() {
		if _, ok := desired.Value(key); ok {
			n++
		}
	}
	if n != declared {
		t.Errorf("Value() returned %d desired values for %d declared settings", n, declared)
	}
}

// 関連リソースに関わる管理の単位は名前との組で識別するため、Value では引けない。
func TestDesiredState_ValueOfRelatedResourceSettingPanics(t *testing.T) {
	for _, key := range []setting.Key{setting.RepositoryVariable, setting.EnvironmentResource, setting.EnvironmentDeploymentBranchPolicy, setting.EnvironmentVariable} {
		t.Run(string(key), func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("Value(%q) did not panic", key)
				}
			}()
			_, _ = DesiredState{}.Value(key)
		})
	}
}

func TestDesiredState_ValueOfUnknownSettingPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Value() did not panic for an unknown setting")
		}
	}()
	_, _ = DesiredState{}.Value("general.unknown")
}

// fillDesiredValues は、v の中の期待値 (ポインタのフィールド) をすべてゼロ値への参照で埋め、その数を返す。
// 列挙値のゼロ値は定義外の値だが、ここでは期待値の有無だけを数える。
// 関連リソースの宣言 (RepositoryVariables、Environments) は Value で引かない期待値のため、数えない。
func fillDesiredValues(v reflect.Value) int {
	n := 0
	for structField, field := range v.Fields() {
		if structField.Name == "RepositoryVariables" || structField.Name == "Environments" {
			continue
		}
		switch field.Kind() {
		case reflect.Pointer:
			field.Set(reflect.New(field.Type().Elem()))
			n++
		case reflect.Struct:
			n += fillDesiredValues(field)
		default:
			panic("unexpected field kind in desired state: " + field.Kind().String())
		}
	}
	return n
}
