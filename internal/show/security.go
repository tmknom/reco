package show

import (
	"fmt"
	"strings"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// writeSecurity は security を書く。
func writeSecurity(b *strings.Builder, d github.Dependabot, s github.SecretScanning, p github.PrivateVulnerabilityReporting) {
	b.WriteString("    security:\n")
	fmt.Fprintf(b, "      dependabot_alerts: %t\n", d.Alerts)
	b.WriteString("      dependabot_security_updates: " + securityUpdatesText(d.SecurityUpdates) + "\n")
	b.WriteString(publicRepositorySettingLine("secret_scanning", s.Enabled))
	b.WriteString(publicRepositorySettingLine("secret_scanning_push_protection", s.PushProtection))
	b.WriteString(publicRepositorySettingLine("private_vulnerability_reporting", p.Enabled))
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

// publicRepositorySettingLine は、非公開リポジトリでは観測不能になる真偽値の設定 key の行を返す。観測不能の値は期待値として読ませないよう、YAML のコメントにする。
// 非公開以外の理由はこれらの設定に当たらず、非公開の文言で黙って示さないよう不変条件の破れとして panic する。
func publicRepositorySettingLine(key string, o github.SettingObservation[bool]) string {
	if enabled, ok := o.Value(); ok {
		return fmt.Sprintf("      %s: %t\n", key, enabled)
	}
	if reason := o.UnavailableReason(); reason != github.UnavailablePrivateRepository {
		panic(fmt.Sprintf("show: undefined unavailable reason %d for %s", int(reason), key))
	}
	return "      # " + key + ": unavailable (the repository is private)\n"
}
