package show

import (
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/github"
)

//trace:checks FR-004 QR-002
func TestWriteRepositoryVariables(t *testing.T) {
	const notAccepted = "      # reco does not accept the variables in the configuration file yet\n"
	longValue := strings.Repeat("a", 48_000)
	cases := []struct {
		name      string
		variables github.Variables
		want      string
	}{
		{
			name:      "no variables shows none as a comment on the key without the notice",
			variables: github.Variables{Existing: []github.Variable{}},
			want:      "      # variables: none\n",
		},
		{
			name: "variables are shown as comments after the notice in the given order with quoted names and values",
			variables: github.Variables{Existing: []github.Variable{
				{Name: "RELEASE", Value: "1.27"},
				{Name: "APP", Value: "true"},
			}},
			want: notAccepted +
				"      # variables:\n" +
				"      #   - name: \"RELEASE\"\n      #     value: \"1.27\"\n" +
				"      #   - name: \"APP\"\n      #     value: \"true\"\n",
		},
		{
			name:      "an empty value is shown as an empty string",
			variables: github.Variables{Existing: []github.Variable{{Name: "EMPTY", Value: ""}}},
			want:      notAccepted + "      # variables:\n      #   - name: \"EMPTY\"\n      #     value: \"\"\n",
		},
		{
			name:      "a name and a value with a newline and ESC are escaped on one line",
			variables: github.Variables{Existing: []github.Variable{{Name: "A\nB", Value: "a\n\x1b[31mb"}}},
			want:      notAccepted + "      # variables:\n      #   - name: \"A\\nB\"\n      #     value: \"a\\n\\x1b[31mb\"\n",
		},
		{
			name:      "a long value is shown without truncation",
			variables: github.Variables{Existing: []github.Variable{{Name: "LONG", Value: longValue}}},
			want:      notAccepted + "      # variables:\n      #   - name: \"LONG\"\n      #     value: \"" + longValue + "\"\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder
			writeRepositoryVariables(&b, tc.variables)
			if b.String() != tc.want {
				t.Errorf("writeRepositoryVariables() wrote %q, want %q", b.String(), tc.want)
			}
		})
	}
}
