package github

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/tmknom/reco/internal/githubtest"
	"github.com/tmknom/reco/internal/setting"
)

//trace:checks FR-004 QR-002
func TestDependabotAlertsEnabled(t *testing.T) {
	notFound := fmt.Errorf("%w: status 404", ErrNotFound)
	forbidden := fmt.Errorf("%w: status 403", ErrForbidden)
	successes := []struct {
		name  string
		err   error
		admin bool
		want  bool
	}{
		{name: "success is enabled without admin access", err: nil, admin: false, want: true},
		{name: "not found with admin access is disabled", err: notFound, admin: true, want: false},
	}
	for _, tc := range successes {
		t.Run(tc.name, func(t *testing.T) {
			got, err := dependabotAlertsEnabled(tc.err, tc.admin)
			if err != nil || got != tc.want {
				t.Errorf("dependabotAlertsEnabled() = (%t, %v), want (%t, nil)", got, err, tc.want)
			}
		})
	}

	failures := []struct {
		name  string
		err   error
		admin bool
	}{
		{name: "not found without admin access stays not found", err: notFound, admin: false},
		{name: "forbidden with admin access stays forbidden", err: forbidden, admin: true},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			_, err := dependabotAlertsEnabled(tc.err, tc.admin)
			if !errors.Is(err, tc.err) {
				t.Errorf("dependabotAlertsEnabled() error = %v, want %v", err, tc.err)
			}
		})
	}
}

//trace:checks FR-004 QR-002
func TestDecodeAutomatedSecurityFixes(t *testing.T) {
	successes := []struct {
		name string
		body string
		want setting.DependabotSecurityUpdatesState
	}{
		{name: "enabled and not paused is enabled", body: `{"enabled": true, "paused": false}`, want: setting.DependabotSecurityUpdatesEnabled},
		{name: "enabled and paused is paused", body: `{"enabled": true, "paused": true}`, want: setting.DependabotSecurityUpdatesPaused},
		{name: "disabled and not paused is disabled", body: `{"enabled": false, "paused": false}`, want: setting.DependabotSecurityUpdatesDisabled},
	}
	for _, tc := range successes {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeAutomatedSecurityFixes([]byte(tc.body))
			if err != nil || got != tc.want {
				t.Errorf("decodeAutomatedSecurityFixes() = (%d, %v), want (%d, nil)", got, err, tc.want)
			}
		})
	}

	failures := []struct {
		name        string
		body        string
		errContains string
	}{
		{name: "disabled and paused is unexpected response", body: `{"enabled": false, "paused": true}`, errContains: "enabled, paused"},
		{name: "missing enabled is unexpected response", body: `{"paused": false}`, errContains: "enabled"},
		{name: "missing paused is unexpected response", body: `{"enabled": true}`, errContains: "paused"},
		{name: "non-JSON body is unexpected response", body: `<html>`},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeAutomatedSecurityFixes([]byte(tc.body))
			if !errors.Is(err, ErrUnexpectedResponse) {
				t.Fatalf("decodeAutomatedSecurityFixes() error = %v, want %v", err, ErrUnexpectedResponse)
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Errorf("decodeAutomatedSecurityFixes() error = %q, want it to contain %q", err, tc.errContains)
			}
		})
	}
}

const (
	vulnerabilityAlertsPath    = "/repos/owner/repo/vulnerability-alerts"
	automatedSecurityFixesPath = "/repos/owner/repo/automated-security-fixes"
)

