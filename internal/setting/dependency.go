package setting

// Dependency は設定の依存関係で、前提の設定を無効にすると GitHub が依存する設定も無効にする、前提の設定と依存する設定の組。
type Dependency struct {
	Prerequisite, Dependent Key
	// DisabledDependent は依存する設定の無効の値。
	DisabledDependent Value
}

// Dependencies は、設定の依存関係を、YAML 設定仕様の表の並びで返す。
// 呼び出し側が要素を書き換えても共有されないよう、呼び出すたびに新しいスライスを返す。
func Dependencies() []Dependency {
	return []Dependency{
		{Prerequisite: DependabotAlerts, Dependent: DependabotSecurityUpdates, DisabledDependent: DependabotSecurityUpdatesValue(DependabotSecurityUpdatesDisabled)},
		{Prerequisite: SecretScanningEnabled, Dependent: RepositoryPushProtection, DisabledDependent: BoolValue(false)},
	}
}
