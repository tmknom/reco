package github

import (
	"errors"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/githubtest"
	"github.com/tmknom/reco/internal/setting"
)

// 成功のケースは、nil と長さ 0 の並びを区別しないため、key の並びを slices.Equal で比べる。
//
//trace:checks FR-004 QR-002
func TestDecodeOIDCSubject(t *testing.T) {
	successes := []struct {
		name string
		body string
		want setting.OIDCSubjectCustomization
	}{
		{
			name: "default customization has no claim keys",
			body: `{"use_default": true, "use_immutable_subject": false, "sub_claim_prefix": "repo:owner/repo"}`,
			want: setting.OIDCSubjectCustomization{UseDefault: true},
		},
		{
			name: "claim keys are read in the returned order",
			body: `{"use_default": false, "include_claim_keys": ["repo", "context", "job_workflow_ref"], "use_immutable_subject": false}`,
			want: setting.OIDCSubjectCustomization{IncludeClaimKeys: []string{"repo", "context", "job_workflow_ref"}},
		},
		{
			name: "missing claim keys without the default is the empty sequence",
			body: `{"use_default": false, "use_immutable_subject": false}`,
			want: setting.OIDCSubjectCustomization{},
		},
		{
			name: "null claim keys without the default is the empty sequence",
			body: `{"use_default": false, "include_claim_keys": null, "use_immutable_subject": false}`,
			want: setting.OIDCSubjectCustomization{},
		},
		{
			name: "claim keys returned with the default are not held",
			body: `{"use_default": true, "include_claim_keys": ["repo"], "use_immutable_subject": false}`,
			want: setting.OIDCSubjectCustomization{UseDefault: true},
		},
		{
			name: "immutable subject is read as observed",
			body: `{"use_default": true, "use_immutable_subject": true}`,
			want: setting.OIDCSubjectCustomization{UseDefault: true, UseImmutableSubject: true},
		},
	}
	for _, tc := range successes {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeOIDCSubject([]byte(tc.body))
			if err != nil {
				t.Fatalf("decodeOIDCSubject() error = %v", err)
			}
			if got.UseDefault != tc.want.UseDefault || got.UseImmutableSubject != tc.want.UseImmutableSubject || !slices.Equal(got.IncludeClaimKeys, tc.want.IncludeClaimKeys) {
				t.Errorf("decodeOIDCSubject() = %+v, want %+v", got, tc.want)
			}
		})
	}

	failures := []struct {
		name        string
		body        string
		errContains string
	}{
		{name: "missing use_default is unexpected response", body: `{"use_immutable_subject": false}`, errContains: "missing fields: use_default"},
		{name: "null use_immutable_subject is unexpected response", body: `{"use_default": true, "use_immutable_subject": null}`, errContains: "missing fields: use_immutable_subject"},
		{name: "every missing field is named", body: `{}`, errContains: "missing fields: use_default, use_immutable_subject"},
		{name: "non-string claim key is unexpected response", body: `{"use_default": false, "include_claim_keys": ["repo", 1], "use_immutable_subject": false}`},
		{name: "null claim key is unexpected response", body: `{"use_default": false, "include_claim_keys": ["repo", null], "use_immutable_subject": false}`, errContains: "include_claim_keys"},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeOIDCSubject([]byte(tc.body))
			if !errors.Is(err, ErrUnexpectedResponse) {
				t.Fatalf("decodeOIDCSubject() error = %v, want %v", err, ErrUnexpectedResponse)
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Errorf("decodeOIDCSubject() error = %q, want it to contain %q", err, tc.errContains)
			}
		})
	}
}

//trace:checks FR-004 QR-002
func TestObserveRepositoryOIDCSubject(t *testing.T) {
	cases := []struct {
		name           string
		repositoryBody string
	}{
		{name: "public repository reads the oidc subject", repositoryBody: repositoryBody},
		{name: "private repository reads the oidc subject", repositoryBody: privateRepositoryBody},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
				"GET " + repositoryPath:  githubtest.OK(tc.repositoryBody),
				"GET " + oidcSubjectPath: githubtest.OK(githubtest.OIDCSubjectBody{IncludeClaimKeys: []string{"repo", "context"}, UseImmutableSubject: true}.JSON()),
			}))
			c := New(testToken, WithBaseURL(s.URL()))

			observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaOIDCSubject)
			if err != nil {
				t.Fatalf("ObserveRepository() error = %v", err)
			}
			want := &OIDCSubject{Customization: setting.OIDCSubjectCustomization{IncludeClaimKeys: []string{"repo", "context"}, UseImmutableSubject: true}}
			if !reflect.DeepEqual(observed.OIDCSubject, want) {
				t.Errorf("ObserveRepository().OIDCSubject = %+v, want %+v", observed.OIDCSubject, want)
			}
			wantLines := []string{"GET " + repositoryPath, "GET " + oidcSubjectPath}
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
		{name: "missing use_default is unexpected response", response: githubtest.OK(`{"use_immutable_subject": false}`), want: ErrUnexpectedResponse},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			s := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
				"GET " + repositoryPath:  githubtest.OK(repositoryBody),
				"GET " + oidcSubjectPath: tc.response,
			}))
			c := New(testToken, WithBaseURL(s.URL()))

			observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaOIDCSubject)
			const wantFetch = "fetch oidc subject owner/repo"
			if !errors.Is(err, tc.want) || !strings.Contains(err.Error(), wantFetch) {
				t.Fatalf("ObserveRepository() error = %v, want %v from %s", err, tc.want, wantFetch)
			}
			if !reflect.DeepEqual(observed, ObservationResult{}) {
				t.Errorf("ObserveRepository() = %+v, want the zero value", observed)
			}
		})
	}
}

// TestObserveRepositoryOIDCSubjectOrder は、OIDC subject の領域が、渡した並びによらず、保持期間のあと、Repository Variables の前に読まれることを確かめる。
//
//trace:checks FR-004 QR-002
func TestObserveRepositoryOIDCSubjectOrder(t *testing.T) {
	s := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
		"GET " + repositoryPath:          githubtest.OK(repositoryBody),
		"GET " + retentionPath:           githubtest.OK(githubtest.RetentionBody{Days: 90, MaximumAllowedDays: 90}.JSON()),
		"GET " + oidcSubjectPath:         githubtest.OK(githubtest.OIDCSubjectBody{UseDefault: true}.JSON()),
		"GET " + repositoryVariablesPath: githubtest.OK(githubtest.VariablesBody{}.JSON()),
	}))
	c := New(testToken, WithBaseURL(s.URL()))

	_, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"),
		setting.AreaRepositoryVariables, setting.AreaOIDCSubject, setting.AreaRetention)
	if err != nil {
		t.Fatalf("ObserveRepository() error = %v", err)
	}
	wantLines := []string{"GET " + repositoryPath, "GET " + retentionPath, "GET " + oidcSubjectPath, "GET " + repositoryVariablesPath}
	if got := s.RequestLines(); !reflect.DeepEqual(got, wantLines) {
		t.Errorf("requests = %q, want %q", got, wantLines)
	}
}
