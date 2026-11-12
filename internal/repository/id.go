package repository

// ID は GitHub がリポジトリの実体に付与する識別子。
// 名前変更や移転で変わらないため、リポジトリ指定が承認したリポジトリと同じ実体を指すかの確認に使う。
type ID int64
