package show

import (
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/github"
)

//trace:checks FR-004 QR-002
func TestWriteRepositoryVariables(t *testing.T) {
	cases := []struct {
		name      string
		variables github.Variables
		want      string
	}{
		{
			name:      "no variables shows none as a comment on the key",
			variables: github.Variables{Existing: []github.Variable{}},
			want:      "      # variables: none\n",
		},
		{
			name:      "variables are shown as keys under actions",
			variables: github.Variables{Existing: []github.Variable{{Name: "RELEASE", Value: "1.27"}}},
			want:      "      variables:\n        - name: \"RELEASE\"\n          value: \"1.27\"\n",
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

//trace:checks FR-004 QR-002
func TestWriteEnvironmentVariables(t *testing.T) {
	cases := []struct {
		name      string
		variables github.Variables
		want      string
	}{
		{
			name:      "no variables shows none as a comment on the key without the guidance line",
			variables: github.Variables{Existing: []github.Variable{}},
			want:      "        # variables: none\n",
		},
		{
			name:      "variables are shown as comments under the environment after the guidance line",
			variables: github.Variables{Existing: []github.Variable{{Name: "REGION", Value: "eu"}}},
			want: "        # reco does not accept the environment variables in the configuration file yet\n" +
				"        # variables:\n        #   - name: \"REGION\"\n        #     value: \"eu\"\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder
			writeEnvironmentVariables(&b, tc.variables)
			if b.String() != tc.want {
				t.Errorf("writeEnvironmentVariables() wrote %q, want %q", b.String(), tc.want)
			}
		})
	}
}

//trace:checks FR-004 QR-002
func TestWriteVariableLines(t *testing.T) {
	longValue := strings.Repeat("a", 48_000)
	cases := []struct {
		name      string
		variables github.Variables
		want      string
	}{
		{
			name: "variables are shown in the given order with quoted names and values after the prefix",
			variables: github.Variables{Existing: []github.Variable{
				{Name: "RELEASE", Value: "1.27"},
				{Name: "APP", Value: "true"},
			}},
			want: "> variables:\n" +
				">   - name: \"RELEASE\"\n>     value: \"1.27\"\n" +
				">   - name: \"APP\"\n>     value: \"true\"\n",
		},
		{
			name:      "an empty value is shown as an empty string",
			variables: github.Variables{Existing: []github.Variable{{Name: "EMPTY", Value: ""}}},
			want:      "> variables:\n>   - name: \"EMPTY\"\n>     value: \"\"\n",
		},
		{
			name:      "a long value is shown without truncation",
			variables: github.Variables{Existing: []github.Variable{{Name: "LONG", Value: longValue}}},
			want:      "> variables:\n>   - name: \"LONG\"\n>     value: \"" + longValue + "\"\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder
			writeVariableLines(&b, "> ", tc.variables)
			if b.String() != tc.want {
				t.Errorf("writeVariableLines() wrote %q, want %q", b.String(), tc.want)
			}
		})
	}
}
