package show

import (
	"fmt"
	"strings"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// writeSecurity は security を書く。設定宣言が security を受け付けるまでは、表示を設定宣言として読めるよう全行をコメントにする。
func writeSecurity(b *strings.Builder, d github.Dependabot) {
	b.WriteString("    # reco does not accept the security settings in the configuration file yet\n    # security:\n")
	fmt.Fprintf(b, "    #   dependabot_alerts: %t\n", d.Alerts)
	b.WriteString("    #   dependabot_security_updates: " + securityUpdatesText(d.SecurityUpdates) + "\n")
}

// securityUpdatesText は Dependabot security updates の値の表示を返す。
// 設定を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かず、定義にない値は panic する。
func securityUpdatesText(v setting.DependabotSecurityUpdates) string {
	switch v {
	case setting.DependabotSecurityUpdatesDisabled:
		return "false"
	case setting.DependabotSecurityUpdatesEnabled:
		return "true"
	case setting.DependabotSecurityUpdatesPaused:
		return "true # paused by GitHub (still enabled)"
	}
	panic(fmt.Sprintf("show: undefined dependabot security updates value %d", int(v)))
}
