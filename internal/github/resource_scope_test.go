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

// TestObserveRepositoryForNamedResources_Environments は、読む範囲が個別パターンと変数の両方の読み取りに効き、範囲の外の Environment を読まないまま残すつなぎ目を確かめる。
//
//trace:checks FR-004 QR-002
func TestObserveRepositoryForNamedResources_Environments(t *testing.T) {
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

			observed, err := c.ObserveRepositoryForNamedResources(t.Context(), mustReference(t, "owner/repo"), NamedResources{Environments: names}, tc.areas...)
			if err != nil {
				t.Fatalf("ObserveRepositoryForNamedResources() error = %v", err)
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

// TestObserveRepositoryForNamedResources_Rulesets は、名指した Ruleset にだけ詳細を読み、ほかの Ruleset を値を読まないまま一覧に残すつなぎ目を確かめる。
//
//trace:checks FR-004
func TestObserveRepositoryForNamedResources_Rulesets(t *testing.T) {
	detail := func(id int64, name string) githubtest.Response {
		b := githubtest.RulesetBody{ID: id, Name: name, Target: "branch", Enforcement: "active", Rules: []string{githubtest.DeletionRuleBody}}
		return githubtest.OK(b.JSON())
	}
	list := func(summaries ...githubtest.RulesetSummaryBody) githubtest.Response {
		return githubtest.OK(githubtest.RulesetsBody(summaries).JSON())
	}
	selected := &githubtest.DeploymentBranchPolicyBody{CustomBranchPolicies: true}
	noPatterns := githubtest.OK(githubtest.DeploymentBranchPoliciesBody{}.JSON())
	policiesPath := func(name string) string { return environmentsPath + "/" + name + "/deployment-branch-policies" }

	cases := []struct {
		name  string
		named func(t *testing.T) NamedResources
		areas []setting.Area
		// routes は、送ってよい要求の応答。名指さない Ruleset の詳細も置き、送れば要求の並びの比較で検出する。
		routes       map[string]githubtest.Response
		wantRequests []string
		wantRulesets []Ruleset
	}{
		{
			name: "details are read only for the named ruleset of the same name, not for a name whose identity cannot be determined",
			named: func(t *testing.T) NamedResources {
				return NamedResources{Rulesets: []setting.RulesetName{mustRulesetName(t, "release"), mustRulesetName(t, "protect")}}
			},
			areas: []setting.Area{setting.AreaRulesets},
			routes: map[string]githubtest.Response{
				rulesetsPath:        list(githubtest.RulesetSummaryBody{ID: 1, Name: "other"}, githubtest.RulesetSummaryBody{ID: 2, Name: "Protect"}, githubtest.RulesetSummaryBody{ID: 3, Name: "Releasé"}),
				rulesetsPath + "/1": detail(1, "other"),
				rulesetsPath + "/2": detail(2, "Protect"),
				rulesetsPath + "/3": detail(3, "Releasé"),
			},
			wantRequests: []string{repositoryPath + "?", rulesetsList, rulesetsPath + "/2?"},
			wantRulesets: []Ruleset{unreadRuleset(1, "other"), InterpretedRuleset(2, "Protect", interpretedRulesetValue()), unreadRuleset(3, "Releasé")},
		},
		{
			name:  "no named ruleset reads only the list",
			named: func(*testing.T) NamedResources { return NamedResources{} },
			areas: []setting.Area{setting.AreaRulesets},
			routes: map[string]githubtest.Response{
				rulesetsPath:        list(githubtest.RulesetSummaryBody{ID: 1, Name: "protect"}),
				rulesetsPath + "/1": detail(1, "protect"),
			},
			wantRequests: []string{repositoryPath + "?", rulesetsList},
			wantRulesets: []Ruleset{unreadRuleset(1, "protect")},
		},
		{
			name: "environment names and ruleset names limit only their own kind",
			named: func(t *testing.T) NamedResources {
				return NamedResources{Environments: []setting.EnvironmentName{mustEnvironmentName(t, "production")}, Rulesets: []setting.RulesetName{mustRulesetName(t, "protect")}}
			},
			areas: []setting.Area{setting.AreaEnvironments, setting.AreaRulesets},
			routes: map[string]githubtest.Response{
				environmentsPath: githubtest.OK(githubtest.EnvironmentsBody{TotalCount: 2, Environments: []githubtest.EnvironmentBody{
					{ID: 1, Name: "production", DeploymentBranchPolicy: selected},
					{ID: 2, Name: "protect", DeploymentBranchPolicy: selected},
				}}.JSON()),
				policiesPath("production"): noPatterns,
				policiesPath("protect"):    noPatterns,
				rulesetsPath:               list(githubtest.RulesetSummaryBody{ID: 1, Name: "production"}, githubtest.RulesetSummaryBody{ID: 2, Name: "protect"}),
				rulesetsPath + "/1":        detail(1, "production"),
				rulesetsPath + "/2":        detail(2, "protect"),
			},
			wantRequests: []string{repositoryPath + "?", environmentsPath + firstPage, policiesPath("production") + firstPage, rulesetsList, rulesetsPath + "/2?"},
			wantRulesets: []Ruleset{unreadRuleset(1, "production"), InterpretedRuleset(2, "protect", interpretedRulesetValue())},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.routes[repositoryPath] = githubtest.OK(repositoryBody)
			routes := &escapedPathRoutes{t: t, routes: tc.routes}
			c := newTestClient(t, routes.ServeHTTP)

			observed, err := c.ObserveRepositoryForNamedResources(t.Context(), mustReference(t, "owner/repo"), tc.named(t), tc.areas...)
			if err != nil {
				t.Fatalf("ObserveRepositoryForNamedResources() error = %v", err)
			}
			if got := routes.Requests(); !reflect.DeepEqual(got, tc.wantRequests) {
				t.Errorf("requests = %q, want %q", got, tc.wantRequests)
			}
			if want := (&Rulesets{Existing: Observed(tc.wantRulesets)}); !reflect.DeepEqual(observed.Rulesets, want) {
				t.Errorf("Rulesets = %+v, want %+v", observed.Rulesets, want)
			}
		})
	}
}

//trace:checks FR-004
func TestResourceScope_IncludesEnvironment(t *testing.T) {
	named := func(names ...string) resourceScope {
		environments := make([]setting.EnvironmentName, len(names))
		for i, name := range names {
			environments[i] = mustEnvironmentName(t, name)
		}
		return scopeOfNamed(NamedResources{Environments: environments})
	}
	cases := []struct {
		name     string
		scope    resourceScope
		observed string
		want     bool
	}{
		{name: "all resources include any name", scope: allResources(), observed: "café", want: true},
		{name: "a name that differs only in ASCII case is included", scope: named("qa", "production"), observed: "Production", want: true},
		{name: "a different name is not included", scope: named("production"), observed: "staging"},
		{name: "a name whose identity cannot be determined is not included", scope: named("cafe"), observed: "café"},
		{name: "no named environment includes no name", scope: named(), observed: "production"},
		{
			// 削除で失われる値は個別パターンを含むため、使う側が Environments に名指さなくても読む
			name:     "an environment named only for reading what its deletion loses is included",
			scope:    scopeOfNamed(NamedResources{EnvironmentsReadForDeletion: []setting.EnvironmentName{mustEnvironmentName(t, "production")}}),
			observed: "Production",
			want:     true,
		},
		{
			name:     "a named ruleset does not include an environment of the same name",
			scope:    scopeOfNamed(NamedResources{Rulesets: []setting.RulesetName{mustRulesetName(t, "production")}}),
			observed: "production",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.scope.includesEnvironment(tc.observed); got != tc.want {
				t.Errorf("includesEnvironment(%q) = %v, want %v", tc.observed, got, tc.want)
			}
		})
	}
}

