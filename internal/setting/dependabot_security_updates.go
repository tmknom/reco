package setting

// DependabotSecurityUpdatesState は Dependabot security updates の値の表現。
// 一時停止中は有効と同じ設定の値で、表示に一時停止を示すために別の定数で表す。
// 同じ値かどうかを == で比べると一時停止中と有効が別の値になるため、比べるときは Value.Equal を通す。
type DependabotSecurityUpdatesState int

// ゼロ値を定義済みの値と取り違えないよう 1 から始める。
const (
	DependabotSecurityUpdatesDisabled DependabotSecurityUpdatesState = iota + 1
	DependabotSecurityUpdatesEnabled
	DependabotSecurityUpdatesPaused
)

// DependabotSecurityUpdatesStateOf は、宣言の真偽値を期待値の表現にする。一時停止は期待値にならないため返さない。
func DependabotSecurityUpdatesStateOf(enabled bool) DependabotSecurityUpdatesState {
	if enabled {
		return DependabotSecurityUpdatesEnabled
	}
	return DependabotSecurityUpdatesDisabled
}

// valid は、s が定義済みの値かを返す。
func (s DependabotSecurityUpdatesState) valid() bool {
	return DependabotSecurityUpdatesDisabled <= s && s <= DependabotSecurityUpdatesPaused
}

// Enabled は、設定として有効かを返す。一時停止中も有効である。
func (s DependabotSecurityUpdatesState) Enabled() bool {
	return s == DependabotSecurityUpdatesEnabled || s == DependabotSecurityUpdatesPaused
}
