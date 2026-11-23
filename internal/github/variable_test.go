package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/githubtest"
	"github.com/tmknom/reco/internal/setting"
)

const (
	repositoryVariablesPath = "/repos/owner/repo/actions/variables"
	variablesFirstPage      = "?per_page=30&page=1"
)

//trace:checks FR-004 QR-002
func TestDecodeVariable(t *testing.T) {
	successes := []struct {
		name string
		data string
		want Variable
	}{
		{name: "name and value become the variable", data: `{"name": "GO_VERSION", "value": "1.27"}`, want: Variable{Name: "GO_VERSION", Value: "1.27"}},
		{name: "empty value is an existing variable with the empty value", data: `{"name": "EMPTY", "value": ""}`, want: Variable{Name: "EMPTY", Value: ""}},
		{
			name: "value keeps surrounding spaces, CRLF, tab, NUL, and non-ASCII spaces as sent",
			data: `{"name": "RAW", "value": " a\r\n\tb\u0000 \u3000 "}`,
			want: Variable{Name: "RAW", Value: " a\r\n\tb\x00 \u3000 "},
		},
		{
			name: "timestamps and unknown fields are not required",
			data: `{"name": "X", "value": "y", "unknown": {"nested": true}}`,
			want: Variable{Name: "X", Value: "y"},
		},
	}
	for _, tc := range successes {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeVariable(json.RawMessage(tc.data))
			if err != nil {
				t.Fatalf("decodeVariable() error = %v", err)
			}
			if got != tc.want {
				t.Errorf("decodeVariable() = %+v, want %+v", got, tc.want)
			}
		})
	}

	// 目印の値を文言に含めないことで、応答本文の値をエラーに入れないことを確かめる
	const marker = "SECRET_LIKE_MARKER"
	failures := []struct {
		name        string
		data        string
		errContains string
	}{
		{name: "missing name is unexpected response", data: `{"value": "` + marker + `"}`, errContains: "missing fields: name"},
		{name: "null value is unexpected response instead of the empty value", data: `{"name": "` + marker + `", "value": null}`, errContains: "missing fields: value"},
		{name: "null element names both", data: `null`, errContains: "missing fields: name, value"},
		{name: "empty name is unexpected response", data: `{"name": "", "value": "` + marker + `"}`, errContains: "unsupported value: name"},
		{name: "numeric value is unexpected response", data: `{"name": "` + marker + `", "value": 1.27}`},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeVariable(json.RawMessage(tc.data))
			if !errors.Is(err, ErrUnexpectedResponse) {
				t.Fatalf("decodeVariable() error = %v, want %v", err, ErrUnexpectedResponse)
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Errorf("decodeVariable() error = %q, want it to contain %q", err, tc.errContains)
			}
			if strings.Contains(err.Error(), marker) {
				t.Errorf("decodeVariable() error = %q, want it not to contain the response value", err)
			}
		})
	}
}

