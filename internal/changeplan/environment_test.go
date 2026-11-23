package changeplan

import (
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// TestRepositoryPlan_EnvironmentChangeOfModeMismatchPanicsWithoutMatchedExistence は、存在する Environment の方式の不一致を、
// 同じ名前の存在の計画が一致でなければ論理的な設定変更にせず panic することを確かめる。
// 作成の計画と組むと、存在しない Environment の ID を基準にした方式の変更になるため。
//
//trace:checks INV-004
func TestRepositoryPlan_EnvironmentChangeOfModeMismatchPanicsWithoutMatchedExistence(t *testing.T) {
	staging := mustEnvironmentName(t, "staging")
	creation := SettingPlan{Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: staging}, Observed: github.Observed(setting.Absent()), Desired: setting.Present(), Result: Mismatch, Applicability: Applicable()}
	all := setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyAll})
	protected := setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyProtectedBranches})
	modeChange := SettingPlan{Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: staging}, Observed: github.Observed(all), Desired: protected, Result: Mismatch, Applicability: Applicable()}
	r := RepositoryPlan{Target: mustReference(t, "owner/repo"), RepositoryID: 1, Settings: []SettingPlan{creation, modeChange}}
	defer func() {
		if recover() == nil {
			t.Error("environmentChangeOf() did not panic")
		}
	}()
	_, _ = r.environmentChangeOf(modeChange)
}

// protectedWithPattern は、方式が selected でないのに個別パターンを持つデプロイ元制限の値。値の型はこの形を作れるが、宣言の検証は拒む。
func protectedWithPattern() setting.Value {
	main := setting.DeploymentPattern{Type: setting.DeploymentPatternBranch, Name: "main"}
	return setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyProtectedBranches, Patterns: []setting.DeploymentPattern{main}})
}

// TestRepositoryPlan_CreationOfIgnoresPatternsOfAnotherMode は、方式が selected でない期待値が個別パターンを持っていても、
// その要素を加える更新操作を並べず、方式を送る作成一つにすることを確かめる。宣言していない要素を送らないため。
//
//trace:checks INV-001
func TestRepositoryPlan_CreationOfIgnoresPatternsOfAnotherMode(t *testing.T) {
	staging := mustEnvironmentName(t, "staging")
	creation := SettingPlan{Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: staging}, Observed: github.Observed(setting.Absent()), Desired: setting.Present(), Result: Mismatch, Applicability: Applicable()}
	policyOfAbsent := SettingPlan{
		Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: staging}, Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent),
		Desired: protectedWithPattern(), Result: NotComparable, Applicability: Applicable(),
	}
	got := RepositoryPlan{Settings: []SettingPlan{creation, policyOfAbsent}}.creationOf(creation)
	if len(got.Operations) != 1 || got.Operations[0].Kind != EnvironmentCreationWithMode || got.Operations[0].Mode != setting.DeploymentBranchPolicyProtectedBranches {
		t.Errorf("creationOf() = %+v, want one creation sending protected_branches", got)
	}
}

// TestDeploymentBranchPolicyOperationsIgnoresPatternsOfAnotherMode は、方式が selected でない期待値が個別パターンを持っていても、
// その要素を加える更新操作を並べず、方式の更新一つにすることを確かめる。宣言していない要素を送らないため。
//
//trace:checks INV-001
func TestDeploymentBranchPolicyOperationsIgnoresPatternsOfAnotherMode(t *testing.T) {
	staging := mustEnvironmentName(t, "staging")
	all := setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyAll})
	modeChange := SettingPlan{Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: staging}, Observed: github.Observed(all), Desired: protectedWithPattern(), Result: Mismatch, Applicability: Applicable()}
	got := deploymentBranchPolicyOperations(modeChange, RelevantSetting{Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: staging}})
	if len(got) != 1 || got[0].Kind != DeploymentBranchPolicyModeUpdate || got[0].Mode != setting.DeploymentBranchPolicyProtectedBranches {
		t.Errorf("deploymentBranchPolicyOperations() = %+v, want one mode update sending protected_branches", got)
	}
}
