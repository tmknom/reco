package display

import (
	"testing"

	"github.com/tmknom/reco/internal/github"
)

//trace:checks QR-002
func TestUnavailableText(t *testing.T) {
	cases := []struct {
		reason github.UnavailableReason
		want   string
	}{
		{reason: github.UnavailableActionsDisabled, want: "unavailable (GitHub Actions is disabled)"},
		{reason: github.UnavailableNotSelected, want: `unavailable (allowed_actions is not "selected")`},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := UnavailableText(tc.reason); got != tc.want {
				t.Errorf("UnavailableText() = %q, want %q", got, tc.want)
			}
		})
	}

	t.Run("undefined reason panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("UnavailableText() did not panic for an undefined reason")
			}
		}()
		_ = UnavailableText(0)
	})
}