//trace:checks FR-004 QR-002
func TestObserveRepositoryVariables(t *testing.T) {
	t.Run("an observation without the repository variables area sends no list request and has no variables", func(t *testing.T) {
		routes := &escapedPathRoutes{t: t, routes: map[string]githubtest.Response{
			repositoryPath:   githubtest.OK(repositoryBody),
			environmentsPath: githubtest.OK(githubtest.EnvironmentsBody{}.JSON()),
		}}
		c := newTestClient(t, routes.ServeHTTP)

		observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaEnvironments)
		if err != nil {
			t.Fatalf("ObserveRepository() error = %v", err)
		}
		if observed.RepositoryVariables != nil {
			t.Errorf("ObserveRepository().RepositoryVariables = %+v, want nil", observed.RepositoryVariables)
		}
		want := []string{repositoryPath + "?", environmentsPath + firstPage}
		if got := routes.Requests(); !reflect.DeepEqual(got, want) {
			t.Errorf("requests = %q, want %q", got, want)
		}
	})

	t.Run("the list is read after actions access and before environments with 30 per page whatever the order of the areas", func(t *testing.T) {
		routes := &escapedPathRoutes{t: t, routes: map[string]githubtest.Response{
			repositoryPath:          githubtest.OK(githubtest.RepositoryBody{ID: 1, Private: true}.JSON()),
			actionsAccessPath:       githubtest.OK(actionsAccessBody),
			repositoryVariablesPath: githubtest.OK(githubtest.VariablesBody{}.JSON()),
			environmentsPath:        githubtest.OK(githubtest.EnvironmentsBody{}.JSON()),
		}}
		c := newTestClient(t, routes.ServeHTTP)

		_, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"),
			setting.AreaEnvironments, setting.AreaRepositoryVariables, setting.AreaActionsAccess)
		if err != nil {
			t.Fatalf("ObserveRepository() error = %v", err)
		}
		want := []string{repositoryPath + "?", actionsAccessPath + "?", repositoryVariablesPath + variablesFirstPage, environmentsPath + firstPage}
		if got := routes.Requests(); !reflect.DeepEqual(got, want) {
			t.Errorf("requests = %q, want %q", got, want)
		}
	})

	t.Run("no variables is an empty list rather than an unread area", func(t *testing.T) {
		observed := observeRepositoryVariablesList(t, githubtest.OK(githubtest.VariablesBody{}.JSON()))
		if observed.RepositoryVariables == nil || len(observed.RepositoryVariables.Existing) != 0 {
			t.Errorf("ObserveRepository().RepositoryVariables = %+v, want an empty list", observed.RepositoryVariables)
		}
	})

	t.Run("two pages are joined in the order they arrived without sorting by name", func(t *testing.T) {
		first := make([]githubtest.VariableBody, 30)
		for i := range first {
			// 名前の降順で返し、並べ替えれば順序が変わるようにする
			first[i] = githubtest.VariableBody{Name: fmt.Sprintf("V%02d", 40-i), Value: fmt.Sprint(i)}
		}
		last := githubtest.VariableBody{Name: "A_LAST", Value: "last"}
		var requests []string
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			requests = append(requests, r.URL.EscapedPath()+"?"+r.URL.RawQuery)
			switch r.URL.EscapedPath() + "?" + r.URL.RawQuery {
			case repositoryPath + "?":
				_, _ = w.Write([]byte(repositoryBody))
			case repositoryVariablesPath + variablesFirstPage:
				_, _ = w.Write([]byte(githubtest.VariablesBody{TotalCount: 31, Variables: first}.JSON()))
			case repositoryVariablesPath + "?per_page=30&page=2":
				_, _ = w.Write([]byte(githubtest.VariablesBody{TotalCount: 31, Variables: []githubtest.VariableBody{last}}.JSON()))
			default:
				t.Errorf("unexpected request %s", r.URL)
				w.WriteHeader(http.StatusInternalServerError)
			}
		})

		observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaRepositoryVariables)
		if err != nil {
			t.Fatalf("ObserveRepository() error = %v", err)
		}
		want := make([]Variable, 0, 31)
		for _, v := range first {
			want = append(want, Variable{Name: v.Name, Value: v.Value})
		}
		want = append(want, Variable{Name: last.Name, Value: last.Value})
		if !reflect.DeepEqual(observed.RepositoryVariables, &Variables{Existing: want}) {
			t.Errorf("ObserveRepository().RepositoryVariables = %+v, want %+v", observed.RepositoryVariables, want)
		}
		if len(requests) != 3 {
			t.Errorf("requests = %q, want the repository and two pages", requests)
		}
	})

	t.Run("names that only Unicode folding makes equal are different variables", func(t *testing.T) {
		observed := observeRepositoryVariablesList(t, githubtest.OK(githubtest.VariablesBody{TotalCount: 2, Variables: []githubtest.VariableBody{
			{Name: "\u017f", Value: "1"}, {Name: "S", Value: "2"},
		}}.JSON()))
		if observed.RepositoryVariables == nil || len(observed.RepositoryVariables.Existing) != 2 {
			t.Errorf("ObserveRepository().RepositoryVariables = %+v, want the two variables", observed.RepositoryVariables)
		}
	})

	t.Run("a page over 1 MiB within the variables limit is read", func(t *testing.T) {
		variables := make([]githubtest.VariableBody, 30)
		for i := range variables {
			variables[i] = githubtest.VariableBody{Name: fmt.Sprintf("LARGE_%02d", i), Value: strings.Repeat("a", 40_000)}
		}
		body := githubtest.VariablesBody{TotalCount: 30, Variables: variables}.JSON()
		if len(body) <= maxResponseBytes {
			t.Fatalf("test body is %d bytes, want more than %d", len(body), maxResponseBytes)
		}
		observed := observeRepositoryVariablesList(t, githubtest.OK(body))
		if observed.RepositoryVariables == nil || len(observed.RepositoryVariables.Existing) != 30 {
			t.Errorf("ObserveRepository().RepositoryVariables has %v, want 30 variables", observed.RepositoryVariables)
		}
	})

	t.Run("a list that never reaches the total stops after the pages that hold GitHub's maximum of 500 variables", func(t *testing.T) {
		var pages int
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.EscapedPath() == repositoryPath {
				_, _ = w.Write([]byte(repositoryBody))
				return
			}
			pages++
			variables := make([]githubtest.VariableBody, 30)
			for i := range variables {
				variables[i] = githubtest.VariableBody{Name: fmt.Sprintf("P%s_%02d", r.URL.Query().Get("page"), i), Value: "v"}
			}
			_, _ = w.Write([]byte(githubtest.VariablesBody{TotalCount: 3000, Variables: variables}.JSON()))
		})

		_, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaRepositoryVariables)
		if !errors.Is(err, ErrUnexpectedResponse) {
			t.Fatalf("ObserveRepository() error = %v, want %v", err, ErrUnexpectedResponse)
		}
		if pages != 17 {
			t.Errorf("ObserveRepository() read %d pages, want 17", pages)
		}
	})

	failures := []struct {
		name        string
		response    githubtest.Response
		wantErr     error
		errContains []string
	}{
		{
			name:     "a page over the variables limit is unexpected response",
			response: githubtest.OK(`{"total_count": 0, "variables": []` + strings.Repeat(" ", maxVariablesPageBytes) + `}`),
			wantErr:  ErrUnexpectedResponse,
		},
		{
			name: "names that differ only in ASCII case are unexpected response",
			response: githubtest.OK(githubtest.VariablesBody{TotalCount: 2, Variables: []githubtest.VariableBody{
				{Name: "GO_VERSION", Value: "1"}, {Name: "go_version", Value: "2"},
			}}.JSON()),
			wantErr:     ErrUnexpectedResponse,
			errContains: []string{"duplicate variable name"},
		},
		{
			name:     "an element without value is unexpected response",
			response: githubtest.OK(`{"total_count": 1, "variables": [{"name": "A"}]}`),
			wantErr:  ErrUnexpectedResponse,
		},
		{
			name:        "404 is not found rather than no variables and names the repository variables",
			response:    githubtest.Response{Status: http.StatusNotFound},
			wantErr:     ErrNotFound,
			errContains: []string{"repository variables owner/repo"},
		},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			routes := &escapedPathRoutes{t: t, routes: map[string]githubtest.Response{
				repositoryPath:          githubtest.OK(repositoryBody),
				repositoryVariablesPath: tc.response,
			}}
			c := newTestClient(t, routes.ServeHTTP)

			_, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaRepositoryVariables)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ObserveRepository() error = %v, want %v", err, tc.wantErr)
			}
			for _, s := range tc.errContains {
				if !strings.Contains(err.Error(), s) {
					t.Errorf("ObserveRepository() error = %q, want it to contain %q", err, s)
				}
			}
		})
	}
}

