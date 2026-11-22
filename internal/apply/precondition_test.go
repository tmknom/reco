package apply

import (
	"reflect"
	"slices"
	"testing"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// TestMatchesAnticipatedValue は、承認時に予定した値と更新直前の設定の観測結果が同じかの規則を確かめる。
//
//trace:checks INV-001 INV-004 FR-010
func TestMatchesAnticipatedValue(t *testing.T) {
	disabled := github.Unavailable[setting.Value](github.UnavailableActionsDisabled)
	notSelected := github.Unavailable[setting.Value](github.UnavailableNotSelected)
	criteria := func(patterns ...string) github.SettingObservation[setting.Value] {
		return github.Observed(setting.SelectedActionsCriteriaValue(setting.SelectedActionsCriteria{PatternsAllowed: patterns}))
	}
	cases := []struct {
		name        string
		anticipated github.SettingObservation[setting.Value]
		current     github.SettingObservation[setting.Value]
		want        bool
	}{
		{name: "unavailable for the same reason is the same", anticipated: disabled, current: disabled, want: true},
		{name: "unavailable for another reason differs", anticipated: disabled, current: notSelected, want: false},
		{name: "observed value where unavailable was approved differs", anticipated: disabled, current: criteria("a/*"), want: false},
		{name: "unavailable where an observed value was approved differs", anticipated: criteria("a/*"), current: disabled, want: false},
		{name: "another observed value differs", anticipated: criteria("a/*"), current: criteria("b/*"), want: false},
		{name: "observed values differing only in the order of patterns are the same", anticipated: criteria("a/*", "b/*"), current: criteria("b/*", "a/*"), want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchesAnticipatedValue(tc.anticipated, tc.current); got != tc.want {
				t.Errorf("matchesAnticipatedValue() = %v, want %v", got, tc.want)
			}
		})
	}
}

