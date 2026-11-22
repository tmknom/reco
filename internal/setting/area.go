package setting

// Area は設定の領域の識別。
type Area int

// ゼロ値を定義済みの領域と取り違えないよう 1 から始める。
// 定数の並びは YAML 設定仕様の章の並びで、変更計画の表示順はこの並びで決まる。
const (
	AreaGeneral Area = iota + 1
	AreaActionsPermissions
	AreaWorkflowPermissions
	AreaActionsAccess
	AreaRepositoryVariables
	AreaEnvironments
	AreaDependabot
	AreaSecretScanning
	AreaPrivateVulnerabilityReporting
)