//trace:checks FR-004 INV-001
func TestResourceScope_IncludesEnvironmentDeletion(t *testing.T) {
	readForDeletion := func(names ...string) resourceScope {
		environments := make([]setting.EnvironmentName, len(names))
		for i, name := range names {
			environments[i] = mustEnvironmentName(t, name)
		}
		return scopeOfNamed(NamedResources{EnvironmentsReadForDeletion: environments})
	}
	cases := []struct {
		name     string
		scope    resourceScope
		observed string
		want     bool
	}{
		{name: "a name that differs only in ASCII case is included", scope: readForDeletion("qa", "legacy"), observed: "Legacy", want: true},
		{name: "a different name is not included", scope: readForDeletion("legacy"), observed: "staging"},
		{name: "a name whose identity cannot be determined is not included", scope: readForDeletion("cafe"), observed: "café"},
		// show は allResources で全 Environment を読むが、Secret の要求は送らない
		{name: "all resources include no name", scope: allResources(), observed: "legacy"},
		{
			name:     "an environment named only for reading its settings is not included",
			scope:    scopeOfNamed(NamedResources{Environments: []setting.EnvironmentName{mustEnvironmentName(t, "legacy")}}),
			observed: "legacy",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.scope.includesEnvironmentDeletion(tc.observed); got != tc.want {
				t.Errorf("includesEnvironmentDeletion(%q) = %v, want %v", tc.observed, got, tc.want)
			}
		})
	}
}