//trace:checks INV-001 INV-004 FR-010
func TestCheckPrecondition(t *testing.T) {
	operation := allowAutoMergeChange().Operations[0]
	cases := []struct {
		name        string
		observed    github.ObservationResult
		want        preconditionCheck
		wantChanged setting.Key
	}{
		{
			name:     "same identity and current value hold",
			observed: github.ObservationResult{RepositoryID: 1, General: github.General{AllowAutoMerge: false}},
			want:     preconditionHolds,
		},
		{
			name:     "changed value of another setting alone holds",
			observed: github.ObservationResult{RepositoryID: 1, General: github.General{AllowAutoMerge: false, DeleteBranchOnMerge: true}},
			want:     preconditionHolds,
		},
		{
			name:     "different ID alone is an identity mismatch",
			observed: github.ObservationResult{RepositoryID: 2, General: github.General{AllowAutoMerge: false}},
			want:     preconditionIdentityMismatch,
		},
		{
			name:        "changed current value alone is a current value change of the setting even if it equals the desired value",
			observed:    github.ObservationResult{RepositoryID: 1, General: github.General{AllowAutoMerge: true}},
			want:        preconditionCurrentValueChanged,
			wantChanged: setting.AllowAutoMerge,
		},
		{
			name:     "different ID and current value is an identity mismatch",
			observed: github.ObservationResult{RepositoryID: 2, General: github.General{AllowAutoMerge: true}},
			want:     preconditionIdentityMismatch,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got, changed := checkPrecondition(1, operation, tc.observed, github.EnvironmentObservation{}); got != tc.want || changed != (setting.Unit{Key: tc.wantChanged}) {
				t.Errorf("checkPrecondition() = %v, %q, want %v, %q", got, changed.Key, tc.want, tc.wantChanged)
			}
		})
	}

	// 関係する現在値の設定が複数並ぶ変更と、承認時に予定した値が観測不能の変更。値の比べ方は TestMatchesAnticipatedValue が確かめる
	actionsCases := []struct {
		name   string
		change changeplan.LogicalChange
		// index は、確かめる更新操作の change.Operations の添字。
		index       int
		observed    *github.ActionsPermissions
		want        preconditionCheck
		wantChanged setting.Key
	}{
		{
			// 2つ目がすべて上書きする3項目は基準に並ばないため、GitHub が戻した値でも成立する
			name:     "second update to selected holds with the policy selected and the criteria GitHub set back",
			change:   toSelectedChange(),
			index:    1,
			observed: actionsSelected(setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, PatternsAllowed: []string{}}),
			want:     preconditionHolds,
		},
		{
			name:        "second update to selected with the policy not selected is a current value change of allowed_actions",
			change:      toSelectedChange(),
			index:       1,
			observed:    actionsEnabled(setting.AllowedActionsLocalOnly, false),
			want:        preconditionCurrentValueChanged,
			wantChanged: setting.AllowedActions,
		},
		{
			name:        "selected to all with criteria other than shown before approval is a current value change of selected_actions",
			change:      selectedToAllChange(),
			observed:    actionsSelected(setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, PatternsAllowed: []string{"a/*", "b/*", "c/*"}}),
			want:        preconditionCurrentValueChanged,
			wantChanged: setting.SelectedActions,
		},
		{
			name:        "selected to all with the policy no longer selected is a current value change of allowed_actions",
			change:      selectedToAllChange(),
			observed:    actionsEnabled(setting.AllowedActionsLocalOnly, false),
			want:        preconditionCurrentValueChanged,
			wantChanged: setting.AllowedActions,
		},
		{
			name:     "enabling actions holds while actions is still disabled",
			change:   enableActionsChange(),
			observed: actionsDisabled(),
			want:     preconditionHolds,
		},
	}
	for _, tc := range actionsCases {
		t.Run(tc.name, func(t *testing.T) {
			observed := github.ObservationResult{RepositoryID: 1, ActionsPermissions: tc.observed}
			if got, changed := checkPrecondition(1, tc.change.Operations[tc.index], observed, github.EnvironmentObservation{}); got != tc.want || changed != (setting.Unit{Key: tc.wantChanged}) {
				t.Errorf("checkPrecondition() = %v, %q, want %v, %q", got, changed.Key, tc.want, tc.wantChanged)
			}
		})
	}

	t.Run("update operation without relevant settings panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("checkPrecondition() did not panic")
			}
		}()
		empty := changeplan.UpdateOperation{Kind: changeplan.SettingUpdate, Unit: setting.Unit{Key: setting.AllowAutoMerge}, Value: setting.BoolValue(true)}
		checkPrecondition(1, empty, github.ObservationResult{RepositoryID: 1}, github.EnvironmentObservation{})
	})
}

