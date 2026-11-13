package setting

// Area は設定の領域の識別。
type Area int

// ゼロ値を定義済みの領域と取り違えないよう 1 から始める。
const (
	AreaGeneral Area = iota + 1
	AreaWorkflowPermissions
)
