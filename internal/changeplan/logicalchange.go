package changeplan

import (
	"fmt"
	"slices"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// LogicalChange は、承認の対象になる論理的な設定変更。
// 実行は承認した変更内容だけを使い、設定宣言を読み直さずに変更前提と期待状態を確かめる。
type LogicalChange struct {
	// Operations は構成する更新操作を実行順に並べたもの。一つ以上を持つ。
	// 期待状態は、すべての更新操作の期待値の全体になる。
	Operations []UpdateOperation
}

// UpdateOperation は、論理的な設定変更を構成する更新操作について、承認の対象になる内容。
type UpdateOperation struct {
	// Unit は、この更新操作が変える管理の単位。
	Unit setting.Unit
	// Desired は、この更新操作が送る期待値。
	// Environment の作成では、同じ更新要求で設定する deployment_branch_policy の期待値で、方式を宣言していなければゼロ値（本文を送らない）。
	Desired setting.Value
	// RelevantSettings は、この更新操作の変更前提の基準。変わっていたことを報告するときに優先する順に並べる。
	RelevantSettings []RelevantSetting
}

// RelevantSetting は、変更前提で比べる関係する現在値の設定と、その承認時に予定した値。
// ValueAtApproval と EnvironmentAtApproval は、Unit.Key の形に応じてどちらか一方だけを持つ。
type RelevantSetting struct {
	Unit setting.Unit
	// ValueAtApproval は、承認時に予定した値。
	ValueAtApproval github.SettingObservation[setting.Value]
	// EnvironmentAtApproval は、承認時に予定した存在と ID。
	// Environment そのものは値を持たないため、存在の観測結果を承認時に予定した値にする。
	EnvironmentAtApproval github.EnvironmentObservation
}

// Units は、c が変える管理の単位を更新操作の順に返す。
// 方式を送る Environment の作成は、一つの更新操作で Environment とその方式を変えるため、存在、方式の順に二つ返す。
func (c LogicalChange) Units() []setting.Unit {
	var units []setting.Unit
	for _, operation := range c.Operations {
		units = append(units, operation.Unit)
		if operation.CreatesEnvironment() && operation.SetsDeploymentBranchPolicy() {
			units = append(units, setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: operation.Unit.Environment})
		}
	}
	return units
}

// TargetsEnvironment は、この更新操作が Environment を宛先とするかを返す。
func (o UpdateOperation) TargetsEnvironment() bool {
	return o.Unit.Key.Area() == setting.AreaEnvironments
}

// CreatesEnvironment は、この更新操作が Environment の作成かを返す。
func (o UpdateOperation) CreatesEnvironment() bool {
	return o.Unit.Key == setting.EnvironmentResource
}

// SetsDeploymentBranchPolicy は、この更新操作が Environment の deployment_branch_policy を送るかを返す。
func (o UpdateOperation) SetsDeploymentBranchPolicy() bool {
	switch o.Unit.Key {
	case setting.EnvironmentDeploymentBranchPolicy:
		return true
	case setting.EnvironmentResource:
		return !o.Desired.IsZero()
	default:
		return false
	}
}

// SettingsToChange は、ドメインモデルの「論理的な設定変更が変える設定」を返す。変更対象外の設定から除く範囲である。
// Units の管理の単位に、その期待値が含む状態の設定を足したもの。
func (c LogicalChange) SettingsToChange() []setting.Unit {
	units := c.Units()
	if slices.Contains(units, setting.Unit{Key: setting.AllowedActions}) {
		units = append(units, setting.Unit{Key: setting.ActionsEnabled})
	}
	return units
}