//trace:checks FR-004 QR-002
func TestObserveEnvironmentVariables(t *testing.T) {
	t.Run("every environment whatever its mode gets its own variables in the list order", func(t *testing.T) {
		environments := []Environment{
			{ID: 1, Name: "open", DeploymentBranchPolicyMode: setting.DeploymentBranchPolicyAll},
			{ID: 2, Name: "main-only", DeploymentBranchPolicyMode: setting.DeploymentBranchPolicySelected},
			{ID: 3, Name: "Prod/Eu 100%", DeploymentBranchPolicyMode: setting.DeploymentBranchPolicyProtectedBranches},
		}
		routes := &escapedPathRoutes{t: t, routes: map[string]githubtest.Response{
			environmentsPath + "/open/variables":      githubtest.OK(githubtest.VariablesBody{TotalCount: 1, Variables: []githubtest.VariableBody{{Name: "REGION", Value: "us"}}}.JSON()),
			environmentsPath + "/main-only/variables": githubtest.OK(githubtest.VariablesBody{}.JSON()),
			environmentsPath + "/Prod%2FEu%20100%25/variables": githubtest.OK(githubtest.VariablesBody{TotalCount: 2, Variables: []githubtest.VariableBody{
				{Name: "REGION", Value: "eu"}, {Name: "EMPTY", Value: ""},
			}}.JSON()),
		}}
		c := newTestClient(t, routes.ServeHTTP)

		if err := c.observeEnvironmentVariables(t.Context(), mustReference(t, "owner/repo"), environments, allResources()); err != nil {
			t.Fatalf("observeEnvironmentVariables() error = %v", err)
		}
		wantRequests := []string{
			environmentsPath + "/open/variables" + variablesFirstPage,
			environmentsPath + "/main-only/variables" + variablesFirstPage,
			environmentsPath + "/Prod%2FEu%20100%25/variables" + variablesFirstPage,
		}
		if got := routes.Requests(); !reflect.DeepEqual(got, wantRequests) {
			t.Errorf("requests = %q, want %q", got, wantRequests)
		}
		want := []*Variables{
			{Existing: []Variable{{Name: "REGION", Value: "us"}}},
			{Existing: []Variable{}},
			{Existing: []Variable{{Name: "REGION", Value: "eu"}, {Name: "EMPTY", Value: ""}}},
		}
		for i, e := range environments {
			if !reflect.DeepEqual(e.Variables, want[i]) {
				t.Errorf("environments[%d].Variables = %+v, want %+v", i, e.Variables, want[i])
			}
		}
	})

	t.Run("a list that never reaches the total stops after the pages that hold GitHub's maximum of 100 variables per environment", func(t *testing.T) {
		var pages int
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			pages++
			variables := make([]githubtest.VariableBody, 30)
			for i := range variables {
				variables[i] = githubtest.VariableBody{Name: fmt.Sprintf("P%s_%02d", r.URL.Query().Get("page"), i), Value: "v"}
			}
			_, _ = w.Write([]byte(githubtest.VariablesBody{TotalCount: 3000, Variables: variables}.JSON()))
		})

		err := c.observeEnvironmentVariables(t.Context(), mustReference(t, "owner/repo"), []Environment{{ID: 1, Name: "production"}}, allResources())
		if !errors.Is(err, ErrUnexpectedResponse) {
			t.Fatalf("observeEnvironmentVariables() error = %v, want %v", err, ErrUnexpectedResponse)
		}
		if pages != 4 {
			t.Errorf("observeEnvironmentVariables() read %d pages, want 4", pages)
		}
	})

	t.Run("404 for an environment in the list is not found naming that environment and stops before the next environment", func(t *testing.T) {
		const marker = "RESPONSE_BODY_MARKER"
		routes := &escapedPathRoutes{t: t, routes: map[string]githubtest.Response{
			environmentsPath + "/a/variables": githubtest.OK(githubtest.VariablesBody{}.JSON()),
			environmentsPath + "/b/variables": {Status: http.StatusNotFound, Body: `{"message": "` + marker + `"}`},
		}}
		c := newTestClient(t, routes.ServeHTTP)

		err := c.observeEnvironmentVariables(t.Context(), mustReference(t, "owner/repo"), []Environment{{ID: 1, Name: "a"}, {ID: 2, Name: "b"}, {ID: 3, Name: "c"}}, allResources())
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("observeEnvironmentVariables() error = %v, want %v", err, ErrNotFound)
		}
		if !strings.Contains(err.Error(), `variables of environment "b"`) {
			t.Errorf("observeEnvironmentVariables() error = %q, want it to name the environment", err)
		}
		if strings.Contains(err.Error(), marker) {
			t.Errorf("observeEnvironmentVariables() error = %q, want it not to contain the response body", err)
		}
		want := []string{environmentsPath + "/a/variables" + variablesFirstPage, environmentsPath + "/b/variables" + variablesFirstPage}
		if got := routes.Requests(); !reflect.DeepEqual(got, want) {
			t.Errorf("requests = %q, want %q", got, want)
		}
	})

	t.Run("an environment named as a dot segment is unexpected response without a request", func(t *testing.T) {
		routes := &escapedPathRoutes{t: t, routes: map[string]githubtest.Response{}}
		c := newTestClient(t, routes.ServeHTTP)

		err := c.observeEnvironmentVariables(t.Context(), mustReference(t, "owner/repo"), []Environment{{ID: 1, Name: "."}}, allResources())
		if !errors.Is(err, ErrUnexpectedResponse) {
			t.Fatalf("observeEnvironmentVariables() error = %v, want %v", err, ErrUnexpectedResponse)
		}
		if got := routes.Requests(); len(got) != 0 {
			t.Errorf("requests = %q, want none", got)
		}
	})

}

