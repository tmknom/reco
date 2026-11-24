package changeplan

import (
	"testing"

	"github.com/tmknom/reco/internal/github"
)

//trace:checks INV-004
func TestRulesetIdentity(t *testing.T) {
	cases := []struct {
		name      string
		identity  RulesetIdentity
		wantID    github.RulesetID
		wantHasID bool
	}{
		{name: "observed identity has its ID", identity: ObservedRulesetIdentity(42), wantID: 42, wantHasID: true},
		{name: "observed identity of ID 0 still has a basis", identity: ObservedRulesetIdentity(0), wantHasID: true},
		{name: "zero value has no basis", identity: RulesetIdentity{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id, hasID := tc.identity.ID()
			if id != tc.wantID || hasID != tc.wantHasID {
				t.Errorf("ID() = %v, %v, want %v, %v", id, hasID, tc.wantID, tc.wantHasID)
			}
		})
	}
}
