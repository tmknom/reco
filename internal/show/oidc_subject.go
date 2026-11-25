package show

import (
	"fmt"
	"strings"

	"github.com/tmknom/reco/internal/github"
)

// writeOIDCSubject は actions.oidc_subject を、設定宣言が受け付けないことの案内に続けて、すべて YAML のコメントとして書く。
// key は quote で一行にし、改行を含む key の後ろがコメントの外に出て設定宣言として読まれないようにする。
func writeOIDCSubject(b *strings.Builder, s github.OIDCSubject) {
	c := s.Customization
	b.WriteString("      # reco does not accept the OIDC subject customization in the configuration file yet\n      # oidc_subject:\n")
	fmt.Fprintf(b, "      #   use_default: %t\n", c.UseDefault)
	if !c.UseDefault {
		writeIncludeClaimKeys(b, c.IncludeClaimKeys)
	}
	fmt.Fprintf(b, "      #   use_immutable_subject: %t\n", c.UseImmutableSubject)
}

// writeIncludeClaimKeys は include_claim_keys を、空なら [] で、そうでなければ観測した順に一行に一つずつ書く。
func writeIncludeClaimKeys(b *strings.Builder, keys []string) {
	if len(keys) == 0 {
		b.WriteString("      #   include_claim_keys: []\n")
		return
	}
	b.WriteString("      #   include_claim_keys:\n")
	for _, key := range keys {
		fmt.Fprintf(b, "      #     - %s\n", quote(key))
	}
}