// TestPrecedingUpdateNotReflected は、後続の更新操作の直前の観測が、先行する更新操作の直前の状態のままかの規則を確かめる。
// 真のときだけ反映の遅れの可能性として観測し直すため、偽の側に倒れる形を確かめる。
//
//trace:checks INV-004 INV-005
func TestPrecedingUpdateNotReflected(t *testing.T) {
	cases := []struct {
		name     string
		change   changeplan.LogicalChange
		index    int
		observed *github.ActionsPermissions
		want     bool
	}{
		{
			name:     "first update has no preceding update even if the observation is the value before it",
			change:   toSelectedChange(),
			index:    0,
			observed: actionsEnabled(setting.AllowedActionsLocalOnly, false),
			want:     false,
		},
		{
			name:     "policy still local_only as before the first update is not reflected",
			change:   toSelectedChange(),
			index:    1,
			observed: actionsEnabled(setting.AllowedActionsLocalOnly, false),
			want:     true,
		},
		{
			name:     "actions still disabled as before the first update is not reflected",
			change:   enableSelectedChange(),
			index:    1,
			observed: actionsDisabled(),
			want:     true,
		},
		{
			name:     "policy selected is reflected",
			change:   toSelectedChange(),
			index:    1,
			observed: actionsSelected(setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, PatternsAllowed: []string{}}),
			want:     false,
		},
		{
			name:     "policy neither the value before the first update nor selected is not a delay",
			change:   toSelectedChange(),
			index:    1,
			observed: actionsEnabled(setting.AllowedActionsAll, false),
			want:     false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			observed := github.ObservationResult{RepositoryID: 1, ActionsPermissions: tc.observed}
			if got := precedingUpdateNotReflected(1, tc.change.Operations[:tc.index], observed, github.EnvironmentObservation{}); got != tc.want {
				t.Errorf("precedingUpdateNotReflected() = %v, want %v", got, tc.want)
			}
		})
	}

	main, feature := branchPattern("main"), branchPattern("feature/*")
	environmentCases := []struct {
		name         string
		change       changeplan.LogicalChange
		index        int
		environments *github.Environments
		want         bool
	}{
		{
			name:         "pattern addition with the set still as before the preceding removal is not reflected",
			change:       deploymentPatternsChange(t, "staging", selectedEnvironment(7, "Staging", []setting.DeploymentPattern{main, feature}, 10, 11), selectedPolicy(main, branchPattern("release/*"))),
			index:        1,
			environments: environments(selectedEnvironment(7, "Staging", []setting.DeploymentPattern{main, feature}, 10, 11)),
			want:         true,
		},
		{
			name:         "pattern addition after a creation with the environment still absent is not reflected",
			change:       creationWithPatternsChange(t, "production", selectedPolicy(main)),
			index:        1,
			environments: environments(),
			want:         true,
		},
	}
	for _, tc := range environmentCases {
		t.Run(tc.name, func(t *testing.T) {
			observed := github.ObservationResult{RepositoryID: 1, Environments: tc.environments}
			if got := precedingUpdateNotReflected(1, tc.change.Operations[:tc.index], observed, github.EnvironmentObservation{}); got != tc.want {
				t.Errorf("precedingUpdateNotReflected() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestMatchesAnticipatedEnvironment は、承認時に予定した Environment の存在と ID が、更新直前の観測と同じかの規則を確かめる。
//
//trace:checks INV-001 INV-004 FR-010
func TestMatchesAnticipatedEnvironment(t *testing.T) {
	name := mustEnvironmentName(t, "production")
	lookup := func(existing ...github.Environment) github.EnvironmentObservation {
		return github.Environments{Existing: existing}.Lookup(name)
	}
	absent := lookup(observedEnvironment(9, "staging", setting.DeploymentBranchPolicyAll))
	undetermined := lookup(observedEnvironment(9, "prodüction", setting.DeploymentBranchPolicyAll))
	approved := lookup(observedEnvironment(7, "production", setting.DeploymentBranchPolicyAll))
	cases := []struct {
		name        string
		anticipated github.EnvironmentObservation
		current     github.EnvironmentObservation
		want        bool
	}{
		{name: "absent at approval and still absent is the same", anticipated: absent, current: lookup(), want: true},
		{name: "absent at approval and existing now differs", anticipated: absent, current: approved, want: false},
		{name: "absent at approval and existing under a name differing only in case differs", anticipated: absent, current: lookup(observedEnvironment(8, "Production", setting.DeploymentBranchPolicyAll)), want: false},
		{name: "absent at approval and undetermined now differs", anticipated: absent, current: undetermined, want: false},
		{
			name: "existing with the same ID is the same even if the spelling and the mode differ", anticipated: approved,
			current: lookup(observedEnvironment(7, "PRODUCTION", setting.DeploymentBranchPolicyProtectedBranches)), want: true,
		},
		{name: "existing with another ID differs", anticipated: approved, current: lookup(observedEnvironment(8, "production", setting.DeploymentBranchPolicyAll)), want: false},
		{name: "existing at approval and absent now differs", anticipated: approved, current: absent, want: false},
		{name: "existing at approval and undetermined now differs", anticipated: approved, current: undetermined, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchesAnticipatedEnvironment(tc.anticipated, tc.current); got != tc.want {
				t.Errorf("matchesAnticipatedEnvironment() = %v, want %v", got, tc.want)
			}
		})
	}

	// 承認の前に適用不能として止めるため、承認時に予定した値にならない観測結果
	panicCases := []struct {
		name        string
		anticipated github.EnvironmentObservation
	}{
		{name: "undetermined at approval panics", anticipated: undetermined},
		{name: "zero value at approval panics", anticipated: github.EnvironmentObservation{}},
	}
	for _, tc := range panicCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("matchesAnticipatedEnvironment() did not panic")
				}
			}()
			matchesAnticipatedEnvironment(tc.anticipated, absent)
		})
	}
}

