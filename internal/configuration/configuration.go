// Package configuration は、YAML 設定ファイルを解析・検証し、設定宣言に変換する。
//
// YAML の構文要素 (タグ、アンカー) は受理条件の判定に使うため、構造体へ直接デコードせず
// yaml.Node の木と、各ノードの位置にあるソースの文字を走査して検証する。公開する型は YAML を知らない。
package configuration

// Declaration は設定宣言。
type Declaration struct {
	Repositories []RepositoryDeclaration
}

// RepositoryDeclaration はリポジトリごとの宣言。
type RepositoryDeclaration struct {
	Reference RepositoryReference
	General   General
}

// RepositoryReference は OWNER/REPO 形式のリポジトリ指定。
type RepositoryReference struct {
	Owner string
	Repo  string
}

// General はリポジトリ一般設定の期待状態。
// 省略できる設定を持たないため、Load が返す General のフィールドは nil にならない。
type General struct {
	AllowAutoMerge *bool
}
