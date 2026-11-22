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

// quote の出力を端末に書いても、制御文字として働く文字や見た目を偽る文字が生のまま届かないことを確かめる。
//
//trace:checks QR-002
func TestQuoteWritesNoTerminalControl(t *testing.T) {
	// C0、DEL、C1 の制御文字と、行区切り、書字方向の制御、ゼロ幅の文字
	input := "\x00\x07\x1b[2J\r\n\x7f\u0085\u009b\u2028\u2029\u202e\u2066\u200b\ufeff"
	got := quote(input)
	for _, r := range got {
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) || r == '\u2028' || r == '\u2029' || r == '\u202e' || r == '\u2066' || r == '\u200b' || r == '\ufeff' {
			t.Errorf("quote(%q) = %q, want no raw %U", input, got, r)
		}
	}
}