// TestCheckPrecondition_Environments は、Environment の作成と方式の変更の更新直前の判定表を確かめる。
// 存在の比べ方は TestMatchesAnticipatedEnvironment、方式の値の比べ方は TestMatchesAnticipatedValue が確かめる。
//
//trace:checks INV-001 INV-004 FR-010
func TestCheckPrecondition_Environments(t *testing.T) {
	production := mustEnvironmentName(t, "production")
	staging := mustEnvironmentName(t, "staging")
	existence := func(name setting.EnvironmentName) setting.Unit {
		return setting.Unit{Key: setting.EnvironmentResource, Environment: name}
	}
	stagingPolicy := setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: staging}
	approvedStaging := observedEnvironment(7, "Staging", setting.DeploymentBranchPolicyAll)
	selected := func(id github.EnvironmentID, patterns ...string) github.Environment {
		e := observedEnvironment(id, "Staging", setting.DeploymentBranchPolicySelected)
		for _, p := range patterns {
			e.DeploymentBranchPolicy.Patterns = append(e.DeploymentBranchPolicy.Patterns, setting.DeploymentPattern{Type: setting.DeploymentPatternBranch, Name: p})
		}
		return e
	}
	waitTimer := 30
	protectedStaging := approvedStaging
	protectedStaging.Protection = setting.EnvironmentProtection{WaitTimer: &waitTimer, CanAdminsBypass: true}
	main, feature := branchPattern("main"), branchPattern("feature/*")
	// 外す DELETE feature/*（基準 [main, feature/*]）、足す POST release/*（基準 [main]）の順
	replacement := deploymentPatternsChange(t, "staging", selectedEnvironment(7, "Staging", []setting.DeploymentPattern{main, feature}, 10, 11), selectedPolicy(main, branchPattern("release/*")))
	// 作成に続く POST の Environment の基準に、計画時の「存在しない」の観測結果も入れた、changeplan が作らない並び
	creation := creationWithPatternsChange(t, "production", selectedPolicy(main))
	creation.Operations[1].RelevantSettings = slices.Clone(creation.Operations[1].RelevantSettings)
	creation.Operations[1].RelevantSettings[0].AnticipatedEnvironment = github.Environments{}.Lookup(production)

	cases := []struct {
		name   string
		change changeplan.LogicalChange
		// index は、確かめる更新操作の change.Operations の添字。
		index       int
		observed    github.ObservationResult
		want        preconditionCheck
		wantChanged setting.Unit
	}{
		{
			name:     "creation holds when no environment of the same name exists",
			change:   creationChange(t, "production"),
			observed: github.ObservationResult{RepositoryID: 1, Environments: environments(approvedStaging)},
			want:     preconditionHolds,
		},
		{
			name:        "creation with the environment created since approval is a change of the environment",
			change:      creationWithModeChange(t, "production", setting.DeploymentBranchPolicyProtectedBranches),
			observed:    github.ObservationResult{RepositoryID: 1, Environments: environments(observedEnvironment(8, "production", setting.DeploymentBranchPolicyProtectedBranches))},
			want:        preconditionCurrentValueChanged,
			wantChanged: existence(production),
		},
		{
			name:        "creation with an environment of a name differing only in case is a change of the environment",
			change:      creationChange(t, "production"),
			observed:    github.ObservationResult{RepositoryID: 1, Environments: environments(observedEnvironment(8, "Production", setting.DeploymentBranchPolicyAll))},
			want:        preconditionCurrentValueChanged,
			wantChanged: existence(production),
		},
		{
			name:        "creation with an undetermined name is a change of the environment",
			change:      creationChange(t, "production"),
			observed:    github.ObservationResult{RepositoryID: 1, Environments: environments(observedEnvironment(8, "prodüction", setting.DeploymentBranchPolicyAll))},
			want:        preconditionCurrentValueChanged,
			wantChanged: existence(production),
		},
		{
			name:     "creation in another repository is an identity mismatch",
			change:   creationChange(t, "production"),
			observed: github.ObservationResult{RepositoryID: 2, Environments: environments()},
			want:     preconditionIdentityMismatch,
		},
		{
			name:     "mode change holds with the same ID and mode",
			change:   modeChange(t, "staging", approvedStaging, setting.DeploymentBranchPolicyProtectedBranches),
			observed: github.ObservationResult{RepositoryID: 1, Environments: environments(approvedStaging)},
			want:     preconditionHolds,
		},
		{
			name:   "mode change holds when only the protection changed and an undetermined name appeared",
			change: modeChange(t, "staging", approvedStaging, setting.DeploymentBranchPolicyProtectedBranches),
			observed: github.ObservationResult{RepositoryID: 1, Environments: environments(
				observedEnvironment(8, "prodüction", setting.DeploymentBranchPolicyAll), protectedStaging,
			)},
			want: preconditionHolds,
		},
		{
			name:     "mode change holds when the patterns differ only in order",
			change:   modeChange(t, "staging", selected(7, "main", "release"), setting.DeploymentBranchPolicyProtectedBranches),
			observed: github.ObservationResult{RepositoryID: 1, Environments: environments(selected(7, "release", "main"))},
			want:     preconditionHolds,
		},
		{
			name:        "mode change with another set of patterns is a change of the mode",
			change:      modeChange(t, "staging", selected(7, "main"), setting.DeploymentBranchPolicyProtectedBranches),
			observed:    github.ObservationResult{RepositoryID: 1, Environments: environments(selected(7, "main", "release"))},
			want:        preconditionCurrentValueChanged,
			wantChanged: stagingPolicy,
		},
		{
			name:        "mode change with another mode is a change of the mode",
			change:      modeChange(t, "staging", approvedStaging, setting.DeploymentBranchPolicyProtectedBranches),
			observed:    github.ObservationResult{RepositoryID: 1, Environments: environments(observedEnvironment(7, "Staging", setting.DeploymentBranchPolicyProtectedBranches))},
			want:        preconditionCurrentValueChanged,
			wantChanged: stagingPolicy,
		},
		{
			name:        "mode change of an environment recreated with another ID is a change of the environment",
			change:      modeChange(t, "staging", approvedStaging, setting.DeploymentBranchPolicyProtectedBranches),
			observed:    github.ObservationResult{RepositoryID: 1, Environments: environments(observedEnvironment(8, "Staging", setting.DeploymentBranchPolicyAll))},
			want:        preconditionCurrentValueChanged,
			wantChanged: existence(staging),
		},
		{
			name:        "mode change of an environment recreated with another mode is reported as a change of the environment first",
			change:      modeChange(t, "staging", approvedStaging, setting.DeploymentBranchPolicyProtectedBranches),
			observed:    github.ObservationResult{RepositoryID: 1, Environments: environments(observedEnvironment(8, "Staging", setting.DeploymentBranchPolicyProtectedBranches))},
			want:        preconditionCurrentValueChanged,
			wantChanged: existence(staging),
		},
		{
			name:        "mode change of an environment that no longer exists is a change of the environment",
			change:      modeChange(t, "staging", approvedStaging, setting.DeploymentBranchPolicyProtectedBranches),
			observed:    github.ObservationResult{RepositoryID: 1, Environments: environments()},
			want:        preconditionCurrentValueChanged,
			wantChanged: existence(staging),
		},
		{
			name:     "pattern removal holds when only the IDs of the patterns differ",
			change:   replacement,
			observed: github.ObservationResult{RepositoryID: 1, Environments: environments(selectedEnvironment(7, "Staging", []setting.DeploymentPattern{main, feature}, 20, 21))},
			want:     preconditionHolds,
		},
		{
			name:        "pattern addition with a pattern outside the expected set is a change of the policy",
			change:      replacement,
			index:       1,
			observed:    github.ObservationResult{RepositoryID: 1, Environments: environments(selectedEnvironment(7, "Staging", []setting.DeploymentPattern{main, branchPattern("other/*")}, 10, 12))},
			want:        preconditionCurrentValueChanged,
			wantChanged: stagingPolicy,
		},
		{
			name:        "pattern addition to an environment recreated with another ID is a change of the environment even if the set is as expected",
			change:      replacement,
			index:       1,
			observed:    github.ObservationResult{RepositoryID: 1, Environments: environments(selectedEnvironment(8, "Staging", []setting.DeploymentPattern{main}, 10))},
			want:        preconditionCurrentValueChanged,
			wantChanged: existence(staging),
		},
		{
			name:     "pattern addition after a creation reads only that the preceding update created the environment",
			change:   creation,
			index:    1,
			observed: github.ObservationResult{RepositoryID: 1, Environments: environments(selectedEnvironment(8, "production", nil))},
			want:     preconditionHolds,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, changed := checkPrecondition(1, tc.change.Operations[tc.index], tc.observed, github.EnvironmentObservation{})
			if got != tc.want || changed != tc.wantChanged {
				t.Errorf("checkPrecondition() = %v, %+v, want %v, %+v", got, changed, tc.want, tc.wantChanged)
			}
		})
	}
}

