package changeplan

import "github.com/tmknom/reco/internal/github"

// RulesetIdentity は、Ruleset そのものの変更前提で、存在と値に加えて確かめる同一性の基準 (ドメインモデルのリポジトリリソースの同一性)。
// ゼロ値は基準がないことを表し、存在しない Ruleset と、Ruleset そのもの以外の管理の単位が当たる。ID 0 の Ruleset とは取り違えない。
// 基準の形を取り違えた値を作れないよう、フィールドを公開せずコンストラクタでだけ作る。
type RulesetIdentity struct {
	observed bool
	id       github.RulesetID
}

// ObservedRulesetIdentity は、計画時に存在を観測した Ruleset の ID を基準にする。
func ObservedRulesetIdentity(id github.RulesetID) RulesetIdentity {
	return RulesetIdentity{observed: true, id: id}
}

// ID は、計画時に観測した ID を基準に持つときだけ、その ID と真を返す。
func (i RulesetIdentity) ID() (github.RulesetID, bool) {
	return i.id, i.observed
}
