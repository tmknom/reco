package changeplan

import "github.com/tmknom/reco/internal/github"

// EnvironmentIdentity は、Environment そのものの変更前提で、存在に加えて確かめる同一性の基準 (ドメインモデルの関連リソースの同一性)。
// ゼロ値は基準がないことを表し、存在しない Environment と、Environment そのもの以外の管理の単位が当たる。
// 基準の形を取り違えた値を作れないよう、フィールドを公開せずコンストラクタでだけ作る。
type EnvironmentIdentity struct {
	basis identityBasis
	id    github.EnvironmentID
}

type identityBasis int

const (
	identityNone identityBasis = iota
	identityObserved
	identityCreatedByPrecedingOperation
)

// ObservedEnvironmentIdentity は、計画時に存在を観測した Environment の ID を基準にする。
func ObservedEnvironmentIdentity(id github.EnvironmentID) EnvironmentIdentity {
	return EnvironmentIdentity{basis: identityObserved, id: id}
}

// CreatedByPrecedingOperation は、同じ論理的な設定変更の先行する更新操作が作成する Environment を基準にする。
// 承認の時点で ID が定まらないため、ID を持たない。
func CreatedByPrecedingOperation() EnvironmentIdentity {
	return EnvironmentIdentity{basis: identityCreatedByPrecedingOperation}
}

// ID は、計画時に観測した ID を基準に持つときだけ、その ID と真を返す。
func (i EnvironmentIdentity) ID() (github.EnvironmentID, bool) {
	return i.id, i.basis == identityObserved
}

// IsCreatedByPrecedingOperation は、同じ論理的な設定変更の先行する更新操作が作成する Environment を基準にするかを返す。
func (i EnvironmentIdentity) IsCreatedByPrecedingOperation() bool {
	return i.basis == identityCreatedByPrecedingOperation
}
