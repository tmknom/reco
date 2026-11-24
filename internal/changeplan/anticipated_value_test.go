package changeplan

import (
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// TestAnticipatedValue_UnavailableReason は、観測不能のままと予定したときに、その理由を読めることを確かめる。
func TestAnticipatedValue_UnavailableReason(t *testing.T) {
	anticipated := AnticipateUnchanged(github.Unavailable[setting.Value](github.UnavailableActionsDisabled))
	if got := anticipated.UnavailableReason(); got != github.UnavailableActionsDisabled {
		t.Errorf("UnavailableReason() = %v, want %v", got, github.UnavailableActionsDisabled)
	}
}
