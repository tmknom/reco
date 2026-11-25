package configuration

import (
	"reflect"
	"testing"

	"github.com/tmknom/reco/internal/setting"
)

// TestDesiredState_ManagedSettings は、管理範囲の管理設定を期待値とともに、リポジトリ設定、Repository Variable、Environment ごとの順に列挙することを確かめる。
//
//trace:checks FR-001
func TestDesiredState_ManagedSettings(t *testing.T) {
	production, staging := mustEnvironmentName(t, "production"), mustEnvironmentName(t, "staging")
	region, debug := mustVariableName(t, "REGION"), mustVariableName(t, "DEBUG")
	protected := setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyProtectedBranches}

	t.Run("declared settings are listed with their desired values in the order of the management units and the declarations", func(t *testing.T) {
		desired := DesiredState{
			Dependabot:          Dependabot{Alerts: new(false)},
			General:             General{AllowAutoMerge: new(true)},
			RepositoryVariables: []VariableDeclaration{{Name: region, Desired: setting.VariableValue("eu")}, {Name: debug, Desired: setting.VariableValue("")}},
			Environments: []EnvironmentDeclaration{
				{Name: staging, DeploymentBranchPolicy: &protected, Variables: []VariableDeclaration{{Name: region, Desired: setting.VariableValue("us")}}},
				{Name: production},
			},
		}
		want := []ManagedSetting{
			{Unit: setting.Unit{Key: setting.AllowAutoMerge}, Desired: setting.BoolValue(true)},
			{Unit: setting.Unit{Key: setting.DependabotAlerts}, Desired: setting.BoolValue(false)},
			{Unit: setting.Unit{Key: setting.RepositoryVariable, Variable: region}, Desired: setting.VariableValue("eu")},
			{Unit: setting.Unit{Key: setting.RepositoryVariable, Variable: debug}, Desired: setting.VariableValue("")},
			{Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: staging}, Desired: setting.Present()},
			{Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: staging}, Desired: setting.DeploymentBranchPolicyValue(protected)},
			{Unit: setting.Unit{Key: setting.EnvironmentVariable, Environment: staging, Variable: region}, Desired: setting.VariableValue("us")},
			{Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: production}, Desired: setting.Present()},
		}
		if got := desired.ManagedSettings(); !reflect.DeepEqual(got, want) {
			t.Errorf("ManagedSettings() =\n%+v\nwant\n%+v", got, want)
		}
	})

	t.Run("rulesets follow the environments in the declared order with their desired values", func(t *testing.T) {
		zeta, alpha := mustRulesetName(t, "zeta"), mustRulesetName(t, "alpha")
		desired := DesiredState{
			Environments: []EnvironmentDeclaration{{Name: production}},
			Rulesets: []RulesetDeclaration{
				{Name: zeta, Desired: setting.RulesetValue(baseRuleset())},
				{Name: alpha, Desired: setting.RulesetValue(setting.Ruleset{Target: setting.RulesetTargetBranch, Enforcement: setting.RulesetEnforcementDisabled})},
			},
		}
		want := []ManagedSetting{
			{Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: production}, Desired: setting.Present()},
			{Unit: setting.Unit{Key: setting.RulesetResource, Ruleset: zeta}, Desired: setting.RulesetValue(baseRuleset())},
			{Unit: setting.Unit{Key: setting.RulesetResource, Ruleset: alpha}, Desired: setting.RulesetValue(setting.Ruleset{Target: setting.RulesetTargetBranch, Enforcement: setting.RulesetEnforcementDisabled})},
		}
		if got := desired.ManagedSettings(); !reflect.DeepEqual(got, want) {
			t.Errorf("ManagedSettings() =\n%+v\nwant\n%+v", got, want)
		}
	})

	t.Run("nothing declared lists no managed settings", func(t *testing.T) {
		if got := (DesiredState{}).ManagedSettings(); len(got) != 0 {
			t.Errorf("ManagedSettings() = %+v, want none", got)
		}
	})
}

