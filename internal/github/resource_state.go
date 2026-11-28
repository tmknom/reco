package github

// resourceState は、名前で引いた一つのリポジトリリソースの観測結果の状態。種類によらず、存在する、存在しない、理由を伴う観測不能のいずれか一つになる。
type resourceState int

// ゼロ値をいずれかの状態と取り違えないよう 1 から始める。
const (
	resourceExisting resourceState = iota + 1
	resourceAbsent
	resourceUnavailable
)
