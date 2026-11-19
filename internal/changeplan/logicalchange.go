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
	// Key は、この更新操作が変える管理設定。
	Key setting.Key
	// Desired は、この更新操作が送る期待値。
	Desired setting.Value
	// RelevantSettings は、この更新操作の変更前提の基準。変わっていたことを報告するときに優先する順に並べる。
	RelevantSettings []RelevantSetting
}

// RelevantSetting は、変更前提で比べる関係する現在値の設定と、その承認時に予定した値。
type RelevantSetting struct {
	Key        setting.Key
	AtApproval github.SettingObservation[setting.Value]
}

// Keys は、c が変える管理設定を更新操作の順に返す。
func (c LogicalChange) Keys() []setting.Key {
	keys := make([]setting.Key, len(c.Operations))
	for i, operation := range c.Operations {
		keys[i] = operation.Key
	}
	return keys
}

// SettingsToChange は、ドメインモデルの「論理的な設定変更が変える設定」を返す。変更対象外の設定から除く範囲である。
// Keys の管理設定に、その期待値が含む状態の設定を足したもの。
func (c LogicalChange) SettingsToChange() []setting.Key {
	keys := c.Keys()
	if slices.Contains(keys, setting.AllowedActions) {
		keys = append(keys, setting.ActionsEnabled)
	}
	return keys
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
			return nil, fmt.Errorf("%s: %s cannot be applied", r.Target, s.Key)
		}
	}
	_, hasEnabled := r.settingPlanOf(setting.ActionsEnabled)
	if _, hasPolicy := r.settingPlanOf(setting.AllowedActions); hasEnabled && hasPolicy {
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
		}
		operations := []UpdateOperation{operationOf(s)}
		if targetsSelectedPolicy(s) {
			if !paired {
				panic(fmt.Sprintf("changeplan: %s to selected has no not-comparable %s to follow", s.Key, setting.SelectedActions))
			}
			// 2つ目の承認時に予定した値は、1つ目による予定どおりの変化を反映したポリシーである
			operations = append(operations, UpdateOperation{
				Key: criteria.Key, Desired: criteria.Desired,
				RelevantSettings: []RelevantSetting{{Key: s.Key, AtApproval: github.Observed(s.Desired)}},
			})
		}
		changes = append(changes, LogicalChange{Operations: operations})
	}
	return changes, nil
}

// criteriaFollowingPolicy は、遷移先が selected のポリシーの変更予定に続けて更新する、選択方式の Actions 許可条件の比較不能な変更予定を返す。
// 二つが対で計画にあるときだけ真を返す。
func (r RepositoryPlan) criteriaFollowingPolicy() (SettingPlan, bool) {
	policy, hasPolicy := r.settingPlanOf(setting.AllowedActions)
	criteria, hasCriteria := r.settingPlanOf(setting.SelectedActions)
	if !hasPolicy || !hasCriteria || !policy.IsPlannedChange() || !targetsSelectedPolicy(policy) || criteria.Result != NotComparable {
		return SettingPlan{}, false
	}
	return criteria, true
}

// settingPlanOf は、key の管理設定の計画を返す。管理範囲になければ偽を返す。
func (r RepositoryPlan) settingPlanOf(key setting.Key) (SettingPlan, bool) {
	for _, s := range r.Settings {
		if s.Key == key {
			return s, true
		}
	}
	return SettingPlan{}, false
}

// operationOf は、変更予定 s の設定を変える更新操作を返す。
// 基準は、s の計画時の観測結果と、副作用として提示した設定の計画時の観測結果である。
func operationOf(s SettingPlan) UpdateOperation {
	relevant := []RelevantSetting{{Key: s.Key, AtApproval: s.Observed}}
	for _, effect := range s.SideEffects {
		relevant = append(relevant, RelevantSetting{Key: effect.Key, AtApproval: effect.Observed})
	}
	return UpdateOperation{Key: s.Key, Desired: s.Desired, RelevantSettings: relevant}
}

func targetsSelectedPolicy(s SettingPlan) bool {
	return s.Key == setting.AllowedActions && s.Desired.AllowedActionsPolicy() == setting.AllowedActionsSelected
}
