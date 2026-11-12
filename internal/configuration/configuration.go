// Package configuration は、YAML 設定ファイルを解析・検証し、設定宣言に変換する。
//
// YAML の構文要素 (タグ、アンカー) は受理条件の判定に使うため、構造体へ直接デコードせず
// yaml.Node の木と、各ノードの位置にあるソースの文字を走査して検証する。公開する型は YAML を知らない。
package configuration

import "github.com/tmknom/reco/internal/repository"

// Declaration は設定宣言。
type Declaration struct {
	Repositories []RepositoryDeclaration
}

// RepositoryDeclaration はリポジトリごとの宣言。
type RepositoryDeclaration struct {
	Reference repository.Reference
	Desired   DesiredState
}

// DesiredState は、一つのリポジトリについて管理設定に求める状態。
type DesiredState struct {
	General General
}

// General はリポジトリ一般設定の期待値。nil は期待値の省略で、その設定は管理範囲に含まれない。
// 省略できる設定を持たないため、Load が返す General のフィールドは nil にならない。
type General struct {
	AllowAutoMerge *bool
}
