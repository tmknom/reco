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
	disabledPrivateVulnerabilityReporting := github.PrivateVulnerabilityReporting{Enabled: github.Observed(false)}
	const disabledPrivateVulnerabilityReportingLine = "      private_vulnerability_reporting: false\n"
	cases := []struct {
		name                          string
		dependabot                    github.Dependabot
		secretScanning                github.SecretScanning
		privateVulnerabilityReporting github.PrivateVulnerabilityReporting
		want                          string
	}{
		{
			name:                          "enabled alerts and disabled security updates are shown as booleans",
			dependabot:                    github.Dependabot{Alerts: true, SecurityUpdates: setting.DependabotSecurityUpdatesDisabled},
			secretScanning:                enabledSecretScanning,
			privateVulnerabilityReporting: disabledPrivateVulnerabilityReporting,
			want:                          header + "      dependabot_alerts: true\n      dependabot_security_updates: false\n" + enabledSecretScanningLines + disabledPrivateVulnerabilityReportingLine,
		},
		{
			name:                          "disabled alerts and enabled security updates are shown without the paused comment",
			dependabot:                    github.Dependabot{Alerts: false, SecurityUpdates: setting.DependabotSecurityUpdatesEnabled},
			secretScanning:                enabledSecretScanning,
			privateVulnerabilityReporting: disabledPrivateVulnerabilityReporting,
			want:                          header + "      dependabot_alerts: false\n      dependabot_security_updates: true\n" + enabledSecretScanningLines + disabledPrivateVulnerabilityReportingLine,
		},
		{
			name:                          "paused security updates are shown as true with the paused comment",
			dependabot:                    github.Dependabot{Alerts: true, SecurityUpdates: setting.DependabotSecurityUpdatesPaused},
			secretScanning:                enabledSecretScanning,
			privateVulnerabilityReporting: disabledPrivateVulnerabilityReporting,
			want:                          header + "      dependabot_alerts: true\n      dependabot_security_updates: true # paused by GitHub (still enabled)\n" + enabledSecretScanningLines + disabledPrivateVulnerabilityReportingLine,
		},
		{
			name:                          "enabled secret scanning and disabled push protection are shown as booleans",
			dependabot:                    github.Dependabot{Alerts: true, SecurityUpdates: setting.DependabotSecurityUpdatesEnabled},
			secretScanning:                github.SecretScanning{Enabled: github.Observed(true), PushProtection: github.Observed(false)},
			privateVulnerabilityReporting: github.PrivateVulnerabilityReporting{Enabled: github.Observed(true)},
			want: header + "      dependabot_alerts: true\n      dependabot_security_updates: true\n" +
				"      secret_scanning: true\n      secret_scanning_push_protection: false\n      private_vulnerability_reporting: true\n",
		},
		{
			name:       "settings unavailable for the private repository are shown as unavailable in comments",
			dependabot: github.Dependabot{Alerts: true, SecurityUpdates: setting.DependabotSecurityUpdatesEnabled},
			secretScanning: github.SecretScanning{
				Enabled:        github.Unavailable[bool](github.UnavailablePrivateRepository),
				PushProtection: github.Unavailable[bool](github.UnavailablePrivateRepository),
			},
			privateVulnerabilityReporting: github.PrivateVulnerabilityReporting{Enabled: github.Unavailable[bool](github.UnavailablePrivateRepository)},
			want: header + "      dependabot_alerts: true\n      dependabot_security_updates: true\n" +
				"      # secret_scanning: unavailable (the repository is private)\n" +
				"      # secret_scanning_push_protection: unavailable (the repository is private)\n" +
				"      # private_vulnerability_reporting: unavailable (the repository is private)\n",
		},
		{
			name:       "an unknown secret scanning value is a comment and the push protection is shown",
			dependabot: github.Dependabot{Alerts: true, SecurityUpdates: setting.DependabotSecurityUpdatesEnabled},
			secretScanning: github.SecretScanning{
				Enabled:        github.Unavailable[bool](github.UnavailableUnknownValue),
				PushProtection: github.Observed(false),
			},
			privateVulnerabilityReporting: disabledPrivateVulnerabilityReporting,
			want: header + "      dependabot_alerts: true\n      dependabot_security_updates: true\n" +
				"      # secret_scanning: unavailable (GitHub returned a value that reco does not know)\n" +
				"      secret_scanning_push_protection: false\n" + disabledPrivateVulnerabilityReportingLine,
		},
		{
			name:       "an unknown push protection value is a comment and the secret scanning is shown",
			dependabot: github.Dependabot{Alerts: true, SecurityUpdates: setting.DependabotSecurityUpdatesEnabled},
			secretScanning: github.SecretScanning{
				Enabled:        github.Observed(true),
				PushProtection: github.Unavailable[bool](github.UnavailableUnknownValue),
			},
			privateVulnerabilityReporting: disabledPrivateVulnerabilityReporting,
			want: header + "      dependabot_alerts: true\n      dependabot_security_updates: true\n" +
				"      secret_scanning: true\n" +
				"      # secret_scanning_push_protection: unavailable (GitHub returned a value that reco does not know)\n" + disabledPrivateVulnerabilityReportingLine,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder
			writeSecurity(&b, tc.dependabot, tc.secretScanning, tc.privateVulnerabilityReporting)
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
		writeSecurity(&strings.Builder{}, github.Dependabot{Alerts: true}, enabledSecretScanning, disabledPrivateVulnerabilityReporting)
	})

	t.Run("private vulnerability reporting unavailable for an unknown value panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("writeSecurity() did not panic for private vulnerability reporting unavailable because of an unknown value")
			}
		}()
		writeSecurity(&strings.Builder{}, github.Dependabot{Alerts: true, SecurityUpdates: setting.DependabotSecurityUpdatesEnabled}, enabledSecretScanning,
			github.PrivateVulnerabilityReporting{Enabled: github.Unavailable[bool](github.UnavailableUnknownValue)})
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
		}, disabledPrivateVulnerabilityReporting)
	})
}
