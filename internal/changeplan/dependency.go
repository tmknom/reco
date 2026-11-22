package changeplan

import (
	"slices"

	"github.com/tmknom/reco/internal/setting"
)

// settingDependency は、前提の設定を無効にすると GitHub が依存する設定も無効にする組 (Security の設計の適用可能性と副作用)。
// 実行順と、前提の設定を無効にする変更予定の副作用は、組によらず同じ規則で決まる。
// 適用可能性は、前提の設定が無効のときの GitHub の応答が組ごとに違うため、この表に持たせず judgeApplicability が組ごとに判断する。
type settingDependency struct {
	prerequisite, dependent setting.Key
	// disabledDependent は依存する設定の無効の値。Equal で比べるため、一時停止中の Dependabot security updates を有効として扱う。
	disabledDependent setting.Value
	// dependentDisabled は、前提の設定を無効にする更新操作が依存する設定を無効にする副作用の種類。
	dependentDisabled SideEffectKind
}

// settingDependencies は、前提の設定と依存する設定の組を、YAML 設定仕様の表の並びで返す。
func settingDependencies() []settingDependency {
	return []settingDependency{
		{
			prerequisite: setting.DependabotAlerts, dependent: setting.DependabotSecurityUpdates,
			disabledDependent: setting.DependabotSecurityUpdatesValue(setting.DependabotSecurityUpdatesDisabled), dependentDisabled: SideEffectSecurityUpdatesDisabled,
		},
		{
			prerequisite: setting.SecretScanningEnabled, dependent: setting.RepositoryPushProtection,
			disabledDependent: setting.BoolValue(false), dependentDisabled: SideEffectPushProtectionDisabled,
		},
	}
}

// dependencyOnPrerequisite は、key を前提の設定とする組を返す。前提の設定でなければ偽を返す。
func dependencyOnPrerequisite(key setting.Key) (settingDependency, bool) {
	dependencies := settingDependencies()
	if i := slices.IndexFunc(dependencies, func(d settingDependency) bool { return d.prerequisite == key }); i >= 0 {
		return dependencies[i], true
	}
	return settingDependency{}, false
}
