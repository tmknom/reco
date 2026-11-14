package display

import (
	"testing"

	"github.com/tmknom/reco/internal/github"
)

//trace:checks QR-002
func TestUnavailableReasonText(t *testing.T) {
	cases := []struct {
		reason github.UnavailableReason
		want   string
	}{
		{reason: github.UnavailableActionsDisabled, want: "GitHub Actions is disabled"},
		{reason: github.UnavailableNotSelected, want: `allowed_actions is not "selected"`},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := UnavailableReasonText(tc.reason); got != tc.want {
				t.Errorf("UnavailableReasonText() = %q, want %q", got, tc.want)
			}
		})
	}

	t.Run("undefined reason panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("UnavailableReasonText() did not panic for an undefined reason")
			}
		}()
		_ = UnavailableReasonText(0)
	})
}

//trace:checks QR-002
func TestUnavailableText(t *testing.T) {
	want := "unavailable (GitHub Actions is disabled)"
	if got := UnavailableText(github.UnavailableActionsDisabled); got != want {
		t.Errorf("UnavailableText() = %q, want %q", got, want)
	}
}
