package show

import (
	"testing"

	"go.yaml.in/yaml/v3"
)

// quote のエスケープを、設定ファイルを読むライブラリで読み戻して確かめる。
func TestQuote(t *testing.T) {
	cases := []struct {
		name string
		s    string
	}{
		{name: "quotes and backslashes are read back", s: `a"b\c`},
		{name: "YAML indicators stay inside the scalar", s: "#x: y, [z]"},
		{name: "printable non-ASCII is read back", s: "é/*"},
		{name: "control characters escaped with a single letter are read back", s: "\a\b\f\n\r\t\v"},
		{name: "control characters escaped with x are read back", s: "\x01\x7f"},
		{name: "non-printable characters escaped with u and U are read back", s: "\u200b\U000e0001"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			if err := yaml.Unmarshal([]byte(quote(tc.s)), &got); err != nil {
				t.Fatalf("yaml.Unmarshal(%s) error = %v", quote(tc.s), err)
			}
			if got != tc.s {
				t.Errorf("yaml.Unmarshal(%s) = %q, want %q", quote(tc.s), got, tc.s)
			}
		})
	}
}