// TestCheckPrecondition_RepositoryVariables は、変数の作成と値の変更の変更前提を、一覧から引いた存在と値で確かめることを確かめる。
//
//trace:checks INV-001 INV-004 FR-010
func TestCheckPrecondition_RepositoryVariables(t *testing.T) {
	goVersion := setting.Unit{Key: setting.RepositoryVariable, Variable: mustVariableName(t, "GO_VERSION")}
	cases := []struct {
		name        string
		change      changeplan.LogicalChange
		variables   *github.Variables
		want        preconditionCheck
		wantChanged setting.Unit
	}{
		{
			name:      "creation holds when only another variable exists",
			change:    variableCreationChange(t, "GO_VERSION", "1.27"),
			variables: variables(github.Variable{Name: "OTHER", Value: "x"}),
			want:      preconditionHolds,
		},
		{
			name:        "creation with the variable created since approval with the declared value is a change of the variable",
			change:      variableCreationChange(t, "GO_VERSION", "1.27"),
			variables:   variables(github.Variable{Name: "GO_VERSION", Value: "1.27"}),
			want:        preconditionCurrentValueChanged,
			wantChanged: goVersion,
		},
		{
			name:      "value change holds when the value is the planned value",
			change:    variableValueChange(t, "GO_VERSION", "1.26", "1.27"),
			variables: variables(github.Variable{Name: "GO_VERSION", Value: "1.26"}),
			want:      preconditionHolds,
		},
		{
			name:        "value change with the declared value since approval is a change of the variable",
			change:      variableValueChange(t, "GO_VERSION", "1.26", "1.27"),
			variables:   variables(github.Variable{Name: "GO_VERSION", Value: "1.27"}),
			want:        preconditionCurrentValueChanged,
			wantChanged: goVersion,
		},
		{
			name:        "value change with the variable deleted since approval is a change of the variable",
			change:      variableValueChange(t, "GO_VERSION", "1.26", "1.27"),
			variables:   variables(),
			want:        preconditionCurrentValueChanged,
			wantChanged: goVersion,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			observed := github.ObservationResult{RepositoryID: 1, RepositoryVariables: tc.variables}
			got, changed := checkPrecondition(1, tc.change.Operations[0], observed, github.EnvironmentObservation{})
			if got != tc.want || changed != tc.wantChanged {
				t.Errorf("checkPrecondition() = %v, %+v, want %v, %+v", got, changed, tc.want, tc.wantChanged)
			}
		})
	}
}

