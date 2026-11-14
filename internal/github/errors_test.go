package github

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

//trace:checks FR-004
func TestIsRateLimited(t *testing.T) {
	cases := []struct {
		name   string
		header http.Header
		want   bool
	}{
		{name: "retry-after present is rate limited", header: http.Header{"Retry-After": {"60"}}, want: true},
		{name: "remaining zero is rate limited", header: http.Header{"X-Ratelimit-Remaining": {"0"}}, want: true},
		{name: "remaining non-zero is not rate limited", header: http.Header{"X-Ratelimit-Remaining": {"42"}}, want: false},
		{name: "empty retry-after is not rate limited", header: http.Header{"Retry-After": {""}}, want: false},
		{name: "no rate limit headers is not rate limited", header: http.Header{}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isRateLimited(tc.header); got != tc.want {
				t.Errorf("isRateLimited() = %v, want %v", got, tc.want)
			}
		})
	}
}

//trace:checks FR-003
//trace:checks FR-004
//trace:checks FR-013
func TestClassifyFailure(t *testing.T) {
	const renameHint = "the repository may have been renamed or transferred; specify its current name"
	cases := []struct {
		name     string
		status   int
		header   http.Header
		want     error
		wantHint bool
	}{
		{name: "300 is redirect suggesting the current name", status: 300, want: ErrRedirect, wantHint: true},
		{name: "399 is redirect suggesting the current name", status: 399, want: ErrRedirect, wantHint: true},
		{name: "204 is unexpected response", status: 204, want: ErrUnexpectedResponse},
		{name: "400 is unexpected response", status: 400, want: ErrUnexpectedResponse},
		{name: "401 is unauthorized", status: 401, want: ErrUnauthorized},
		{name: "403 without rate limit headers is forbidden", status: 403, header: http.Header{}, want: ErrForbidden},
		{name: "403 with retry-after is rate limited", status: 403, header: http.Header{"Retry-After": {"60"}}, want: ErrRateLimited},
		{name: "404 is not found", status: 404, want: ErrNotFound},
		{name: "409 is conflict", status: 409, want: ErrConflict},
		{name: "422 is unprocessable", status: 422, want: ErrUnprocessable},
		{name: "429 without rate limit headers is rate limited", status: 429, header: http.Header{}, want: ErrRateLimited},
	}
	kinds := []error{ErrRedirect, ErrUnauthorized, ErrForbidden, ErrRateLimited, ErrNotFound, ErrConflict, ErrUnprocessable, ErrUnexpectedResponse, ErrCommunicationFailure}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := classifyFailure(tc.status, tc.header)
			for _, kind := range kinds {
				if got, want := errors.Is(err, kind), errors.Is(tc.want, kind); got != want {
					t.Errorf("errors.Is(%v, %v) = %v, want %v", err, kind, got, want)
				}
			}
			if want := fmt.Sprintf("HTTP %d", tc.status); !strings.Contains(err.Error(), want) {
				t.Errorf("classifyFailure() error = %q, want it to contain %q", err, want)
			}
			if got := strings.Contains(err.Error(), renameHint); got != tc.wantHint {
				t.Errorf("classifyFailure() error = %q, contains rename hint = %v, want %v", err, got, tc.wantHint)
			}
		})
	}
}
