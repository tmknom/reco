package configuration

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/tmknom/reco/internal/setting"
)

// document は、唯一の YAML 文書の根ノードと、ノードの位置から文字を引くためのソースの行。
type document struct {
	root  *yaml.Node
	lines [][]rune
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

// patternsAllowedValue は、許可パターンの並びを宣言の順のまま返す。
func (d document) patternsAllowedValue(n *yaml.Node, path string) ([]string, error) {
	return d.uniquePatterns(n, path, func(*yaml.Node, string, string) error { return nil })
}

// uniquePatterns は、パターンの並びを宣言の順のまま返す。
// 空のシーケンスは「パターンなし」の期待値で、省略 (nil) と区別するため、空でも nil にしない。
// yaml.Node.Value はエスケープを解釈した後の文字列のため、重複はそのまま比べる。
// 誤りは要素ごとに、文字列であること、空や前後の空白、accept が示す個別の規則、重複の順に示す。
func (d document) uniquePatterns(n *yaml.Node, path string, accept func(item *yaml.Node, itemPath, pattern string) error) ([]string, error) {
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
		if err := accept(item, itemPath, s); err != nil {
			return nil, err
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
		return false, invalidBool(n, path)
	}
	return n.Value == "true", nil
}

// intValue は、タグを明示しない引用符なしの 10 進の整数 (0、または 0 以外の数字で始まる数字の列) だけを受理する。
// ライブラリは 0x5、05、+5、1_000 も !!int に解決するため、タグに加えて表記を比べる。
// 引用符付きのスカラーは !!str に、小数は !!float に解決されるため、スタイルは見なくてよい。
func (d document) intValue(n *yaml.Node, path string) (int, error) {
	if err := d.checkNode(n, path); err != nil {
		return 0, err
	}
	if n.Kind != yaml.ScalarNode {
		return 0, invalid(n, path, "must be an integer")
	}
	v, err := strconv.Atoi(n.Value)
	if n.Tag != "!!int" || !isDecimalInteger(n.Value) || err != nil {
		return 0, invalid(n, path, "must be an integer; got %q", n.Value)
	}
	return v, nil
}

// isDecimalInteger は、s が 0 か、0 以外の数字で始まる数字だけの列かを返す。
func isDecimalInteger(s string) bool {
	if s == "0" {
		return true
	}
	return s != "" && s[0] != '0' && !strings.ContainsFunc(s, func(r rune) bool { return r < '0' || r > '9' })
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

// forkPullRequestApprovalPolicyValue は、文字列のうち定義された承認ポリシーの値に完全一致するものだけを受理する。
func (d document) forkPullRequestApprovalPolicyValue(n *yaml.Node, path string) (setting.ForkPullRequestApprovalPolicy, error) {
	s, err := d.stringValue(n, path)
	if err != nil {
		return "", err
	}
	policy, ok := setting.ParseForkPullRequestApprovalPolicy(s)
	if !ok {
		return "", invalid(n, path, "must be first_time_contributors_new_to_github, first_time_contributors, or all_external_contributors; got %q", s)
	}
	return policy, nil
}

// accessLevelValue は、取り得る値のうち宣言で受け付ける none と user だけを受理する。
// organization は show が表示し得る値のため、そのまま書いた利用者に拒否の理由を添える。
func (d document) accessLevelValue(n *yaml.Node, path string) (setting.AccessLevel, error) {
	s, err := d.stringValue(n, path)
	if err != nil {
		return "", err
	}
	switch level, _ := setting.ParseAccessLevel(s); level {
	case setting.AccessNone, setting.AccessUser:
		return level, nil
	case setting.AccessOrganization:
		return "", invalid(n, path, "must be none or user; got %q; organization is for organization-owned repositories, which reco does not support", s)
	}
	return "", invalid(n, path, "must be none or user; got %q", s)
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

// quotableStringValue は、stringValue と同じく文字列を要求し、文字列でないスカラー (引用符のない数値、真偽値、日付) には、
// 引用符で囲んだ表記を直し方として添える。null は checkNode が先に拒否するため、直し方を添えない。
// 引用符で囲めば受け付ける場所 (変数の値) にだけ使う。名前のように囲んでもほかの規則で拒否され得る場所では、直し方が誤りになる。
func (d document) quotableStringValue(n *yaml.Node, path string) (string, error) {
	if err := d.checkNode(n, path); err != nil {
		return "", err
	}
	switch {
	case n.Tag == "!!str":
		return n.Value, nil
	case n.Kind == yaml.ScalarNode:
		return "", invalid(n, path, "must be a string; got %s; write %q in quotes", n.Value, n.Value)
	default:
		return "", invalid(n, path, "must be a string")
	}
}

// invalidBool は、真偽値を書くつもりだったと読める表記に限り、拒否の理由に直し方を添える。
func invalidBool(n *yaml.Node, path string) error {
	lower := strings.ToLower(n.Value)
	quoted := n.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle) != 0
	switch {
	case quoted && (lower == "true" || lower == "false"):
		return invalid(n, path, "must be true or false; got the quoted string %q; write %s without quotes", n.Value, lower)
	case n.Tag == "!!bool":
		return invalid(n, path, "must be true or false; got %s; write %s in lowercase", n.Value, lower)
	default:
		return invalid(n, path, "must be true or false (unquoted, lowercase); got %q", n.Value)
	}
}

// invalid は、問題の箇所 (キーのパスと位置) と理由を示す入力検証のエラーを作る。
func invalid(n *yaml.Node, path, format string, args ...any) error {
	return fmt.Errorf("%s (line %d, column %d): %s", path, n.Line, n.Column, fmt.Sprintf(format, args...))
}
