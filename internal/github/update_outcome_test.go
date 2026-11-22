package github

import (
	"errors"
	"strings"
	"testing"
)

//trace:checks FR-013
func TestUpdateOutcomeOf(t *testing.T) {
	errFailed := errors.New("failed")
	cases := []struct {
		name   string
		status int
		err    error
		want   UpdateOutcome
	}{
		{name: "200 without an error is succeeded", status: 200, err: nil, want: UpdateSucceeded},
		{name: "200 with an error is unknown", status: 200, err: errFailed, want: UpdateUnknown},
		{name: "no response is unknown", status: 0, err: errFailed, want: UpdateUnknown},
		{name: "204 without an error is succeeded", status: 204, err: nil, want: UpdateSucceeded},
		{name: "204 with an error is unknown", status: 204, err: errFailed, want: UpdateUnknown},
		{name: "399 is unknown", status: 399, err: errFailed, want: UpdateUnknown},
		{name: "400 is rejected", status: 400, err: errFailed, want: UpdateRejected},
		{name: "499 is rejected", status: 499, err: errFailed, want: UpdateRejected},
		{name: "500 is unknown", status: 500, err: errFailed, want: UpdateUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := updateOutcomeOf(tc.status, tc.err); got != tc.want {
				t.Errorf("updateOutcomeOf(%d, %v) = %v, want %v", tc.status, tc.err, got, tc.want)
			}
		})
	}
}

//trace:checks FR-013
func TestDescribeUpdateOutcome(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		err         error
		want        UpdateOutcome
		wantUnknown bool
	}{
		{name: "403 is rejected without the unknown outcome", status: 403, err: ErrForbidden, want: UpdateRejected},
		{name: "500 is unknown and says so", status: 500, err: ErrUnexpectedResponse, want: UpdateUnknown, wantUnknown: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outcome, err := describeUpdateOutcome("update workflow permissions", mustReference(t, "owner/repo"), tc.status, tc.err)
			if outcome != tc.want || !errors.Is(err, tc.err) {
				t.Fatalf("describeUpdateOutcome() = %v, %v, want %v with %v", outcome, err, tc.want, tc.err)
			}
			if !strings.HasPrefix(err.Error(), "update workflow permissions owner/repo: ") {
				t.Errorf("error = %q, want it to start with the operation and the repository", err)
			}
			if got := strings.Contains(err.Error(), "update outcome unknown"); got != tc.wantUnknown {
				t.Errorf("error = %q, states unknown outcome = %v, want %v", err, got, tc.wantUnknown)
			}
		})
	}

	t.Run("success returns no error", func(t *testing.T) {
		outcome, err := describeUpdateOutcome("update workflow permissions", mustReference(t, "owner/repo"), 204, nil)
		if outcome != UpdateSucceeded || err != nil {
			t.Errorf("describeUpdateOutcome() = %v, %v, want %v without error", outcome, err, UpdateSucceeded)
		}
	})
}
