package show

import (
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

//trace:checks FR-004 QR-002
func TestWriteSecurity(t *testing.T) {
	const header = "    security:\n"
	enabledSecretScanning := github.SecretScanning{Enabled: github.Observed(true), PushProtection: github.Observed(true)}
	enabledSecretScanningLines := "      secret_scanning: true\n      secret_scanning_push_protection: true\n"
	cases := []struct {
		name           string
		dependabot     github.Dependabot
		secretScanning github.SecretScanning
		want           string
	}{
		{
			name:           "enabled alerts and disabled security updates are shown as booleans",
			dependabot:     github.Dependabot{Alerts: true, SecurityUpdates: setting.DependabotSecurityUpdatesDisabled},
			secretScanning: enabledSecretScanning,
			want:           header + "      dependabot_alerts: true\n      dependabot_security_updates: false\n" + enabledSecretScanningLines,
		},
		{
			name:           "disabled alerts and enabled security updates are shown without the paused comment",
			dependabot:     github.Dependabot{Alerts: false, SecurityUpdates: setting.DependabotSecurityUpdatesEnabled},
			secretScanning: enabledSecretScanning,
			want:           header + "      dependabot_alerts: false\n      dependabot_security_updates: true\n" + enabledSecretScanningLines,
		},
		{
			name:           "paused security updates are shown as true with the paused comment",
			dependabot:     github.Dependabot{Alerts: true, SecurityUpdates: setting.DependabotSecurityUpdatesPaused},
			secretScanning: enabledSecretScanning,
			want:           header + "      dependabot_alerts: true\n      dependabot_security_updates: true # paused by GitHub (still enabled)\n" + enabledSecretScanningLines,
		},
		{
			name:           "enabled secret scanning and disabled push protection are shown as booleans",
			dependabot:     github.Dependabot{Alerts: true, SecurityUpdates: setting.DependabotSecurityUpdatesEnabled},
			secretScanning: github.SecretScanning{Enabled: github.Observed(true), PushProtection: github.Observed(false)},
			want: header + "      dependabot_alerts: true\n      dependabot_security_updates: true\n" +
				"      secret_scanning: true\n      secret_scanning_push_protection: false\n",
		},
		{
			name:       "secret scanning settings unavailable for the private repository are shown as unavailable in comments",
			dependabot: github.Dependabot{Alerts: true, SecurityUpdates: setting.DependabotSecurityUpdatesEnabled},
			secretScanning: github.SecretScanning{
				Enabled:        github.Unavailable[bool](github.UnavailablePrivateRepository),
				PushProtection: github.Unavailable[bool](github.UnavailablePrivateRepository),
			},
			want: header + "      dependabot_alerts: true\n      dependabot_security_updates: true\n" +
				"      # secret_scanning: unavailable (the repository is private)\n" +
				"      # secret_scanning_push_protection: unavailable (the repository is private)\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder
			writeSecurity(&b, tc.dependabot, tc.secretScanning)
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
		writeSecurity(&strings.Builder{}, github.Dependabot{Alerts: true}, enabledSecretScanning)
	})

	t.Run("secret scanning unavailable for a reason other than private panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("writeSecurity() did not panic for secret scanning unavailable while GitHub Actions is disabled")
			}
		}()
		writeSecurity(&strings.Builder{}, github.Dependabot{Alerts: true, SecurityUpdates: setting.DependabotSecurityUpdatesEnabled}, github.SecretScanning{
			Enabled:        github.Unavailable[bool](github.UnavailableActionsDisabled),
			PushProtection: github.Observed(true),
		})
	})
}
