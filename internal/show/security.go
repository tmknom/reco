package show

import (
	"fmt"
	"strings"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// writeSecurity は security を書く。
func writeSecurity(b *strings.Builder, d github.Dependabot, s github.SecretScanning) {
	b.WriteString("    security:\n")
	fmt.Fprintf(b, "      dependabot_alerts: %t\n", d.Alerts)
	b.WriteString("      dependabot_security_updates: " + securityUpdatesText(d.SecurityUpdates) + "\n")
	b.WriteString("      # reco does not accept the secret scanning settings in the configuration file yet\n")
	b.WriteString("      # secret_scanning: " + secretScanningValueText(s.Enabled) + "\n")
	b.WriteString("      # secret_scanning_push_protection: " + secretScanningValueText(s.PushProtection) + "\n")
}

// securityUpdatesText は Dependabot security updates の値の表示を返す。
// 設定を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かず、定義にない値は panic する。
func securityUpdatesText(v setting.DependabotSecurityUpdatesState) string {
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

// secretScanningValueText は Secret scanning の設定の値の表示を返す。
// 非公開以外の理由はこれらの設定に当たらず、非公開の文言で黙って示さないよう不変条件の破れとして panic する。
func secretScanningValueText(o github.SettingObservation[bool]) string {
	if enabled, ok := o.Value(); ok {
		return fmt.Sprintf("%t", enabled)
	}
	if reason := o.UnavailableReason(); reason != github.UnavailablePrivateRepository {
		panic(fmt.Sprintf("show: undefined unavailable reason %d for secret scanning", int(reason)))
	}
	return "unavailable (the repository is private)"
}
