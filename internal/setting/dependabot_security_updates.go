package setting

// DependabotSecurityUpdates は Dependabot security updates の値。
// 一時停止中は有効と同じ設定の値で、表示に一時停止を示すために別の定数で表す。
type DependabotSecurityUpdates int

// ゼロ値を定義済みの値と取り違えないよう 1 から始める。
const (
	DependabotSecurityUpdatesDisabled DependabotSecurityUpdates = iota + 1
	DependabotSecurityUpdatesEnabled
	DependabotSecurityUpdatesPaused
)
