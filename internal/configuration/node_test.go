package configuration

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// nodeAt は、root から Content の添字を順にたどったノードを返す。添字がなければ root を返す。
func nodeAt(root *yaml.Node, indexes []int) *yaml.Node {
	n := root
	for _, i := range indexes {
		n = n.Content[i]
	}
	return n
}

//trace:checks FR-001
func TestDocumentHasTag(t *testing.T) {
	cases := []struct {
		name string
		src  string
		node []int
		want bool
	}{
		{name: "non-specific tag is detected", src: "! x\n", want: true},
		{name: "standard tag is detected", src: "!!str x\n", want: true},
		{name: "custom tag is detected", src: "!custom x\n", want: true},
		{name: "plain scalar has no tag", src: "x\n", want: false},
		{name: "quoted scalar starting with ! has no tag", src: "\"!x\"\n", want: false},
		{name: "tag inside flow mapping is detected", src: "{a: ! x}\n", node: []int{1}, want: true},
		{
			// 値なしの null は行末より後ろを指す
			name: "missing value at line end has no tag",
			src:  "a:\n",
			node: []int{1},
			want: false,
		},
		{
			// ライブラリはストリームの終端で改行を補い、根の null が最終行の次の行を指す
			name: "missing value after last line has no tag",
			src:  "---",
			want: false,
		},
		// 位置はライブラリの行と列の数え方とソースの行の分け方が一致するときだけ正しいため、改行の種類ごとに確かめる
		{name: "tag after LF is detected", src: "a: 1\nb: ! x\n", node: []int{3}, want: true},
		{name: "tag after CRLF is detected", src: "a: 1\r\nb: ! x\r\n", node: []int{3}, want: true},
		{name: "tag after CR is detected", src: "a: 1\rb: ! x\r", node: []int{3}, want: true},
		{name: "tag after NEL is detected", src: "a: 1\u0085b: ! x\u0085", node: []int{3}, want: true},
		{name: "tag after LS is detected", src: "a: 1\u2028b: ! x\u2028", node: []int{3}, want: true},
		{name: "tag after PS is detected", src: "a: 1\u2029b: ! x\u2029", node: []int{3}, want: true},
		{name: "tag on last line without trailing newline is detected", src: "a: 1\nb: ! x", node: []int{3}, want: true},
		{name: "tag after leading BOM is detected", src: "\uFEFFa: ! x\n", node: []int{1}, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, err := doc.hasTag(nodeAt(doc.root, tc.node))
			if err != nil {
				t.Fatalf("hasTag() error = %v", err)
			}
			if got != tc.want {
				t.Errorf("hasTag() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestDocumentHasTagOutsideSource は、ライブラリの位置の数え方とソースの行の分け方が食い違ったときに、
// panic せずタグなしとしても通さず、エラーを返すことを確かめる。この位置を返させる入力は見つかっていないため、ノードの位置を書き換えて作る。
//
//trace:checks FR-001
func TestDocumentHasTagOutsideSource(t *testing.T) {
	// "a: x\n" は 2 行 (2 行目は空) に分かれ、値なしの null が指し得る 3 行目まではソースの範囲とみなす
	cases := []struct {
		name   string
		line   int
		column int
	}{
		{name: "line 0 is rejected", line: 0, column: 1},
		{name: "line beyond the line after the last is rejected", line: 4, column: 1},
		{name: "column 0 is rejected", line: 1, column: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, "a: x\n")
			n := *doc.root.Content[1]
			n.Line, n.Column = tc.line, tc.column
			if _, err := doc.hasTag(&n); err == nil || !strings.Contains(err.Error(), "outside the source") {
				t.Errorf("hasTag() error = %v, want it to contain %q", err, "outside the source")
			}
		})
	}
}

//trace:checks FR-001
func TestDocumentCheckNode(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		node    []int
		wantErr string
	}{
		{name: "plain scalar is accepted", src: "x\n"},
		{name: "anchor is rejected", src: "&a x\n", wantErr: "anchors are not allowed"},
		{name: "alias is rejected", src: "[&a x, *a]\n", node: []int{1}, wantErr: "aliases are not allowed"},
		{name: "explicit tag is rejected", src: "!!bool \"true\"\n", wantErr: "explicit tags are not allowed"},
		{name: "null is rejected", src: "null\n", wantErr: "must not be null"},
		{name: "missing value is rejected as null", src: "a:\n", node: []int{1}, wantErr: "must not be null"},
		{
			// 根の null がソースの最終行の次の行を指しても、タグの判定を経て null として拒否する
			name:    "missing value after last line is rejected as null",
			src:     "---",
			wantErr: "must not be null",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			err := doc.checkNode(nodeAt(doc.root, tc.node), "v")
			if tc.wantErr == "" {
				if err != nil {
					t.Errorf("checkNode() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("checkNode() error = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

// TestDocumentCheckNodeOutsideSource は、タグの判定が位置の食い違いで失敗したら、問題の箇所を付けて拒否することを確かめる。
//
//trace:checks FR-001
func TestDocumentCheckNodeOutsideSource(t *testing.T) {
	doc := decodeSnippet(t, "x\n")
	n := *doc.root
	n.Line = 0
	err := doc.checkNode(&n, "v")
	if err == nil || !strings.HasPrefix(err.Error(), "v (") || !strings.Contains(err.Error(), "outside the source") {
		t.Errorf("checkNode() error = %v, want it to start with %q and contain %q", err, "v (", "outside the source")
	}
}

//trace:checks FR-001
func TestDocumentMappingEntries(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		want    []string
		wantErr string
	}{
		{name: "mapping with every key once is accepted", src: "{a: 1, b: 2}\n", want: []string{"a", "b"}},
		{name: "null is rejected", src: "~\n", wantErr: "must not be null"},
		{name: "sequence is rejected as not a mapping", src: "[a, b]\n", wantErr: "must be a mapping"},
		{name: "empty mapping is rejected", src: "{}\n", wantErr: "must not be empty"},
		{name: "anchored key is rejected", src: "{&k a: 1, b: 2}\n", wantErr: "anchors are not allowed"},
		{name: "merge key is rejected", src: "{a: 1, b: 2, <<: {c: 3}}\n", wantErr: "merge keys are not allowed"},
		{name: "non-string key is rejected", src: "{a: 1, b: 2, true: x}\n", wantErr: "keys must be strings"},
		{name: "unknown key is rejected", src: "{a: 1, b: 2, c: 3}\n", wantErr: `unknown key "c"`},
		{name: "duplicate key with quoted notation is rejected", src: "{a: 1, b: 2, \"a\": 3}\n", wantErr: `duplicate key "a"`},
		{name: "missing key is rejected", src: "{a: 1}\n", wantErr: `missing key "b"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, err := doc.mappingEntries(doc.root, "v", "a", "b")
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("mappingEntries() error = %v, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("mappingEntries() error = %v", err)
			}
			if keys := slices.Sorted(maps.Keys(got)); !slices.Equal(keys, tc.want) {
				t.Errorf("mappingEntries() keys = %v, want %v", keys, tc.want)
			}
		})
	}
}

//trace:checks FR-001
func TestDocumentSequenceItems(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		want    int
		wantErr string
	}{
		{name: "sequence is accepted", src: "[a, b]\n", want: 2},
		{name: "null is rejected", src: "~\n", wantErr: "must not be null"},
		{name: "mapping is rejected as not a sequence", src: "{a: 1}\n", wantErr: "must be a sequence"},
		{name: "empty sequence is rejected", src: "[]\n", wantErr: "must not be empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, err := doc.sequenceItems(doc.root, "v")
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("sequenceItems() error = %v, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("sequenceItems() error = %v", err)
			}
			if len(got) != tc.want {
				t.Errorf("sequenceItems() returned %d items, want %d", len(got), tc.want)
			}
		})
	}
}

//trace:checks FR-001
func TestDocumentBoolValue(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		node    []int
		want    bool
		wantErr string
	}{
		{name: "true is read as true", src: "true\n", want: true},
		{name: "false is read as false", src: "false\n", want: false},
		{name: "explicit tag is rejected", src: "!!bool \"true\"\n", wantErr: "explicit tags are not allowed"},
		{name: "quoted true is rejected", src: "\"true\"\n", wantErr: "must be true or false"},
		{name: "uppercase TRUE is rejected", src: "TRUE\n", wantErr: "must be true or false"},
		{name: "yes is rejected", src: "yes\n", wantErr: "must be true or false"},
		{name: "number 1 is rejected", src: "1\n", wantErr: "must be true or false"},
		{name: "error shows path, line and column", src: "a: TRUE\n", node: []int{1}, wantErr: "v (line 1, column 4): must be true or false"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, err := doc.boolValue(nodeAt(doc.root, tc.node), "v")
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("boolValue() error = %v, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("boolValue() error = %v", err)
			}
			if got != tc.want {
				t.Errorf("boolValue() = %v, want %v", got, tc.want)
			}
		})
	}
}

//trace:checks FR-001
func TestDocumentStringValue(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		want    string
		wantErr string
	}{
		{name: "plain string is accepted", src: "owner/repo\n", want: "owner/repo"},
		{name: "explicit tag is rejected", src: "!!str owner/repo\n", wantErr: "explicit tags are not allowed"},
		{name: "number is rejected as not a string", src: "42\n", wantErr: "must be a string"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, err := doc.stringValue(doc.root, "v")
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("stringValue() error = %v, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("stringValue() error = %v", err)
			}
			if got != tc.want {
				t.Errorf("stringValue() = %q, want %q", got, tc.want)
			}
		})
	}
}