//trace:checks FR-004 QR-002
func TestObserveRepositoryEnvironmentVariables(t *testing.T) {
	selected := &githubtest.DeploymentBranchPolicyBody{CustomBranchPolicies: true}
	twoEnvironments := githubtest.OK(githubtest.EnvironmentsBody{TotalCount: 2, Environments: []githubtest.EnvironmentBody{
		{ID: 1, Name: "production", DeploymentBranchPolicy: selected},
		{ID: 2, Name: "staging"},
	}}.JSON())
	noPatterns := githubtest.OK(githubtest.DeploymentBranchPoliciesBody{}.JSON())
	noVariables := githubtest.OK(githubtest.VariablesBody{}.JSON())

	t.Run("an observation of the environments area alone sends no variables request and has no variables", func(t *testing.T) {
		routes := &escapedPathRoutes{t: t, routes: map[string]githubtest.Response{
			repositoryPath:   githubtest.OK(repositoryBody),
			environmentsPath: twoEnvironments,
			environmentsPath + "/production/deployment-branch-policies": noPatterns,
		}}
		c := newTestClient(t, routes.ServeHTTP)

		observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaEnvironments)
		if err != nil {
			t.Fatalf("ObserveRepository() error = %v", err)
		}
		for _, e := range observed.Environments.Existing {
			if e.Variables != nil {
				t.Errorf("environment %q Variables = %+v, want nil", e.Name, e.Variables)
			}
		}
	})

	t.Run("variables are read after every deployment pattern list and before dependabot whatever the order of the areas", func(t *testing.T) {
		routes := &escapedPathRoutes{t: t, routes: map[string]githubtest.Response{
			repositoryPath:   githubtest.OK(repositoryBody),
			environmentsPath: twoEnvironments,
			environmentsPath + "/production/deployment-branch-policies": noPatterns,
			environmentsPath + "/production/variables":                  noVariables,
			environmentsPath + "/staging/variables":                     noVariables,
			vulnerabilityAlertsPath:                                     {Status: http.StatusNoContent},
			automatedSecurityFixesPath:                                  githubtest.OK(githubtest.AutomatedSecurityFixesBody{Enabled: true}.JSON()),
		}}
		c := newTestClient(t, routes.ServeHTTP)

		_, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaDependabot, setting.AreaEnvironmentVariables, setting.AreaEnvironments)
		if err != nil {
			t.Fatalf("ObserveRepository() error = %v", err)
		}
		want := []string{
			repositoryPath + "?",
			environmentsPath + firstPage,
			environmentsPath + "/production/deployment-branch-policies" + firstPage,
			environmentsPath + "/production/variables" + variablesFirstPage,
			environmentsPath + "/staging/variables" + variablesFirstPage,
			vulnerabilityAlertsPath + "?",
			automatedSecurityFixesPath + "?",
		}
		if got := routes.Requests(); !reflect.DeepEqual(got, want) {
			t.Errorf("requests = %q, want %q", got, want)
		}
	})

	t.Run("the environment variables area alone also reads the environments and their deployment patterns", func(t *testing.T) {
		routes := &escapedPathRoutes{t: t, routes: map[string]githubtest.Response{
			repositoryPath:   githubtest.OK(repositoryBody),
			environmentsPath: twoEnvironments,
			environmentsPath + "/production/deployment-branch-policies": noPatterns,
			environmentsPath + "/production/variables":                  noVariables,
			environmentsPath + "/staging/variables":                     noVariables,
		}}
		c := newTestClient(t, routes.ServeHTTP)

		observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaEnvironmentVariables)
		if err != nil {
			t.Fatalf("ObserveRepository() error = %v", err)
		}
		if observed.Environments == nil || len(observed.Environments.Existing) != 2 {
			t.Fatalf("ObserveRepository().Environments = %+v, want the two environments", observed.Environments)
		}
		if patterns := environmentsPath + "/production/deployment-branch-policies" + firstPage; !slices.Contains(routes.Requests(), patterns) {
			t.Errorf("requests = %q, want them to contain %q", routes.Requests(), patterns)
		}
		for _, e := range observed.Environments.Existing {
			if !reflect.DeepEqual(e.Variables, &Variables{Existing: []Variable{}}) {
				t.Errorf("environment %q Variables = %+v, want an empty list", e.Name, e.Variables)
			}
		}
	})

	t.Run("a failed variables list fails the whole observation as the environment variables", func(t *testing.T) {
		routes := &escapedPathRoutes{t: t, routes: map[string]githubtest.Response{
			repositoryPath:   githubtest.OK(repositoryBody),
			environmentsPath: twoEnvironments,
			environmentsPath + "/production/deployment-branch-policies": noPatterns,
			environmentsPath + "/production/variables":                  noVariables,
			environmentsPath + "/staging/variables":                     {Status: http.StatusForbidden},
		}}
		c := newTestClient(t, routes.ServeHTTP)

		observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaEnvironmentVariables)
		if err == nil {
			t.Fatal("ObserveRepository() error = nil, want an error")
		}
		if !strings.Contains(err.Error(), "fetch environment variables owner/repo") {
			t.Errorf("ObserveRepository() error = %q, want it to name the environment variables", err)
		}
		if !reflect.DeepEqual(observed, ObservationResult{}) {
			t.Errorf("ObserveRepository() = %+v, want the zero observation", observed)
		}
	})

	t.Run("a repository variable and an environment variable of the same name are observed separately", func(t *testing.T) {
		routes := &escapedPathRoutes{t: t, routes: map[string]githubtest.Response{
			repositoryPath:                          githubtest.OK(repositoryBody),
			repositoryVariablesPath:                 githubtest.OK(githubtest.VariablesBody{TotalCount: 1, Variables: []githubtest.VariableBody{{Name: "REGION", Value: "repository"}}}.JSON()),
			environmentsPath:                        githubtest.OK(githubtest.EnvironmentsBody{TotalCount: 1, Environments: []githubtest.EnvironmentBody{{ID: 2, Name: "staging"}}}.JSON()),
			environmentsPath + "/staging/variables": githubtest.OK(githubtest.VariablesBody{TotalCount: 1, Variables: []githubtest.VariableBody{{Name: "REGION", Value: "environment"}}}.JSON()),
		}}
		c := newTestClient(t, routes.ServeHTTP)

		observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaRepositoryVariables, setting.AreaEnvironmentVariables)
		if err != nil {
			t.Fatalf("ObserveRepository() error = %v", err)
		}
		if want := (&Variables{Existing: []Variable{{Name: "REGION", Value: "repository"}}}); !reflect.DeepEqual(observed.RepositoryVariables, want) {
			t.Errorf("ObserveRepository().RepositoryVariables = %+v, want %+v", observed.RepositoryVariables, want)
		}
		if want := (&Variables{Existing: []Variable{{Name: "REGION", Value: "environment"}}}); !reflect.DeepEqual(observed.Environments.Existing[0].Variables, want) {
			t.Errorf("environment Variables = %+v, want %+v", observed.Environments.Existing[0].Variables, want)
		}
	})
}

