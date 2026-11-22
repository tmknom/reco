package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
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
		observed := observeVariablesList(t, githubtest.OK(githubtest.VariablesBody{}.JSON()))
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
		observed := observeVariablesList(t, githubtest.OK(githubtest.VariablesBody{TotalCount: 2, Variables: []githubtest.VariableBody{
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
		observed := observeVariablesList(t, githubtest.OK(body))
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
		{name: "name missing from the list is absent", variable: "GO", want: Observed(setting.AbsentVariable())},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := observed.Lookup(mustVariableName(t, tc.variable)); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Lookup(%q) = %+v, want %+v", tc.variable, got, tc.want)
			}
		})
	}
}

// observeVariablesList は、変数の一覧が response を返すリポジトリを、Repository Variables の領域だけで観測する。
func observeVariablesList(t *testing.T, response githubtest.Response) ObservationResult {
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
