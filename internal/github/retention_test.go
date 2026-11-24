package github

import (
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/githubtest"
	"github.com/tmknom/reco/internal/setting"
)

//trace:checks FR-004 QR-002
func TestDecodeRetention(t *testing.T) {
	successes := []struct {
		name string
		body string
		want Retention
	}{
		{name: "days and maximum_allowed_days are read in place", body: `{"days": 30, "maximum_allowed_days": 400}`, want: Retention{Days: 30, MaximumAllowedDays: 400}},
		{name: "days over the maximum is read as observed", body: `{"days": 90, "maximum_allowed_days": 30}`, want: Retention{Days: 90, MaximumAllowedDays: 30}},
	}
	for _, tc := range successes {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeRetention([]byte(tc.body))
			if err != nil || got != tc.want {
				t.Errorf("decodeRetention() = (%+v, %v), want (%+v, nil)", got, err, tc.want)
			}
		})
	}

	failures := []struct {
		name        string
		body        string
		errContains string
	}{
		{name: "missing days is unexpected response", body: `{"maximum_allowed_days": 400}`, errContains: "missing fields: days"},
		{name: "null maximum_allowed_days is unexpected response", body: `{"days": 90, "maximum_allowed_days": null}`, errContains: "missing fields: maximum_allowed_days"},
		{name: "every missing field is named", body: `{}`, errContains: "missing fields: days, maximum_allowed_days"},
		{name: "fractional days is unexpected response", body: `{"days": 30.5, "maximum_allowed_days": 400}`},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeRetention([]byte(tc.body))
			if !errors.Is(err, ErrUnexpectedResponse) {
				t.Fatalf("decodeRetention() error = %v, want %v", err, ErrUnexpectedResponse)
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Errorf("decodeRetention() error = %q, want it to contain %q", err, tc.errContains)
			}
		})
	}
}

//trace:checks FR-004 QR-002
func TestObserveRepositoryRetention(t *testing.T) {
	cases := []struct {
		name           string
		repositoryBody string
	}{
		{name: "public repository reads the retention", repositoryBody: repositoryBody},
		{name: "private repository reads the retention", repositoryBody: privateRepositoryBody},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
				"GET " + repositoryPath: githubtest.OK(tc.repositoryBody),
				"GET " + retentionPath:  githubtest.OK(githubtest.RetentionBody{Days: 30, MaximumAllowedDays: 400}.JSON()),
			}))
			c := New(testToken, WithBaseURL(s.URL()))

			observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaRetention)
			if err != nil {
				t.Fatalf("ObserveRepository() error = %v", err)
			}
			want := &Retention{Days: 30, MaximumAllowedDays: 400}
			if !reflect.DeepEqual(observed.Retention, want) {
				t.Errorf("ObserveRepository().Retention = %+v, want %+v", observed.Retention, want)
			}
			wantLines := []string{"GET " + repositoryPath, "GET " + retentionPath}
			if got := s.RequestLines(); !reflect.DeepEqual(got, wantLines) {
				t.Errorf("requests = %q, want %q", got, wantLines)
			}
		})
	}

	failures := []struct {
		name     string
		response githubtest.Response
		want     error
	}{
		{name: "301 is redirect", response: githubtest.Response{Status: http.StatusMovedPermanently}, want: ErrRedirect},
		{name: "missing days is unexpected response", response: githubtest.OK(`{"maximum_allowed_days": 400}`), want: ErrUnexpectedResponse},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			s := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
				"GET " + repositoryPath: githubtest.OK(repositoryBody),
				"GET " + retentionPath:  tc.response,
			}))
			c := New(testToken, WithBaseURL(s.URL()))

			observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaRetention)
			const wantFetch = "fetch retention owner/repo"
			if !errors.Is(err, tc.want) || !strings.Contains(err.Error(), wantFetch) {
				t.Fatalf("ObserveRepository() error = %v, want %v from %s", err, tc.want, wantFetch)
			}
			if observed != (ObservationResult{}) {
				t.Errorf("ObserveRepository() = %+v, want the zero value", observed)
			}
		})
	}
}
