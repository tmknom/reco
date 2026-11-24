package configuration

import (
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/tmknom/reco/internal/setting"
)

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
		{name: "mapping with required and optional keys is accepted", src: "{a: 1, b: 2}\n", want: []string{"a", "b"}},
		{name: "omitted optional key is not returned", src: "{a: 1}\n", want: []string{"a"}},
		{name: "null is rejected", src: "~\n", wantErr: "must not be null"},
		{name: "sequence is rejected as not a mapping", src: "[a, b]\n", wantErr: "must be a mapping"},
		{name: "empty mapping is rejected", src: "{}\n", wantErr: "must not be empty"},
		{name: "anchored key is rejected", src: "{&k a: 1, b: 2}\n", wantErr: "anchors are not allowed"},
		{name: "merge key is rejected", src: "{a: 1, b: 2, <<: {c: 3}}\n", wantErr: "merge keys are not allowed"},
		{name: "non-string key is rejected", src: "{a: 1, b: 2, true: x}\n", wantErr: "keys must be strings"},
		{name: "unknown key is rejected listing required and optional keys", src: "{a: 1, b: 2, c: 3}\n", wantErr: `unknown key "c"; expected a, b`},
		{name: "duplicate key with quoted notation is rejected", src: "{a: 1, b: 2, \"a\": 3}\n", wantErr: `duplicate key "a"`},
		{name: "missing required key is rejected", src: "{b: 2}\n", wantErr: `missing key "a"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, err := doc.mappingEntries(doc.root, "v", []string{"a"}, []string{"b"})
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
		{name: "double-quoted true is rejected with a hint to remove the quotes", src: "\"true\"\n", wantErr: `must be true or false; got the quoted string "true"; write true without quotes`},
		{name: "single-quoted FALSE is rejected with a hint to write lowercase false without quotes", src: "'FALSE'\n", wantErr: `must be true or false; got the quoted string "FALSE"; write false without quotes`},
		{name: "uppercase TRUE is rejected with a hint to write lowercase", src: "TRUE\n", wantErr: "must be true or false; got TRUE; write true in lowercase"},
		{name: "yes is rejected without a hint", src: "yes\n", wantErr: `must be true or false (unquoted, lowercase); got "yes"`},
		{name: "quoted yes is rejected without a hint", src: "\"yes\"\n", wantErr: `must be true or false (unquoted, lowercase); got "yes"`},
		{name: "folded block true is rejected without a hint", src: ">-\n  true\n", wantErr: `must be true or false (unquoted, lowercase); got "true"`},
		{name: "number 1 is rejected without a hint", src: "1\n", wantErr: `must be true or false (unquoted, lowercase); got "1"`},
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
func TestDocumentIntValue(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		node    []int
		want    int
		wantErr string
	}{
		{name: "5 is read as 5", src: "5\n", want: 5},
		{name: "0 is read as 0", src: "0\n", want: 0},
		{name: "explicit tag is rejected", src: "!!int 5\n", wantErr: "explicit tags are not allowed"},
		{name: "null is rejected", src: "~\n", wantErr: "must not be null or empty"},
		{name: "quoted number is rejected", src: "\"5\"\n", wantErr: `must be an integer; got "5"`},
		{name: "float is rejected", src: "5.0\n", wantErr: `must be an integer; got "5.0"`},
		{name: "hexadecimal is rejected", src: "0x5\n", wantErr: `must be an integer; got "0x5"`},
		{name: "octal with o prefix is rejected", src: "0o5\n", wantErr: `must be an integer; got "0o5"`},
		{name: "zero-padded number is rejected", src: "05\n", wantErr: `must be an integer; got "05"`},
		{name: "plus sign is rejected", src: "+5\n", wantErr: `must be an integer; got "+5"`},
		{name: "negative number is rejected", src: "-1\n", wantErr: `must be an integer; got "-1"`},
		{name: "underscore separator is rejected", src: "1_000\n", wantErr: `must be an integer; got "1_000"`},
		{name: "number beyond the signed 64-bit range is rejected", src: "9223372036854775808\n", wantErr: `must be an integer; got "9223372036854775808"`},
		{name: "number beyond the unsigned 64-bit range is rejected", src: "99999999999999999999\n", wantErr: "must be an integer"},
		{name: "boolean is rejected", src: "true\n", wantErr: `must be an integer; got "true"`},
		{name: "sequence is rejected", src: "[1]\n", wantErr: "must be an integer"},
		{name: "error shows path, line and column", src: "a: 5.0\n", node: []int{1}, wantErr: "v (line 1, column 4): must be an integer"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, err := doc.intValue(nodeAt(doc.root, tc.node), "v")
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("intValue() error = %v, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("intValue() error = %v", err)
			}
			if got != tc.want {
				t.Errorf("intValue() = %d, want %d", got, tc.want)
			}
		})
	}
}

//trace:checks FR-001
func TestDocumentPermissionLevelValue(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		want    setting.PermissionLevel
		wantErr string
	}{
		{name: "read is accepted", src: "read\n", want: setting.PermissionRead},
		{name: "write is accepted", src: "write\n", want: setting.PermissionWrite},
		{name: "quoted read is accepted", src: "\"read\"\n", want: setting.PermissionRead},
		{name: "capitalized Read is rejected", src: "Read\n", wantErr: `must be read or write; got "Read"`},
		{name: "undefined admin is rejected", src: "admin\n", wantErr: `must be read or write; got "admin"`},
		{name: "quoted read with leading space is rejected", src: "\" read\"\n", wantErr: `must be read or write; got " read"`},
		{name: "empty string is rejected", src: "\"\"\n", wantErr: `must be read or write; got ""`},
		{name: "boolean true is rejected as not a string", src: "true\n", wantErr: "must be a string"},
		{name: "number 1 is rejected as not a string", src: "1\n", wantErr: "must be a string"},
		{name: "null is rejected", src: "null\n", wantErr: "must not be null"},
		{name: "explicit tag is rejected", src: "!!str read\n", wantErr: "explicit tags are not allowed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, err := doc.permissionLevelValue(doc.root, "v")
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("permissionLevelValue() error = %v, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("permissionLevelValue() error = %v", err)
			}
			if got != tc.want {
				t.Errorf("permissionLevelValue() = %q, want %q", got, tc.want)
			}
		})
	}
}

//trace:checks FR-001
func TestDocumentAccessLevelValue(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		want    setting.AccessLevel
		wantErr string
	}{
		{name: "none is accepted", src: "none\n", want: setting.AccessNone},
		{name: "user is accepted", src: "user\n", want: setting.AccessUser},
		{name: "organization is rejected with the reason it cannot be declared", src: "organization\n", wantErr: `must be none or user; got "organization"; organization is for organization-owned repositories, which reco does not support`},
		{name: "undefined enterprise is rejected", src: "enterprise\n", wantErr: `must be none or user; got "enterprise"`},
		{name: "boolean true is rejected as not a string", src: "true\n", wantErr: "must be a string"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, err := doc.accessLevelValue(doc.root, "v")
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("accessLevelValue() error = %v, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("accessLevelValue() error = %v", err)
			}
			if got != tc.want {
				t.Errorf("accessLevelValue() = %q, want %q", got, tc.want)
			}
		})
	}
}

//trace:checks FR-001
func TestDocumentForkPullRequestApprovalPolicyValue(t *testing.T) {
	const wantReason = `must be first_time_contributors_new_to_github, first_time_contributors, or all_external_contributors; got `
	cases := []struct {
		name    string
		src     string
		want    setting.ForkPullRequestApprovalPolicy
		wantErr string
	}{
		{name: "first_time_contributors is accepted", src: "first_time_contributors\n", want: setting.ForkPullRequestApprovalFirstTimeContributors},
		{name: "uppercase FIRST_TIME_CONTRIBUTORS is rejected listing the defined values", src: "FIRST_TIME_CONTRIBUTORS\n", wantErr: wantReason + `"FIRST_TIME_CONTRIBUTORS"`},
		{name: "null is rejected", src: "~\n", wantErr: "must not be null"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, err := doc.forkPullRequestApprovalPolicyValue(doc.root, "v")
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("forkPullRequestApprovalPolicyValue() error = %v, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("forkPullRequestApprovalPolicyValue() error = %v", err)
			}
			if got != tc.want {
				t.Errorf("forkPullRequestApprovalPolicyValue() = %q, want %q", got, tc.want)
			}
		})
	}
}

//trace:checks FR-001
func TestDocumentAllowedActionsPolicyValue(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		want    setting.AllowedActionsPolicy
		wantErr string
	}{
		{name: "all is accepted", src: "all\n", want: setting.AllowedActionsAll},
		{name: "local_only is accepted", src: "local_only\n", want: setting.AllowedActionsLocalOnly},
		{name: "selected is accepted", src: "selected\n", want: setting.AllowedActionsSelected},
		{name: "quoted selected is accepted", src: "\"selected\"\n", want: setting.AllowedActionsSelected},
		{name: "capitalized All is rejected", src: "All\n", wantErr: `must be all, local_only, or selected; got "All"`},
		{name: "undefined none is rejected", src: "none\n", wantErr: `must be all, local_only, or selected; got "none"`},
		{name: "empty string is rejected", src: "\"\"\n", wantErr: `must be all, local_only, or selected; got ""`},
		{name: "boolean true is rejected as not a string", src: "true\n", wantErr: "must be a string"},
		{name: "null is rejected", src: "null\n", wantErr: "must not be null"},
		{name: "explicit tag is rejected", src: "!!str all\n", wantErr: "explicit tags are not allowed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, err := doc.allowedActionsPolicyValue(doc.root, "v")
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("allowedActionsPolicyValue() error = %v, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("allowedActionsPolicyValue() error = %v", err)
			}
			if got != tc.want {
				t.Errorf("allowedActionsPolicyValue() = %q, want %q", got, tc.want)
			}
		})
	}
}

//trace:checks FR-001
func TestDocumentPatternsAllowedValue(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		want    []string
		wantErr string
	}{
		{name: "two items are read in declared order", src: "[b/*, a/*]\n", want: []string{"b/*", "a/*"}},
		{name: "one item is read", src: "[a/*]\n", want: []string{"a/*"}},
		{name: "empty sequence is read as non-nil empty", src: "[]\n", want: []string{}},
		{name: "items differing only in letter case are not duplicates", src: "[a/*, A/*]\n", want: []string{"a/*", "A/*"}},
		{name: "item with inner space is accepted", src: "[\"a b\"]\n", want: []string{"a b"}},
		{name: "null is rejected", src: "~\n", wantErr: "must not be null"},
		{name: "string is rejected as not a sequence", src: "a/*\n", wantErr: "must be a sequence"},
		{name: "mapping is rejected as not a sequence", src: "{a: 1}\n", wantErr: "must be a sequence"},
		{name: "empty string item is rejected", src: "[a/*, \"\"]\n", wantErr: `v[1] (line 1, column 7): must not be empty or have leading or trailing whitespace; got ""`},
		{name: "whitespace-only item is rejected", src: "[\" \"]\n", wantErr: `v[0] (line 1, column 2): must not be empty or have leading or trailing whitespace; got " "`},
		{name: "item with leading space is rejected", src: "[\" a\"]\n", wantErr: `v[0] (line 1, column 2): must not be empty or have leading or trailing whitespace; got " a"`},
		{name: "item with trailing space is rejected", src: "[\"a \"]\n", wantErr: `v[0] (line 1, column 2): must not be empty or have leading or trailing whitespace; got "a "`},
		{name: "item wrapped in no-break spaces is rejected", src: "[\"\\u00A0a\\u00A0\"]\n", wantErr: "v[0] (line 1, column 2): must not be empty or have leading or trailing whitespace"},
		{name: "null item is rejected", src: "[a/*, null]\n", wantErr: "v[1] (line 1, column 7): must not be null"},
		{name: "number item is rejected", src: "[a/*, 1]\n", wantErr: "v[1] (line 1, column 7): must be a string"},
		{name: "boolean item is rejected", src: "[a/*, true]\n", wantErr: "v[1] (line 1, column 7): must be a string"},
		{name: "nested sequence item is rejected", src: "[a/*, [b/*]]\n", wantErr: "v[1] (line 1, column 7): must be a string"},
		{name: "mapping item is rejected", src: "[a/*, {b: c}]\n", wantErr: "v[1] (line 1, column 7): must be a string"},
		{name: "tagged item is rejected", src: "[a/*, !!str b/*]\n", wantErr: "v[1] (line 1, column 7): explicit tags are not allowed"},
		{name: "duplicate item is rejected naming the earlier index", src: "[a/*, b/*, a/*]\n", wantErr: `v[2] (line 1, column 12): duplicate pattern "a/*"; same as v[0]`},
		{name: "item differing only in escape notation is rejected as a duplicate", src: "[\"a\", \"\\x61\"]\n", wantErr: `v[1] (line 1, column 7): duplicate pattern "a"; same as v[0]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, err := doc.patternsAllowedValue(doc.root, "v")
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("patternsAllowedValue() error = %v, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("patternsAllowedValue() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("patternsAllowedValue() = %#v, want %#v", got, tc.want)
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

//trace:checks FR-001
func TestDocumentQuotableStringValue(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		want    string
		wantErr string
	}{
		{name: "quoted string that looks like a number is accepted", src: "\"1.27\"\n", want: "1.27"},
		{name: "unquoted float is rejected with its quoted spelling", src: "1.27\n", wantErr: `must be a string; got 1.27; write "1.27" in quotes`},
		{name: "unquoted boolean is rejected with its quoted spelling", src: "true\n", wantErr: `must be a string; got true; write "true" in quotes`},
		{name: "unquoted date is rejected with its quoted spelling", src: "2026-11-22\n", wantErr: `must be a string; got 2026-11-22; write "2026-11-22" in quotes`},
		{name: "mapping is rejected without a quoting hint", src: "{a: b}\n", wantErr: "must be a string"},
		{name: "null is rejected as null without a quoting hint", src: "~\n", wantErr: "must not be null or empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, err := doc.quotableStringValue(doc.root, "v")
			if tc.wantErr != "" {
				if err == nil || !strings.HasSuffix(err.Error(), ": "+tc.wantErr) {
					t.Errorf("quotableStringValue() error = %v, want it to end with %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("quotableStringValue() error = %v", err)
			}
			if got != tc.want {
				t.Errorf("quotableStringValue() = %q, want %q", got, tc.want)
			}
		})
	}
}

// nodeAt は、root から Content の添字を順にたどったノードを返す。添字がなければ root を返す。
func nodeAt(root *yaml.Node, indexes []int) *yaml.Node {
	n := root
	for _, i := range indexes {
		n = n.Content[i]
	}
	return n
}
