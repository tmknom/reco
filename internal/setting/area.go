package setting

// Area は設定の領域の識別。
// General を指す定数は、指す側がないため置かない。
type Area int

const (
	// AreaWorkflowPermissions はワークフロー権限の領域。ゼロ値を定義済みの領域と取り違えないよう 1 から始める。
	AreaWorkflowPermissions Area = iota + 1
)
