package setting

import (
	"slices"
	"testing"
)

func TestRepositorySettingUnits_FollowsConfigurationOrder(t *testing.T) {
	want := []Key{DeleteBranchOnMerge, AllowAutoMerge, AllowSquashMerge, AllowMergeCommit, AllowRebaseMerge, AllowUpdateBranch, SquashMergeCommit, MergeCommit, HasIssues, HasProjects, HasWiki, WebCommitSignoffRequired, ActionsEnabled, AllowedActions, SelectedActions, SHAPinningRequired, DefaultWorkflowPermissions, CanApprovePullRequestReviews, ActionsAccessLevel, ForkPullRequestApproval, PrivateForkPullRequestWorkflows, RetentionDays, OIDCSubject, DependabotAlerts, DependabotSecurityUpdates, SecretScanningEnabled, RepositoryPushProtection, PrivateVulnerabilityReportingEnabled}
	if got := RepositorySettingUnits(); !slices.Equal(got, want) {
		t.Errorf("RepositorySettingUnits() = %v, want %v", got, want)
	}
}

// TestMergeMethods は、マージ方式の列挙が YAML 設定仕様の表の並びで、allow_update_branch を含まないことを確かめる。
// 並びは適用不能の理由に示すマージ方式の並びになる。
func TestMergeMethods(t *testing.T) {
	want := []Key{AllowSquashMerge, AllowMergeCommit, AllowRebaseMerge}
	if got := MergeMethods(); !slices.Equal(got, want) {
		t.Errorf("MergeMethods() = %v, want %v", got, want)
	}
}

// TestKey_CorrespondingMergeMethod は、マージメッセージだけが対応するマージ方式を持ち、squash merge と merge commit を取り違えないことを確かめる。
func TestKey_CorrespondingMergeMethod(t *testing.T) {
	tests := []struct {
		key        Key
		wantMethod Key
		wantOK     bool
	}{
		{key: SquashMergeCommit, wantMethod: AllowSquashMerge, wantOK: true},
		{key: MergeCommit, wantMethod: AllowMergeCommit, wantOK: true},
		{key: AllowSquashMerge},
	}
	for _, tt := range tests {
		t.Run(string(tt.key), func(t *testing.T) {
			method, ok := tt.key.CorrespondingMergeMethod()
			if method != tt.wantMethod || ok != tt.wantOK {
				t.Errorf("CorrespondingMergeMethod() = (%q, %t), want (%q, %t)", method, ok, tt.wantMethod, tt.wantOK)
			}
		})
	}
}

func TestKeyArea(t *testing.T) {
	tests := []struct {
		key  Key
		want Area
	}{
		{key: DeleteBranchOnMerge, want: AreaGeneral},
		{key: AllowAutoMerge, want: AreaGeneral},
		{key: AllowSquashMerge, want: AreaGeneral},
		{key: AllowMergeCommit, want: AreaGeneral},
		{key: AllowRebaseMerge, want: AreaGeneral},
		{key: AllowUpdateBranch, want: AreaGeneral},
		{key: SquashMergeCommit, want: AreaGeneral},
		{key: MergeCommit, want: AreaGeneral},
		{key: HasIssues, want: AreaGeneral},
		{key: HasProjects, want: AreaGeneral},
		{key: HasWiki, want: AreaGeneral},
		{key: WebCommitSignoffRequired, want: AreaGeneral},
		{key: ActionsEnabled, want: AreaActionsPermissions},
		{key: AllowedActions, want: AreaActionsPermissions},
		{key: SHAPinningRequired, want: AreaActionsPermissions},
		{key: SelectedActions, want: AreaActionsPermissions},
		{key: DefaultWorkflowPermissions, want: AreaWorkflowPermissions},
		{key: CanApprovePullRequestReviews, want: AreaWorkflowPermissions},
		{key: ActionsAccessLevel, want: AreaActionsAccess},
		{key: ForkPullRequestApproval, want: AreaForkPullRequestApproval},
		{key: PrivateForkPullRequestWorkflows, want: AreaPrivateForkPullRequestWorkflows},
		{key: RetentionDays, want: AreaRetention},
		{key: OIDCSubject, want: AreaOIDCSubject},
		{key: RepositoryVariable, want: AreaRepositoryVariables},
		{key: EnvironmentResource, want: AreaEnvironments},
		{key: EnvironmentDeploymentBranchPolicy, want: AreaEnvironments},
		{key: EnvironmentReviewerSet, want: AreaEnvironments},
		{key: EnvironmentPreventSelfReview, want: AreaEnvironments},
		{key: EnvironmentWaitTimer, want: AreaEnvironments},
		{key: EnvironmentVariable, want: AreaEnvironmentVariables},
		{key: DependabotAlerts, want: AreaDependabot},
		{key: DependabotSecurityUpdates, want: AreaDependabot},
		{key: SecretScanningEnabled, want: AreaSecretScanning},
		{key: RepositoryPushProtection, want: AreaSecretScanning},
		{key: PrivateVulnerabilityReportingEnabled, want: AreaPrivateVulnerabilityReporting},
		{key: RulesetResource, want: AreaRulesets},
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
		{key: EnvironmentReviewerSet, want: true},
		{key: EnvironmentVariable, want: true},
		{key: RepositoryVariable, want: false},
		{key: AllowAutoMerge, want: false},
		{key: RulesetResource, want: false},
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

// TestKey_DeclaredKeyName は、管理の単位を宣言するキーとして、キーのパスの最も内側の要素を返し、パスが一つの要素だけならそれを返すことを確かめる。
func TestKey_DeclaredKeyName(t *testing.T) {
	tests := []struct {
		key  Key
		want string
	}{
		{key: DependabotAlerts, want: "dependabot_alerts"},
		{key: RulesetResource, want: "rulesets"},
	}
	for _, tt := range tests {
		t.Run(string(tt.key), func(t *testing.T) {
			if got := tt.key.DeclaredKeyName(); got != tt.want {
				t.Errorf("DeclaredKeyName() = %q, want %q", got, tt.want)
			}
		})
	}
}
