package show

import (
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

//trace:checks FR-004 QR-002
func TestWriteSecurity(t *testing.T) {
	const header = "    # reco does not accept the security settings in the configuration file yet\n    # security:\n"
	cases := []struct {
		name       string
		dependabot github.Dependabot
		want       string
	}{
		{
			name:       "enabled alerts and disabled security updates are shown as commented booleans",
			dependabot: github.Dependabot{Alerts: true, SecurityUpdates: setting.DependabotSecurityUpdatesDisabled},
			want:       header + "    #   dependabot_alerts: true\n    #   dependabot_security_updates: false\n",
		},
		{
			name:       "disabled alerts and enabled security updates are shown without the paused comment",
			dependabot: github.Dependabot{Alerts: false, SecurityUpdates: setting.DependabotSecurityUpdatesEnabled},
			want:       header + "    #   dependabot_alerts: false\n    #   dependabot_security_updates: true\n",
		},
		{
			name:       "paused security updates are shown as true with the paused comment",
			dependabot: github.Dependabot{Alerts: true, SecurityUpdates: setting.DependabotSecurityUpdatesPaused},
			want:       header + "    #   dependabot_alerts: true\n    #   dependabot_security_updates: true # paused by GitHub (still enabled)\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder
			writeSecurity(&b, tc.dependabot)
			if b.String() != tc.want {
				t.Errorf("writeSecurity() wrote %q, want %q", b.String(), tc.want)
			}
		})
	}

	t.Run("undefined security updates value panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("writeSecurity() did not panic for the zero value of the security updates")
			}
		}()
		writeSecurity(&strings.Builder{}, github.Dependabot{Alerts: true})
	})
}