// TestObserveRepositoryForNamedResources_EnvironmentDeletions は、削除で失われるものまで読む範囲の Environment にだけ、
// 個別パターンのあとに変数と Secret を読み、変数を二度読まないつなぎ目を確かめる。
//
//trace:checks FR-004 INV-001
func TestObserveRepositoryForNamedResources_EnvironmentDeletions(t *testing.T) {
	selected := &githubtest.DeploymentBranchPolicyBody{CustomBranchPolicies: true}
	noPatterns := githubtest.OK(githubtest.DeploymentBranchPoliciesBody{}.JSON())
	noVariables := githubtest.OK(githubtest.VariablesBody{}.JSON())
	noSecrets := githubtest.OK(environmentSecretsBody(0))
	policiesPath := func(name string) string { return environmentsPath + "/" + name + "/deployment-branch-policies" }
	variablesPath := func(name string) string { return environmentsPath + "/" + name + "/variables" }
	secretsPath := func(name string) string { return environmentsPath + "/" + name + "/secrets" }
	list := func(environments ...githubtest.EnvironmentBody) githubtest.Response {
		return githubtest.OK(githubtest.EnvironmentsBody{TotalCount: len(environments), Environments: environments}.JSON())
	}
	twoEnvironments := list(githubtest.EnvironmentBody{ID: 1, Name: "Legacy", DeploymentBranchPolicy: selected}, githubtest.EnvironmentBody{ID: 2, Name: "staging"})

	cases := []struct {
		name string
		// environments は Environments に名指す名前で、readForDeletion は EnvironmentsReadForDeletion に名指す名前。
		environments    []string
		readForDeletion []string
		areas           []setting.Area
		// routes は、送ってよい要求の応答。範囲の外の Environment の応答も置き、送れば要求の並びの比較で検出する。
		routes       map[string]githubtest.Response
		wantRequests []string
		// wantRead は、変数と Secret の両方を読んだはずの Environment の名前。ほかの Environment は Secret を読んでいないはずである。
		wantRead []string
	}{
		{
			// legacy を Environments に名指さず、削除で失われるものまで読む範囲から個別パターンも読むことを確かめる
			name:            "variables and secrets of the environment read for deletion follow its deployment patterns",
			environments:    []string{"staging"},
			readForDeletion: []string{"legacy"},
			areas:           []setting.Area{setting.AreaEnvironments},
			routes: map[string]githubtest.Response{
				environmentsPath:         twoEnvironments,
				policiesPath("Legacy"):   noPatterns,
				variablesPath("Legacy"):  noVariables,
				secretsPath("Legacy"):    noSecrets,
				variablesPath("staging"): noVariables,
				secretsPath("staging"):   noSecrets,
			},
			wantRequests: []string{
				repositoryPath + "?", environmentsPath + firstPage, policiesPath("Legacy") + firstPage,
				variablesPath("Legacy") + variablesFirstPage, secretsPath("Legacy") + firstPage,
			},
			wantRead: []string{"Legacy"},
		},
		{
			name:            "an environment read for deletion that is not in the list gets no request",
			readForDeletion: []string{"legacy"},
			areas:           []setting.Area{setting.AreaEnvironments},
			routes: map[string]githubtest.Response{
				environmentsPath:         list(githubtest.EnvironmentBody{ID: 2, Name: "staging"}),
				variablesPath("staging"): noVariables,
				secretsPath("staging"):   noSecrets,
			},
			wantRequests: []string{repositoryPath + "?", environmentsPath + firstPage},
		},
		{
			name:            "variables already read for the environment variables area are not read again",
			environments:    []string{"staging"},
			readForDeletion: []string{"legacy"},
			areas:           []setting.Area{setting.AreaEnvironments, setting.AreaEnvironmentVariables},
			routes: map[string]githubtest.Response{
				environmentsPath:         twoEnvironments,
				policiesPath("Legacy"):   noPatterns,
				variablesPath("Legacy"):  noVariables,
				variablesPath("staging"): noVariables,
				secretsPath("Legacy"):    noSecrets,
				secretsPath("staging"):   noSecrets,
			},
			wantRequests: []string{
				repositoryPath + "?", environmentsPath + firstPage, policiesPath("Legacy") + firstPage,
				variablesPath("Legacy") + variablesFirstPage, variablesPath("staging") + variablesFirstPage,
				secretsPath("Legacy") + firstPage,
			},
			wantRead: []string{"Legacy"},
		},
		{
			name:            "no area of environments reads no secrets",
			readForDeletion: []string{"legacy"},
			areas:           nil,
			routes:          map[string]githubtest.Response{},
			wantRequests:    []string{repositoryPath + "?"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.routes[repositoryPath] = githubtest.OK(repositoryBody)
			routes := &escapedPathRoutes{t: t, routes: tc.routes}
			c := newTestClient(t, routes.ServeHTTP)
			names := func(ss []string) []setting.EnvironmentName {
				parsed := make([]setting.EnvironmentName, len(ss))
				for i, s := range ss {
					parsed[i] = mustEnvironmentName(t, s)
				}
				return parsed
			}
			named := NamedResources{Environments: names(tc.environments), EnvironmentsReadForDeletion: names(tc.readForDeletion)}

			observed, err := c.ObserveRepositoryForNamedResources(t.Context(), mustReference(t, "owner/repo"), named, tc.areas...)
			if err != nil {
				t.Fatalf("ObserveRepositoryForNamedResources() error = %v", err)
			}
			if got := routes.Requests(); !reflect.DeepEqual(got, tc.wantRequests) {
				t.Errorf("requests = %q, want %q", got, tc.wantRequests)
			}
			if observed.Environments == nil {
				return
			}
			for _, e := range observed.Environments.Existing {
				read := slices.Contains(tc.wantRead, e.Name)
				if read && (e.Variables == nil || !reflect.DeepEqual(e.Secrets, &EnvironmentSecrets{Existing: []EnvironmentSecret{}})) {
					t.Errorf("environment %q = %+v, want its variables and no secrets read", e.Name, e)
				}
				if !read && e.Secrets != nil {
					t.Errorf("environment %q secrets = %+v, want nil", e.Name, e.Secrets)
				}
			}
		})
	}
}

