package show

import (
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

//trace:checks FR-004 QR-002
func TestWriteOIDCSubject(t *testing.T) {
	const notAccepted = "      # reco does not accept the OIDC subject customization in the configuration file yet\n      # oidc_subject:\n"
	cases := []struct {
		name          string
		customization setting.OIDCSubjectCustomization
		want          string
	}{
		{
			name:          "default customization omits the claim keys line",
			customization: setting.OIDCSubjectCustomization{UseDefault: true},
			want: notAccepted +
				"      #   use_default: true\n" +
				"      #   use_immutable_subject: false\n",
		},
		{
			// 整列した並びと異なる順にし、並べ替える誤りを検出する。改行を含む key で、コメントの外に出る誤りを検出する
			name:          "claim keys are shown quoted in the observed order with the immutable subject",
			customization: setting.OIDCSubjectCustomization{IncludeClaimKeys: []string{"repo", "context", "a\nb"}, UseImmutableSubject: true},
			want: notAccepted +
				"      #   use_default: false\n" +
				"      #   include_claim_keys:\n" +
				"      #     - \"repo\"\n" +
				"      #     - \"context\"\n" +
				"      #     - \"a\\nb\"\n" +
				"      #   use_immutable_subject: true\n",
		},
		{
			name:          "customization without claim keys shows the empty sequence",
			customization: setting.OIDCSubjectCustomization{},
			want: notAccepted +
				"      #   use_default: false\n" +
				"      #   include_claim_keys: []\n" +
				"      #   use_immutable_subject: false\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder
			writeOIDCSubject(&b, github.OIDCSubject{Customization: tc.customization})
			if b.String() != tc.want {
				t.Errorf("writeOIDCSubject() wrote %q, want %q", b.String(), tc.want)
			}
		})
	}
}