// LogicalChanges は、変更予定から論理的な設定変更を、計画の並びで導く。
// 適用可能でない変更予定があれば、承認の対象にしないため error にする。
// 副作用を伴う変更予定の更新操作は、operationOf が副作用として提示した設定を必ず基準に載せる。載せずに受け付けると承認の前に示していない値を失わせ得るため。
// 遷移先が selected のポリシーの変更予定と、選択方式の Actions 許可条件の比較不能な変更予定は、ポリシーの位置で一つの論理的な設定変更にまとめ、
// ポリシー、許可条件の順に二つの更新操作で実現する。
// 対にならない形は、設定宣言の組み合わせの規則と観測の取得条件のどちらかが破れたときにしか生じないため、不変条件の破れとして panic する。
// Actions の有効状態と Actions 許可ポリシーの計画が、照合結果によらず同じリポジトリの計画に並ぶ形も、
// 設定宣言の組み合わせの規則が破れたときにしか生じないため panic する。
// 有効状態の変更予定とポリシーの組を実行すると、有効状態の更新操作が無効にする前のポリシーで Actions を有効にし、宣言していない状態を残し得るため。
func (r RepositoryPlan) LogicalChanges() ([]LogicalChange, error) {
	for _, s := range r.Settings {
		if s.IsPlannedChange() && s.Applicability != Applicable {
			return nil, fmt.Errorf("%s: %s cannot be applied", r.Target, s.Unit().String())
		}
	}
	_, hasEnabled := r.settingPlanOf(setting.Unit{Key: setting.ActionsEnabled})
	if _, hasPolicy := r.settingPlanOf(setting.Unit{Key: setting.AllowedActions}); hasEnabled && hasPolicy {
		panic(fmt.Sprintf("changeplan: plans of %s and %s are in the same repository plan", setting.ActionsEnabled, setting.AllowedActions))
	}
	criteria, paired := r.criteriaFollowingPolicy()
	var changes []LogicalChange
	for _, s := range r.Settings {
		switch {
		case !s.IsPlannedChange():
			continue
		case s.Key == setting.SelectedActions && s.Result == NotComparable:
			if !paired {
				panic(fmt.Sprintf("changeplan: %s is not comparable without a policy change to selected", s.Key))
			}
			continue
		case s.Key.Area() == setting.AreaEnvironments:
			if operation, ok := r.environmentOperationOf(s); ok {
				changes = append(changes, LogicalChange{Operations: []UpdateOperation{operation}})
			}
			continue
		}
		operation := operationOf(s)
		if s.Key == setting.SHAPinningRequired {
			operation = shaPinningRequiredOperationOf(s)
		}
		operations := []UpdateOperation{operation}
		if targetsSelectedPolicy(s) {
			if !paired {
				panic(fmt.Sprintf("changeplan: %s to selected has no not-comparable %s to follow", s.Key, setting.SelectedActions))
			}
			// 2つ目の承認時に予定した値は、1つ目による予定どおりの変化を反映したポリシーである
			operations = append(operations, UpdateOperation{
				Unit: criteria.Unit(), Desired: criteria.Desired,
				RelevantSettings: []RelevantSetting{{Unit: s.Unit(), ValueAtApproval: github.Observed(s.Desired)}},
			})
		}
		changes = append(changes, LogicalChange{Operations: operations})
	}
	return changes, nil
}

// criteriaFollowingPolicy は、遷移先が selected のポリシーの変更予定に続けて更新する、選択方式の Actions 許可条件の比較不能な変更予定を返す。
// 二つが対で計画にあるときだけ真を返す。
func (r RepositoryPlan) criteriaFollowingPolicy() (SettingPlan, bool) {
	policy, hasPolicy := r.settingPlanOf(setting.Unit{Key: setting.AllowedActions})
	criteria, hasCriteria := r.settingPlanOf(setting.Unit{Key: setting.SelectedActions})
	if !hasPolicy || !hasCriteria || !policy.IsPlannedChange() || !targetsSelectedPolicy(policy) || criteria.Result != NotComparable {
		return SettingPlan{}, false
	}
	return criteria, true
}

// settingPlanOf は、管理の単位 unit の計画を返す。管理範囲になければ偽を返す。
// Environment の名前は、GitHub が同じ Environment とみなす名前どうしを同じ単位にするため、EqualFold で比べる。
func (r RepositoryPlan) settingPlanOf(unit setting.Unit) (SettingPlan, bool) {
	for _, s := range r.Settings {
		if s.Key == unit.Key && s.Environment.EqualFold(unit.Environment) {
			return s, true
		}
	}
	return SettingPlan{}, false
}

// operationOf は、変更予定 s の設定を変える更新操作を返す。
// 基準は、s の計画時の観測結果と、副作用として提示した設定の計画時の観測結果である。
func operationOf(s SettingPlan) UpdateOperation {
	relevant := []RelevantSetting{{Unit: s.Unit(), ValueAtApproval: s.Observed}}
	for _, effect := range s.SideEffects {
		relevant = append(relevant, RelevantSetting{Unit: setting.Unit{Key: effect.Key}, ValueAtApproval: effect.Observed})
	}
	return UpdateOperation{Unit: s.Unit(), Desired: s.Desired, RelevantSettings: relevant}
}

// shaPinningRequiredOperationOf は、sha_pinning_required の変更予定 s の更新操作を返す。
// 更新要求は必須の enabled に true を送るため、Actions が有効であることを基準に載せ、無効のリポジトリへ送って Actions を有効にしないようにする。
// 計画時に Actions が無効で観測不能だった値は基準に載せない（ドメインモデルの承認と変更前提）。
func shaPinningRequiredOperationOf(s SettingPlan) UpdateOperation {
	var relevant []RelevantSetting
	if s.Result == Mismatch {
		relevant = append(relevant, RelevantSetting{Unit: s.Unit(), ValueAtApproval: s.Observed})
	}
	relevant = append(relevant, RelevantSetting{Unit: setting.Unit{Key: setting.ActionsEnabled}, ValueAtApproval: github.Observed(setting.BoolValue(true))})
	return UpdateOperation{Unit: s.Unit(), Desired: s.Desired, RelevantSettings: relevant}
}

func targetsSelectedPolicy(s SettingPlan) bool {
	return s.Key == setting.AllowedActions && s.Desired.AllowedActionsPolicy() == setting.AllowedActionsSelected
}