// TestCheckPrecondition_EnvironmentVariables は、Environment Variable の作成と値の変更の変更前提を、
// 計画時に存在した Environment では Environment の存在と ID、変数の順に、同じ変更計画が作成した Environment では変数が存在しないことだけで確かめることを確かめる。
//
//trace:checks INV-001 INV-004 FR-010
func TestCheckPrecondition_EnvironmentVariables(t *testing.T) {
	production := mustEnvironmentName(t, "production")
	existence := setting.Unit{Key: setting.EnvironmentResource, Environment: production}
	region := setting.Unit{Key: setting.EnvironmentVariable, Environment: production, Variable: mustVariableName(t, "REGION")}
	planned := github.Variable{Name: "REGION", Value: "ap-northeast-1"}
	approved := withVariables(observedEnvironment(7, "Production", setting.DeploymentBranchPolicyAll), planned)
	valueChange := environmentVariableValueChange(t, "production", approved, "REGION", "ap-northeast-1", "ap-northeast-3")
	creation := createdEnvironmentVariableChange(t, "production", "REGION", "ap-northeast-1")
	cases := []struct {
		name        string
		change      changeplan.LogicalChange
		existing    []github.Environment
		want        preconditionCheck
		wantChanged setting.Unit
	}{
		{
			name:     "value change holds when the environment and the value are as planned",
			change:   valueChange,
			existing: []github.Environment{approved},
			want:     preconditionHolds,
		},
		{
			name:        "value change of an environment that no longer exists is a change of the environment",
			change:      valueChange,
			existing:    nil,
			want:        preconditionCurrentValueChanged,
			wantChanged: existence,
		},
		{
			// 作り直された Environment の変数は、計画時と同じ値でも承認した変更の相手ではない
			name:        "value change of an environment recreated with another ID is a change of the environment even if the value is as planned",
			change:      valueChange,
			existing:    []github.Environment{withVariables(observedEnvironment(8, "Production", setting.DeploymentBranchPolicyAll), planned)},
			want:        preconditionCurrentValueChanged,
			wantChanged: existence,
		},
		{
			name:        "value change with another value since approval is a change of the variable",
			change:      valueChange,
			existing:    []github.Environment{withVariables(observedEnvironment(7, "Production", setting.DeploymentBranchPolicyAll), github.Variable{Name: "REGION", Value: "us-east-1"})},
			want:        preconditionCurrentValueChanged,
			wantChanged: region,
		},
		{
			name:     "creation in a created environment holds when the environment exists without the variable",
			change:   creation,
			existing: []github.Environment{withVariables(observedEnvironment(8, "production", setting.DeploymentBranchPolicyAll), github.Variable{Name: "OTHER", Value: "x"})},
			want:     preconditionHolds,
		},
		{
			// 変数は観測不能になり、存在しないことと同じには扱わない
			name:        "creation in a created environment that does not exist is a change of the variable",
			change:      creation,
			existing:    nil,
			want:        preconditionCurrentValueChanged,
			wantChanged: region,
		},
		{
			name:        "creation in a created environment with the variable created with the declared value is a change of the variable",
			change:      creation,
			existing:    []github.Environment{withVariables(observedEnvironment(8, "production", setting.DeploymentBranchPolicyAll), planned)},
			want:        preconditionCurrentValueChanged,
			wantChanged: region,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			observed := github.ObservationResult{RepositoryID: 1, Environments: environments(tc.existing...)}
			got, changed := checkPrecondition(1, tc.change.Operations[0], observed, github.EnvironmentObservation{})
			if got != tc.want || changed != tc.wantChanged {
				t.Errorf("checkPrecondition() = %v, %+v, want %v, %+v", got, changed, tc.want, tc.wantChanged)
			}
		})
	}
}

