package configuration

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/tmknom/reco/internal/setting"
)

// document は、唯一の YAML 文書の根ノードと、ノードの位置から文字を引くためのソースの行。
type document struct {
	root  *yaml.Node
	lines [][]rune
}

// splitLines は、ライブラリが Node.Line と Node.Column を数えるのと同じ規則でソースを行に分ける。
// 改行は CR LF、CR、LF、NEL、LS、PS で、列は文字単位で数え、先頭の BOM は数えない。
func splitLines(data []byte) [][]rune {
	src := []rune(strings.TrimPrefix(string(data), "\uFEFF"))
	var lines [][]rune
	start := 0
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '\r', '\n', '\u0085', '\u2028', '\u2029':
			lines = append(lines, src[start:i])
			if src[i] == '\r' && i+1 < len(src) && src[i+1] == '\n' {
				i++
			}
			start = i + 1
		}
	}
	return append(lines, src[start:])
}

// ライブラリは非特定タグ ! を Node に残さないため、Tag と Style ではなく、ノードの開始位置の文字がタグの ! かで判定する。
// タグのないノードの開始位置は、! で始められないプレーンスカラー、引用符、ブロックスカラーやコレクションの指示子を指す。
// 値なしの null の開始位置は、行末より後ろや、末尾に改行のない入力では最終行の次の行
// (ライブラリはストリームの終端で改行を補う) を指すことがあり、そこにはソースの文字がないためタグなしとする。
// それ以外のソースの外の位置は行や列の数え方の食い違いであり、タグなしとして通さずエラーにする。
// 位置はライブラリが入力から数えた値のため、食い違いを不変条件の破れとして panic にせず、入力のエラーとして返す。
func (d document) hasTag(n *yaml.Node) (bool, error) {
	if n.Line < 1 || n.Line > len(d.lines)+1 || n.Column < 1 {
		return false, errors.New("node position is outside the source")
	}
	if n.Line == len(d.lines)+1 {
		return false, nil
	}
	line := d.lines[n.Line-1]
	column := n.Column - 1
	return column < len(line) && line[column] == '!', nil
}

// checkNode は、どの位置のノードにも共通する YAML の形式規則を確かめる。
func (d document) checkNode(n *yaml.Node, path string) error {
	tagged, err := d.hasTag(n)
	if err != nil {
		return invalid(n, path, "%v", err)
	}
	switch {
	case n.Kind == yaml.AliasNode:
		return invalid(n, path, "aliases are not allowed")
	case n.Anchor != "":
		return invalid(n, path, "anchors are not allowed")
	case tagged:
		return invalid(n, path, "explicit tags are not allowed")
	case n.Tag == "!!null":
		return invalid(n, path, "must not be null or empty")
	default:
		return nil
	}
}

// mappingEntries は、n が required のキーをすべて持ち、required と optional 以外のキーを持たず、
// どのキーも一度だけ持つマッピングであることを確かめ、キーごとの値を返す。
// optional のキーは、宣言されたときだけ返す値に含まれる。
// キーは引用符を外した後の文字列で比べるため、name と "name" は重複になる。
func (d document) mappingEntries(n *yaml.Node, path string, required, optional []string) (map[string]*yaml.Node, error) {
	keys := slices.Concat(required, optional)
	if err := d.checkNode(n, path); err != nil {
		return nil, err
	}
	if n.Kind != yaml.MappingNode {
		return nil, invalid(n, path, "must be a mapping")
	}
	if len(n.Content) == 0 {
		return nil, invalid(n, path, "must not be empty")
	}
	entries := make(map[string]*yaml.Node, len(keys))
	for pair := range slices.Chunk(n.Content, 2) {
		key, value := pair[0], pair[1]
		if err := d.checkNode(key, path); err != nil {
			return nil, err
		}
		switch {
		case key.Tag == "!!merge":
			return nil, invalid(key, path, "merge keys are not allowed")
		case key.Tag != "!!str":
			return nil, invalid(key, path, "keys must be strings")
		case !slices.Contains(keys, key.Value):
			return nil, invalid(key, path, "unknown key %q; expected %s", key.Value, strings.Join(keys, ", "))
		}
		if _, ok := entries[key.Value]; ok {
			return nil, invalid(key, path, "duplicate key %q", key.Value)
		}
		entries[key.Value] = value
	}
	for _, k := range required {
		if _, ok := entries[k]; !ok {
			return nil, invalid(n, path, "missing key %q", k)
		}
	}
	return entries, nil
}