//trace:checks FR-001
func TestDesiredState_value(t *testing.T) {
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
		{name: "declared approval_policy returns its desired value", desired: DesiredState{ForkPullRequestApproval: ForkPullRequestApproval{ApprovalPolicy: new(setting.ForkPullRequestApprovalFirstTimeContributors)}}, key: setting.ForkPullRequestApproval, want: setting.ForkPullRequestApprovalPolicyValue(setting.ForkPullRequestApprovalFirstTimeContributors), wantOK: true},
		{name: "declared private_repositories returns its desired value", desired: DesiredState{PrivateForkPullRequestWorkflows: PrivateForkPullRequestWorkflows{Conditions: &setting.PrivateForkPullRequestWorkflowConditions{RunWorkflowsFromForkPullRequests: true, SendSecretsAndVariables: true}}}, key: setting.PrivateForkPullRequestWorkflows, want: setting.PrivateForkPullRequestWorkflowConditionsValue(setting.PrivateForkPullRequestWorkflowConditions{RunWorkflowsFromForkPullRequests: true, SendSecretsAndVariables: true}), wantOK: true},
		{name: "omitted private_repositories is outside the management scope", desired: DesiredState{ForkPullRequestApproval: ForkPullRequestApproval{ApprovalPolicy: new(setting.ForkPullRequestApprovalFirstTimeContributors)}}, key: setting.PrivateForkPullRequestWorkflows, wantOK: false},
		{name: "omitted approval_policy is outside the management scope", desired: DesiredState{ActionsAccess: ActionsAccess{AccessLevel: new(setting.AccessNone)}}, key: setting.ForkPullRequestApproval, wantOK: false},
		{name: "omitted sha_pinning_required is outside the management scope", desired: DesiredState{ActionsPermissions: ActionsPermissions{AllowedActions: new(setting.AllowedActionsAll)}}, key: setting.SHAPinningRequired, wantOK: false},
		{name: "omitted allowed_actions is outside the management scope", desired: DesiredState{WorkflowPermissions: WorkflowPermissions{CanApprovePullRequestReviews: new(true)}}, key: setting.AllowedActions, wantOK: false},
		{name: "omitted selected_actions is outside the management scope", desired: DesiredState{ActionsPermissions: ActionsPermissions{AllowedActions: new(setting.AllowedActionsAll)}}, key: setting.SelectedActions, wantOK: false},
		{name: "declared allow_squash_merge returns its desired value", desired: DesiredState{General: General{AllowSquashMerge: new(true)}}, key: setting.AllowSquashMerge, want: setting.BoolValue(true), wantOK: true},
		{name: "declared allow_merge_commit returns its desired value", desired: DesiredState{General: General{AllowMergeCommit: new(true)}}, key: setting.AllowMergeCommit, want: setting.BoolValue(true), wantOK: true},
		{name: "declared allow_rebase_merge returns its desired value", desired: DesiredState{General: General{AllowRebaseMerge: new(true)}}, key: setting.AllowRebaseMerge, want: setting.BoolValue(true), wantOK: true},
		{name: "declared allow_update_branch returns its desired value", desired: DesiredState{General: General{AllowUpdateBranch: new(true)}}, key: setting.AllowUpdateBranch, want: setting.BoolValue(true), wantOK: true},
		{name: "omitted allow_squash_merge is outside the management scope", desired: DesiredState{General: General{AllowMergeCommit: new(true)}}, key: setting.AllowSquashMerge, wantOK: false},
		{name: "omitted allow_merge_commit is outside the management scope", desired: DesiredState{General: General{AllowRebaseMerge: new(true)}}, key: setting.AllowMergeCommit, wantOK: false},
		{name: "omitted allow_rebase_merge is outside the management scope", desired: DesiredState{General: General{AllowUpdateBranch: new(true)}}, key: setting.AllowRebaseMerge, wantOK: false},
		{name: "omitted allow_update_branch is outside the management scope", desired: DesiredState{General: General{AllowSquashMerge: new(true)}}, key: setting.AllowUpdateBranch, wantOK: false},
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
			got, ok := tc.desired.value(tc.key)
			if !reflect.DeepEqual(got, tc.want) || ok != tc.wantOK {
				t.Errorf("value(%q) = %v, %v, want %v, %v", tc.key, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

// TestDesiredState_valueCoversEveryDesiredValue は、期待状態が持つ期待値の数と、value が返す期待値の数が食い違わないことを確かめる。
// 期待状態に設定を足して setting.RepositorySettingUnits や value に足し忘れると、宣言を受理した設定が照合されないまま
// コンパイルもほかのテストも通るため、期待状態の全フィールドを埋めて数える。
func TestDesiredState_valueCoversEveryDesiredValue(t *testing.T) {
	var desired DesiredState
	declared := fillDesiredValues(reflect.ValueOf(&desired).Elem())

	n := 0
	for _, key := range setting.RepositorySettingUnits() {
		if _, ok := desired.value(key); ok {
			n++
		}
	}
	if n != declared {
		t.Errorf("value() returned %d desired values for %d declared settings", n, declared)
	}
}

// 関連リソースに関わる管理の単位は名前との組で識別するため、value では引けない。
func TestDesiredState_valueOfRelatedResourceSettingPanics(t *testing.T) {
	for _, key := range []setting.Key{setting.RepositoryVariable, setting.EnvironmentResource, setting.EnvironmentDeploymentBranchPolicy, setting.EnvironmentVariable, setting.RulesetResource} {
		t.Run(string(key), func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("value(%q) did not panic", key)
				}
			}()
			_, _ = DesiredState{}.value(key)
		})
	}
}

func TestDesiredState_valueOfUnknownSettingPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("value() did not panic for an unknown setting")
		}
	}()
	_, _ = DesiredState{}.value("general.unknown")
}

// fillDesiredValues は、v の中の期待値 (ポインタのフィールド) をすべてゼロ値への参照で埋め、その数を返す。
// 列挙値のゼロ値は定義外の値だが、ここでは期待値の有無だけを数える。
// 関連リソースの宣言 (RepositoryVariables、Environments、Rulesets) は value で引かない期待値のため、数えない。
func fillDesiredValues(v reflect.Value) int {
	n := 0
	for structField, field := range v.Fields() {
		if structField.Name == "RepositoryVariables" || structField.Name == "Environments" || structField.Name == "Rulesets" {
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
