package changeplan

import (
	"testing"

	"github.com/tmknom/reco/internal/github"
)

//trace:checks INV-004
func TestEnvironmentIdentity(t *testing.T) {
	cases := []struct {
		name        string
		identity    EnvironmentIdentity
		wantID      github.EnvironmentID
		wantHasID   bool
		wantCreated bool
	}{
		{name: "observed identity has its ID and is not a creation", identity: ObservedEnvironmentIdentity(7), wantID: 7, wantHasID: true},
		{name: "creation by a preceding operation has no ID", identity: CreatedByPrecedingOperation(), wantCreated: true},
		{name: "zero value has no basis", identity: EnvironmentIdentity{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id, hasID := tc.identity.ID()
			if id != tc.wantID || hasID != tc.wantHasID {
				t.Errorf("ID() = %v, %v, want %v, %v", id, hasID, tc.wantID, tc.wantHasID)
			}
			if got := tc.identity.IsCreatedByPrecedingOperation(); got != tc.wantCreated {
				t.Errorf("IsCreatedByPrecedingOperation() = %v, want %v", got, tc.wantCreated)
			}
		})
	}
}
