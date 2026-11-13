package setting

import (
	"slices"
	"testing"
)

func TestGeneralKeys_FollowsConfigurationTableOrder(t *testing.T) {
	want := []Key{DeleteBranchOnMerge, AllowAutoMerge}
	if got := GeneralKeys(); !slices.Equal(got, want) {
		t.Errorf("GeneralKeys() = %v, want %v", got, want)
	}
}