// sequenceContent は、n がシーケンスであることを確かめ、空のシーケンスも受理して要素を返す。
func (d document) sequenceContent(n *yaml.Node, path string) ([]*yaml.Node, error) {
	if err := d.checkNode(n, path); err != nil {
		return nil, err
	}
	if n.Kind != yaml.SequenceNode {
		return nil, invalid(n, path, "must be a sequence")
	}
	return n.Content, nil
}

func (d document) sequenceItems(n *yaml.Node, path string) ([]*yaml.Node, error) {
	items, err := d.sequenceContent(n, path)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, invalid(n, path, "must not be empty")
	}
	return items, nil
}

// patternsAllowedValue は、許可パターンの並びを宣言の順のまま返す。
// 空のシーケンスは「許可パターンなし」の期待値で、省略 (nil) と区別するため、空でも nil にしない。
// yaml.Node.Value はエスケープを解釈した後の文字列のため、重複はそのまま比べる。
func (d document) patternsAllowedValue(n *yaml.Node, path string) ([]string, error) {
	items, err := d.sequenceContent(n, path)
	if err != nil {
		return nil, err
	}
	patterns := make([]string, 0, len(items))
	seen := make(map[string]int, len(items))
	for i, item := range items {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		s, err := d.stringValue(item, itemPath)
		if err != nil {
			return nil, err
		}
		if s == "" || strings.TrimSpace(s) != s {
			return nil, invalid(item, itemPath, "must not be empty or have leading or trailing whitespace; got %q", s)
		}
		if j, ok := seen[s]; ok {
			return nil, invalid(item, itemPath, "duplicate pattern %q; same as %s[%d]", s, path, j)
		}
		seen[s] = i
		patterns = append(patterns, s)
	}
	return patterns, nil
}

// boolValue は、タグを明示しない引用符なし小文字の true / false だけを受理する。
// ライブラリは True や TRUE も !!bool に解決するため、タグに加えて表記を比べる。
// 引用符付きのスカラーは !!str に解決されるため、スタイルは見なくてよい。
func (d document) boolValue(n *yaml.Node, path string) (bool, error) {
	if err := d.checkNode(n, path); err != nil {
		return false, err
	}
	if n.Tag != "!!bool" || (n.Value != "true" && n.Value != "false") {
		return false, invalid(n, path, "must be true or false (unquoted, lowercase); got %q", n.Value)
	}
	return n.Value == "true", nil
}

// permissionLevelValue は、文字列のうち定義された権限の値に完全一致するものだけを受理する。
func (d document) permissionLevelValue(n *yaml.Node, path string) (setting.PermissionLevel, error) {
	s, err := d.stringValue(n, path)
	if err != nil {
		return "", err
	}
	level, ok := setting.ParsePermissionLevel(s)
	if !ok {
		return "", invalid(n, path, "must be read or write; got %q", s)
	}
	return level, nil
}

// allowedActionsPolicyValue は、文字列のうち定義された Actions 許可ポリシーの値に完全一致するものだけを受理する。
func (d document) allowedActionsPolicyValue(n *yaml.Node, path string) (setting.AllowedActionsPolicy, error) {
	s, err := d.stringValue(n, path)
	if err != nil {
		return "", err
	}
	policy, ok := setting.ParseAllowedActionsPolicy(s)
	if !ok {
		return "", invalid(n, path, "must be all, local_only, or selected; got %q", s)
	}
	return policy, nil
}

func (d document) stringValue(n *yaml.Node, path string) (string, error) {
	if err := d.checkNode(n, path); err != nil {
		return "", err
	}
	if n.Tag != "!!str" {
		return "", invalid(n, path, "must be a string")
	}
	return n.Value, nil
}

// invalid は、問題の箇所 (キーのパスと位置) と理由を示す入力検証のエラーを作る。
func invalid(n *yaml.Node, path, format string, args ...any) error {
	return fmt.Errorf("%s (line %d, column %d): %s", path, n.Line, n.Column, fmt.Sprintf(format, args...))
}
