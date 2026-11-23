package github

import (
	"reflect"
	"testing"

	"github.com/tmknom/reco/internal/githubtest"
	"github.com/tmknom/reco/internal/setting"
)

// TestObserveRepositoryForEnvironments は、読む範囲が個別パターンと変数の両方の読み取りに効き、範囲の外の Environment を読まないまま残すつなぎ目を確かめる。
// どの名前を範囲に含めるかの規則は TestEnvironmentScope_Includes が確かめる。
//
//trace:checks FR-004 QR-002
func TestObserveRepositoryForEnvironments(t *testing.T) {
	selected := &githubtest.DeploymentBranchPolicyBody{CustomBranchPolicies: true}
	noPatterns := githubtest.OK(githubtest.DeploymentBranchPoliciesBody{}.JSON())
	noVariables := githubtest.OK(githubtest.VariablesBody{}.JSON())
	policiesPath := func(name string) string { return environmentsPath + "/" + name + "/deployment-branch-policies" }
	variablesPath := func(name string) string { return environmentsPath + "/" + name + "/variables" }
	list := func(environments ...githubtest.EnvironmentBody) githubtest.Response {
		return githubtest.OK(githubtest.EnvironmentsBody{TotalCount: len(environments), Environments: environments}.JSON())
	}

	cases := []struct {
		name  string
		names []string
		areas []setting.Area
		// routes は、送ってよい要求の応答。名指さない Environment の応答も置き、送れば要求の並びの比較で検出する。
		routes       map[string]githubtest.Response
		wantRequests []string
		// wantUnread は、個別パターンと変数のどちらも読まなかったはずの Environment の名前。
		wantUnread []string
	}{
		{
			name:  "deployment patterns are read only for the named selected environment of the same name",
			names: []string{"staging", "qa"},
			areas: []setting.Area{setting.AreaEnvironments},
			routes: map[string]githubtest.Response{
				environmentsPath: list(
					githubtest.EnvironmentBody{ID: 1, Name: "other", DeploymentBranchPolicy: selected},
					githubtest.EnvironmentBody{ID: 2, Name: "Staging", DeploymentBranchPolicy: selected},
					githubtest.EnvironmentBody{ID: 3, Name: "qa"},
				),
				policiesPath("other"):   noPatterns,
				policiesPath("Staging"): noPatterns,
			},
			wantRequests: []string{repositoryPath + "?", environmentsPath + firstPage, policiesPath("Staging") + firstPage},
			wantUnread:   []string{"other"},
		},
		{
			name:  "an unnamed environment named dot does not fail the observation",
			names: []string{"production"},
			areas: []setting.Area{setting.AreaEnvironments, setting.AreaEnvironmentVariables},
			routes: map[string]githubtest.Response{
				environmentsPath: list(
					githubtest.EnvironmentBody{ID: 1, Name: ".", DeploymentBranchPolicy: selected},
					githubtest.EnvironmentBody{ID: 2, Name: "production", DeploymentBranchPolicy: selected},
				),
				policiesPath("production"):  noPatterns,
				variablesPath("production"): noVariables,
			},
			wantRequests: []string{
				repositoryPath + "?", environmentsPath + firstPage,
				policiesPath("production") + firstPage, variablesPath("production") + variablesFirstPage,
			},
			wantUnread: []string{"."},
		},
		{
			name:  "variables are read only from the named environments after the patterns in the list order",
			names: []string{"staging", "production"},
			areas: []setting.Area{setting.AreaEnvironmentVariables},
			routes: map[string]githubtest.Response{
				environmentsPath: list(
					githubtest.EnvironmentBody{ID: 1, Name: "production", DeploymentBranchPolicy: selected},
					githubtest.EnvironmentBody{ID: 2, Name: "other"},
					githubtest.EnvironmentBody{ID: 3, Name: "staging"},
				),
				policiesPath("production"):  noPatterns,
				variablesPath("production"): noVariables,
				variablesPath("other"):      noVariables,
				variablesPath("staging"):    noVariables,
			},
			wantRequests: []string{
				repositoryPath + "?", environmentsPath + firstPage, policiesPath("production") + firstPage,
				variablesPath("production") + variablesFirstPage, variablesPath("staging") + variablesFirstPage,
			},
			wantUnread: []string{"other"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.routes[repositoryPath] = githubtest.OK(repositoryBody)
			routes := &escapedPathRoutes{t: t, routes: tc.routes}
			c := newTestClient(t, routes.ServeHTTP)
			names := make([]setting.EnvironmentName, len(tc.names))
			for i, name := range tc.names {
				names[i] = mustEnvironmentName(t, name)
			}

			observed, err := c.ObserveRepositoryForEnvironments(t.Context(), mustReference(t, "owner/repo"), names, tc.areas...)
			if err != nil {
				t.Fatalf("ObserveRepositoryForEnvironments() error = %v", err)
			}
			if got := routes.Requests(); !reflect.DeepEqual(got, tc.wantRequests) {
				t.Errorf("requests = %q, want %q", got, tc.wantRequests)
			}
			for _, e := range observed.Environments.Existing {
				for _, unread := range tc.wantUnread {
					if e.Name == unread && (e.DeploymentPatterns != nil || e.Variables != nil) {
						t.Errorf("environment %q = %+v, want neither patterns nor variables read", e.Name, e)
					}
				}
			}
		})
	}
}

//trace:checks FR-004
func TestEnvironmentScope_Includes(t *testing.T) {
	cases := []struct {
		name     string
		scope    environmentScope
		observed string
		want     bool
	}{
		{name: "all environments include any name", scope: allEnvironments(), observed: "café", want: true},
		{name: "a name that differs only in ASCII case is included", scope: namedEnvironments([]setting.EnvironmentName{mustEnvironmentName(t, "qa"), mustEnvironmentName(t, "production")}), observed: "Production", want: true},
		{name: "a different name is not included", scope: namedEnvironments([]setting.EnvironmentName{mustEnvironmentName(t, "production")}), observed: "staging"},
		{name: "a name whose identity cannot be determined is not included", scope: namedEnvironments([]setting.EnvironmentName{mustEnvironmentName(t, "cafe")}), observed: "café"},
		{name: "no named environment includes no name", scope: namedEnvironments(nil), observed: "production"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.scope.includes(tc.observed); got != tc.want {
				t.Errorf("includes(%q) = %v, want %v", tc.observed, got, tc.want)
			}
		})
	}
}
