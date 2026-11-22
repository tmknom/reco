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
	b.WriteString("      variables:\n")
	for _, variable := range v.Existing {
		fmt.Fprintf(b, "        - name: %s\n          value: %s\n", quote(variable.Name), quote(variable.Value))
	}
}
