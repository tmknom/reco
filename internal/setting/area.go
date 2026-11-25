package setting

// Area は設定の領域の識別。
type Area int

// ゼロ値を定義済みの領域と取り違えないよう 1 から始める。
// 定数の並びは YAML 設定仕様の章の並びで、同じ章の領域は章の中の並びとする。
// 宣言を受け付けず YAML 設定仕様に章のない領域は、設定ごとの設計の観測が定める読み取りの順に置く。
// 変更計画の表示順は、changeplan がこの並びをもとに決める。
const (
	AreaGeneral Area = iota + 1
	AreaActionsPermissions
	AreaWorkflowPermissions
	AreaActionsAccess
	AreaForkPullRequestApproval
	AreaPrivateForkPullRequestWorkflows
	AreaRetention
	AreaOIDCSubject
	AreaRepositoryVariables
	AreaEnvironments
	AreaEnvironmentVariables
	AreaDependabot
	AreaSecretScanning
	AreaPrivateVulnerabilityReporting
	AreaRulesets
)
