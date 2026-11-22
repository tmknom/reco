package show

import (
	"fmt"
	"strings"

	"github.com/tmknom/reco/internal/github"
)

// writeRepositoryVariables は actions.variables を書く。
func writeRepositoryVariables(b *strings.Builder, v github.Variables) {
	if len(v.Existing) == 0 {
		b.WriteString("      # variables: none\n")
		return
	}
	writeVariableLines(b, "      ", v)
}

// writeEnvironmentVariables は environments[].variables を書く。
func writeEnvironmentVariables(b *strings.Builder, v github.Variables) {
	if len(v.Existing) == 0 {
		b.WriteString("        # variables: none\n")
		return
	}
	b.WriteString("        # reco does not accept the environment variables in the configuration file yet\n")
	writeVariableLines(b, "        # ", v)
}

// writeVariableLines は、variables のキーと変数ごとの name と value の行を、各行の先頭に prefix を付けて書く。
func writeVariableLines(b *strings.Builder, prefix string, v github.Variables) {
	b.WriteString(prefix + "variables:\n")
	for _, variable := range v.Existing {
		fmt.Fprintf(b, "%s  - name: %s\n%s    value: %s\n", prefix, quote(variable.Name), prefix, quote(variable.Value))
	}
}
