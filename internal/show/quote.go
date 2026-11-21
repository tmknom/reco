package show

import "strconv"

// quote は s を一行の二重引用符のスカラーで返す。
// strconv.Quote のエスケープは、UTF-8 として正しい文字列では YAML の二重引用符のスカラーでも同じ文字に読める。
// 観測した文字列は JSON のデコードを経ており、不正な UTF-8 を含まない。
func quote(s string) string {
	return strconv.Quote(s)
}