// TestMatchesCreatedEnvironment は、同じ論理的な設定変更が作成した Environment について、後続の更新操作の直前の観測が予定どおりかの規則を確かめる。
//
//trace:checks INV-004 FR-010
func TestMatchesCreatedEnvironment(t *testing.T) {
	name := mustEnvironmentName(t, "production")
	lookup := func(existing ...github.Environment) github.EnvironmentObservation {
		return github.Environments{Existing: existing}.Lookup(name)
	}
	created := lookup(observedEnvironment(8, "production", setting.DeploymentBranchPolicySelected))
	cases := []struct {
		name    string
		created github.EnvironmentObservation
		current github.EnvironmentObservation
		want    bool
	}{
		{name: "existing without a base ID yet is as expected", created: github.EnvironmentObservation{}, current: created, want: true},
		{name: "absent is not as expected", created: github.EnvironmentObservation{}, current: lookup(), want: false},
		{name: "undetermined is not as expected", created: github.EnvironmentObservation{}, current: lookup(observedEnvironment(9, "prodüction", setting.DeploymentBranchPolicySelected)), want: false},
		{name: "existing with an ID other than the base is not as expected", created: created, current: lookup(observedEnvironment(9, "production", setting.DeploymentBranchPolicySelected)), want: false},
		{name: "existing with the base ID is as expected", created: created, current: lookup(observedEnvironment(8, "Production", setting.DeploymentBranchPolicyAll)), want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchesCreatedEnvironment(tc.created, tc.current); got != tc.want {
				t.Errorf("matchesCreatedEnvironment() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestEnvironmentCreatedIn は、同じ論理的な設定変更が作成した Environment の ID の基準を、作成に続く最初の更新操作の直前の観測から引く規則を確かめる。
//
//trace:checks INV-004 FR-010
func TestEnvironmentCreatedIn(t *testing.T) {
	creation := creationWithPatternsChange(t, "production", selectedPolicy(branchPattern("main"), tagPattern("v*")))
	observedWith := func(id github.EnvironmentID) github.ObservationResult {
		return github.ObservationResult{RepositoryID: 1, Environments: environments(selectedEnvironment(id, "Production", nil))}
	}
	t.Run("creation with one read before the update has no base yet", func(t *testing.T) {
		// 作成の直前の観測を引けば、そこにある Environment が基準になってしまうことを、存在する Environment を置いて確かめる
		before := []github.ObservationResult{observedWith(7)}
		if got := environmentCreatedIn(creation, before); !reflect.DeepEqual(got, github.EnvironmentObservation{}) {
			t.Errorf("environmentCreatedIn() = %+v, want the zero value", got)
		}
	})
	t.Run("creation with later reads is the environment read before the first update after the creation", func(t *testing.T) {
		before := []github.ObservationResult{{RepositoryID: 1, Environments: environments()}, observedWith(8), observedWith(9)}
		got, ok := environmentCreatedIn(creation, before).Existing()
		if !ok || got.ID != 8 {
			t.Errorf("environmentCreatedIn() = %+v, %v, want the environment with ID 8", got, ok)
		}
	})
	t.Run("change without a creation has no base", func(t *testing.T) {
		change := deploymentPatternsChange(t, "production", observedEnvironment(8, "production", setting.DeploymentBranchPolicyAll), selectedPolicy(branchPattern("main")))
		before := []github.ObservationResult{observedWith(8), observedWith(8)}
		if got := environmentCreatedIn(change, before); !reflect.DeepEqual(got, github.EnvironmentObservation{}) {
			t.Errorf("environmentCreatedIn() = %+v, want the zero value", got)
		}
	})
	t.Run("creation with the environment not existing before the first update after it panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("environmentCreatedIn() did not panic")
			}
		}()
		environmentCreatedIn(creation, []github.ObservationResult{{RepositoryID: 1, Environments: environments()}, {RepositoryID: 1, Environments: environments()}})
	})
	t.Run("change without a creation expecting a created environment panics", func(t *testing.T) {
		change := deploymentPatternsChange(t, "production", observedEnvironment(8, "production", setting.DeploymentBranchPolicyAll), selectedPolicy(branchPattern("main")))
		change.Operations[1].RelevantSettings[0] = changeplan.RelevantSetting{Unit: change.Operations[1].RelevantSettings[0].Unit, CreatedByPrecedingOperation: true}
		defer func() {
			if recover() == nil {
				t.Error("environmentCreatedIn() did not panic")
			}
		}()
		environmentCreatedIn(change, []github.ObservationResult{observedWith(8)})
	})
}