//trace:checks FR-004
func TestVariables_Lookup(t *testing.T) {
	observed := Variables{Existing: []Variable{{Name: "GO_ARCH", Value: "amd64"}, {Name: "go_version", Value: "1.27"}, {Name: "EMPTY", Value: ""}}}
	cases := []struct {
		name     string
		variable string
		want     SettingObservation[setting.Value]
	}{
		{name: "same spelling returns the value", variable: "GO_ARCH", want: Observed(setting.VariableValue("amd64"))},
		{name: "lower case spelling in the list returns that variable", variable: "GO_VERSION", want: Observed(setting.VariableValue("1.27"))},
		{name: "empty value is an existing variable", variable: "EMPTY", want: Observed(setting.VariableValue(""))},
		{name: "name missing from the list is absent", variable: "GO", want: Observed(setting.Absent())},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := observed.Lookup(mustVariableName(t, tc.variable)); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Lookup(%q) = %+v, want %+v", tc.variable, got, tc.want)
			}
		})
	}
}

// observeRepositoryVariablesList は、変数の一覧が response を返すリポジトリを、Repository Variables の領域だけで観測する。
func observeRepositoryVariablesList(t *testing.T, response githubtest.Response) ObservationResult {
	t.Helper()
	routes := &escapedPathRoutes{t: t, routes: map[string]githubtest.Response{
		repositoryPath:          githubtest.OK(repositoryBody),
		repositoryVariablesPath: response,
	}}
	c := newTestClient(t, routes.ServeHTTP)
	observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaRepositoryVariables)
	if err != nil {
		t.Fatalf("ObserveRepository() error = %v", err)
	}
	return observed
}