//trace:checks FR-004 QR-002
func TestObserveRepositoryDependabot(t *testing.T) {
	noAdminRepositoryBody := githubtest.RepositoryBody{ID: 1296269, AllowAutoMerge: true}.JSON()
	enabledFixes := githubtest.OK(githubtest.AutomatedSecurityFixesBody{Enabled: true}.JSON())

	t.Run("dependabot is read after the environments with alerts first", func(t *testing.T) {
		s := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
			"GET " + repositoryPath:             githubtest.OK(repositoryBody),
			"GET " + environmentsPath:           githubtest.OK(githubtest.EnvironmentsBody{}.JSON()),
			"GET " + vulnerabilityAlertsPath:    {Status: http.StatusNoContent},
			"GET " + automatedSecurityFixesPath: enabledFixes,
		}))
		c := New(testToken, WithBaseURL(s.URL()))

		observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaDependabot, setting.AreaEnvironments)
		if err != nil {
			t.Fatalf("ObserveRepository() error = %v", err)
		}
		want := &Dependabot{Alerts: true, SecurityUpdates: setting.DependabotSecurityUpdatesEnabled}
		if !reflect.DeepEqual(observed.Dependabot, want) {
			t.Errorf("ObserveRepository().Dependabot = %+v, want %+v", observed.Dependabot, want)
		}
		wantLines := []string{"GET " + repositoryPath, "GET " + environmentsPath, "GET " + vulnerabilityAlertsPath, "GET " + automatedSecurityFixesPath}
		if got := s.RequestLines(); !reflect.DeepEqual(got, wantLines) {
			t.Errorf("requests = %q, want %q", got, wantLines)
		}
		for i, r := range s.Requests() {
			if r.Body != nil {
				t.Errorf("request %d has body %v, want no body", i, r.Body)
			}
		}
	})

	t.Run("alerts not found with admin access is disabled and the security updates are still read", func(t *testing.T) {
		s := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
			"GET " + repositoryPath:             githubtest.OK(repositoryBody),
			"GET " + vulnerabilityAlertsPath:    {Status: http.StatusNotFound, Body: `{"message": "Vulnerability alerts are disabled."}`},
			"GET " + automatedSecurityFixesPath: githubtest.OK(githubtest.AutomatedSecurityFixesBody{}.JSON()),
		}))
		c := New(testToken, WithBaseURL(s.URL()))

		observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaDependabot)
		if err != nil {
			t.Fatalf("ObserveRepository() error = %v", err)
		}
		want := &Dependabot{Alerts: false, SecurityUpdates: setting.DependabotSecurityUpdatesDisabled}
		if !reflect.DeepEqual(observed.Dependabot, want) {
			t.Errorf("ObserveRepository().Dependabot = %+v, want %+v", observed.Dependabot, want)
		}
	})

	failures := []struct {
		name           string
		repositoryBody string
		routes         map[string]githubtest.Response
		want           error
		wantFetch      string
		wantLines      []string
	}{
		{
			name:           "alerts not found without admin access is not found and does not read the security updates",
			repositoryBody: noAdminRepositoryBody,
			routes: map[string]githubtest.Response{
				"GET " + vulnerabilityAlertsPath: {Status: http.StatusNotFound},
			},
			want:      ErrNotFound,
			wantFetch: "fetch dependabot alerts owner/repo",
			wantLines: []string{"GET " + repositoryPath, "GET " + vulnerabilityAlertsPath},
		},
		{
			name:           "security updates not found with admin access is not found",
			repositoryBody: repositoryBody,
			routes: map[string]githubtest.Response{
				"GET " + vulnerabilityAlertsPath:    {Status: http.StatusNoContent},
				"GET " + automatedSecurityFixesPath: {Status: http.StatusNotFound},
			},
			want:      ErrNotFound,
			wantFetch: "fetch dependabot security updates owner/repo",
			wantLines: []string{"GET " + repositoryPath, "GET " + vulnerabilityAlertsPath, "GET " + automatedSecurityFixesPath},
		},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			tc.routes["GET "+repositoryPath] = githubtest.OK(tc.repositoryBody)
			s := githubtest.NewServer(t, githubtest.Routes(t, tc.routes))
			c := New(testToken, WithBaseURL(s.URL()))

			observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaDependabot)
			if !errors.Is(err, tc.want) || !strings.Contains(err.Error(), tc.wantFetch) {
				t.Fatalf("ObserveRepository() error = %v, want %v from %s", err, tc.want, tc.wantFetch)
			}
			if observed != (ObservationResult{}) {
				t.Errorf("ObserveRepository() = %+v, want the zero value", observed)
			}
			if got := s.RequestLines(); !reflect.DeepEqual(got, tc.wantLines) {
				t.Errorf("requests = %q, want %q", got, tc.wantLines)
			}
		})
	}

	t.Run("alerts redirect is not followed", func(t *testing.T) {
		var redirected atomic.Int32
		redirectTarget := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			redirected.Add(1)
		}))
		t.Cleanup(redirectTarget.Close)
		location := redirectTarget.URL + "/repositories/1296269/vulnerability-alerts"
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == vulnerabilityAlertsPath {
				http.Redirect(w, r, location, http.StatusMovedPermanently)
				return
			}
			_, _ = w.Write([]byte(repositoryBody))
		})

		_, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaDependabot)
		if !errors.Is(err, ErrRedirect) {
			t.Fatalf("ObserveRepository() error = %v, want %v", err, ErrRedirect)
		}
		for _, secret := range []string{testToken, location} {
			if strings.Contains(err.Error(), secret) {
				t.Errorf("error = %q, must not contain %q", err, secret)
			}
		}
		if n := redirected.Load(); n != 0 {
			t.Errorf("redirect target received %d requests, want 0", n)
		}
	})
}