// TestObserveRepositoryForNamedResources_EnvironmentDeletionFailure は、削除で失われるものの読み取りの失敗が観測の失敗になり、
// どの読み取りかを示すことを確かめる。
//
//trace:checks FR-003 FR-004
func TestObserveRepositoryForNamedResources_EnvironmentDeletionFailure(t *testing.T) {
	environments := githubtest.OK(githubtest.EnvironmentsBody{TotalCount: 1, Environments: []githubtest.EnvironmentBody{{ID: 1, Name: "legacy"}}}.JSON())
	cases := []struct {
		name     string
		routes   map[string]githubtest.Response
		wantErr  error
		contains string
	}{
		{
			name: "404 of the secrets is not found naming the environment",
			routes: map[string]githubtest.Response{
				environmentsPath + "/legacy/variables": githubtest.OK(githubtest.VariablesBody{}.JSON()),
				environmentsPath + "/legacy/secrets":   {Status: http.StatusNotFound},
			},
			wantErr:  ErrNotFound,
			contains: `fetch environments owner/repo: secrets of environment "legacy"`,
		},
		{
			name: "403 of the variables is forbidden naming the environment and reads no secrets",
			routes: map[string]githubtest.Response{
				environmentsPath + "/legacy/variables": {Status: http.StatusForbidden},
			},
			wantErr:  ErrForbidden,
			contains: `fetch environments owner/repo: variables of environment "legacy"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.routes[repositoryPath] = githubtest.OK(repositoryBody)
			tc.routes[environmentsPath] = environments
			routes := &escapedPathRoutes{t: t, routes: tc.routes}
			c := newTestClient(t, routes.ServeHTTP)
			legacy := []setting.EnvironmentName{mustEnvironmentName(t, "legacy")}

			_, err := c.ObserveRepositoryForNamedResources(t.Context(), mustReference(t, "owner/repo"),
				NamedResources{EnvironmentsReadForDeletion: legacy}, setting.AreaEnvironments)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ObserveRepositoryForNamedResources() error = %v, want %v", err, tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.contains) {
				t.Errorf("ObserveRepositoryForNamedResources() error = %q, want it to contain %q", err, tc.contains)
			}
		})
	}
}

// TestObserveRepository_ReadsNoEnvironmentSecrets は、状態確認（show）の経路が、全 Environment を読んでも Secret の要求を送らないことを確かめる。
//
//trace:checks FR-004
func TestObserveRepository_ReadsNoEnvironmentSecrets(t *testing.T) {
	routes := &escapedPathRoutes{t: t, routes: map[string]githubtest.Response{
		repositoryPath:                         githubtest.OK(repositoryBody),
		environmentsPath:                       githubtest.OK(githubtest.EnvironmentsBody{TotalCount: 1, Environments: []githubtest.EnvironmentBody{{ID: 1, Name: "legacy"}}}.JSON()),
		environmentsPath + "/legacy/variables": githubtest.OK(githubtest.VariablesBody{}.JSON()),
		environmentsPath + "/legacy/secrets":   githubtest.OK(environmentSecretsBody(0)),
	}}
	c := newTestClient(t, routes.ServeHTTP)

	observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaEnvironments, setting.AreaEnvironmentVariables)
	if err != nil {
		t.Fatalf("ObserveRepository() error = %v", err)
	}
	want := []string{repositoryPath + "?", environmentsPath + firstPage, environmentsPath + "/legacy/variables" + variablesFirstPage}
	if got := routes.Requests(); !reflect.DeepEqual(got, want) {
		t.Errorf("requests = %q, want %q", got, want)
	}
	if secrets := observed.Environments.Existing[0].Secrets; secrets != nil {
		t.Errorf("Secrets = %+v, want nil", secrets)
	}
}

//trace:checks FR-004 FR-005
func TestResourceScope_IncludesRuleset(t *testing.T) {
	named := func(names ...string) resourceScope {
		rulesets := make([]setting.RulesetName, len(names))
		for i, name := range names {
			rulesets[i] = mustRulesetName(t, name)
		}
		return scopeOfNamed(NamedResources{Rulesets: rulesets})
	}
	cases := []struct {
		name     string
		scope    resourceScope
		observed string
		want     bool
	}{
		{name: "all resources include any name", scope: allResources(), observed: "Protéct", want: true},
		{name: "a name that differs only in ASCII case is included", scope: named("protect"), observed: "Protect", want: true},
		{name: "a different name is not included", scope: named("protect"), observed: "release"},
		{name: "a name whose identity cannot be determined is not included", scope: named("protect"), observed: "Protéct"},
		{name: "no named ruleset includes no name", scope: named(), observed: "protect"},
		{
			name:     "a named environment does not include a ruleset of the same name",
			scope:    scopeOfNamed(NamedResources{Environments: []setting.EnvironmentName{mustEnvironmentName(t, "protect")}}),
			observed: "protect",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.scope.includesRuleset(tc.observed); got != tc.want {
				t.Errorf("includesRuleset(%q) = %v, want %v", tc.observed, got, tc.want)
			}
		})
	}
}
