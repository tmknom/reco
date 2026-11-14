package setting

import (
	"slices"
	"testing"
)

func TestKeys_FollowsConfigurationOrder(t *testing.T) {
	want := []Key{DeleteBranchOnMerge, AllowAutoMerge, AllowedActions, SelectedActions, DefaultWorkflowPermissions, CanApprovePullRequestReviews}
	if got := Keys(); !slices.Equal(got, want) {
		t.Errorf("Keys() = %v, want %v", got, want)
	}
}

func TestKeyArea(t *testing.T) {
	tests := []struct {
		key  Key
		want Area
	}{
		{key: DeleteBranchOnMerge, want: AreaGeneral},
		{key: AllowAutoMerge, want: AreaGeneral},
		{key: AllowedActions, want: AreaActionsPermissions},
		{key: SelectedActions, want: AreaActionsPermissions},
		{key: DefaultWorkflowPermissions, want: AreaWorkflowPermissions},
		{key: CanApprovePullRequestReviews, want: AreaWorkflowPermissions},
	}
	for _, tt := range tests {
		t.Run(string(tt.key), func(t *testing.T) {
			if got := tt.key.Area(); got != tt.want {
				t.Errorf("Area() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKeyArea_UnknownKeyPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Area() did not panic for an unknown key")
		}
	}()
	Key("general.unknown").Area()
}
