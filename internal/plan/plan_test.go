package plan

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

func mustReference(t *testing.T, name string) repository.Reference {
	t.Helper()
	reference, err := repository.ParseReference(name)
	if err != nil {
		t.Fatalf("ParseReference(%q) error = %v", name, err)
	}
	return reference
}

func TestPluralize(t *testing.T) {
	cases := []struct {
		name             string
		n                int
		singular, plural string
		want             string
	}{
		{name: "zero is plural", n: 0, singular: "known change", plural: "known changes", want: "0 known changes"},
		{name: "one is singular", n: 1, singular: "known change", plural: "known changes", want: "1 known change"},
		{name: "two uses the given plural form", n: 2, singular: "repository", plural: "repositories", want: "2 repositories"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pluralize(tc.n, tc.singular, tc.plural); got != tc.want {
				t.Errorf("pluralize() = %q, want %q", got, tc.want)
			}
		})
	}
}

//trace:checks FR-005
//trace:checks FR-007
//trace:checks QR-002
func TestWriteChangePlan(t *testing.T) {
	target := mustReference(t, "owner/repo")
	cases := []struct {
		name     string
		settings []changeplan.SettingPlan
		want     string
	}{
		{
			name: "mismatch is shown as a change with a summary",
			settings: []changeplan.SettingPlan{
				{Key: "general.allow_auto_merge", Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: changeplan.Mismatch, Applicability: changeplan.Applicable},
			},
			want: "Configuration: ./reco.yaml\n\nRepository: owner/repo\n  general.allow_auto_merge: false -> true\n\n" +
				"Plan: 1 repository to update, 1 known change, 0 not-comparable settings\n",
		},
		{
			name: "general and workflow permissions mismatches are shown in the plan order with enumerated values and counted",
			settings: []changeplan.SettingPlan{
				{Key: setting.AllowAutoMerge, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: changeplan.Mismatch, Applicability: changeplan.Applicable},
				{
					Key:      setting.DefaultWorkflowPermissions,
					Observed: github.Observed(setting.PermissionLevelValue(setting.PermissionWrite)), Desired: setting.PermissionLevelValue(setting.PermissionRead),
					Result: changeplan.Mismatch, Applicability: changeplan.Applicable,
				},
				{Key: setting.CanApprovePullRequestReviews, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(false), Result: changeplan.Mismatch, Applicability: changeplan.Applicable},
			},
			want: "Configuration: ./reco.yaml\n\nRepository: owner/repo\n" +
				"  general.allow_auto_merge: false -> true\n" +
				"  actions.workflow.default_workflow_permissions: write -> read\n" +
				"  actions.workflow.can_approve_pull_request_reviews: true -> false\n\n" +
				"Plan: 1 repository to update, 3 known changes, 0 not-comparable settings\n",
		},
		{
			name: "matched settings are not shown as changes",
			settings: []changeplan.SettingPlan{
				{Key: "general.delete_branch_on_merge", Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(true), Result: changeplan.Match},
				{Key: "general.allow_auto_merge", Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(false), Result: changeplan.Mismatch, Applicability: changeplan.Applicable},
			},
			want: "Configuration: ./reco.yaml\n\nRepository: owner/repo\n  general.allow_auto_merge: true -> false\n\n" +
				"Plan: 1 repository to update, 1 known change, 0 not-comparable settings\n",
		},
		{
			name: "all matches report no changes",
			settings: []changeplan.SettingPlan{
				{Key: "general.allow_auto_merge", Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(false), Result: changeplan.Match},
			},
			want: "No changes.\nRepositories checked: 1\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			p := changeplan.Plan{Target: target, RepositoryID: 1, Settings: tc.settings}
			if err := writeChangePlan(&out, "./reco.yaml", p); err != nil {
				t.Fatalf("writeChangePlan() error = %v", err)
			}
			if out.String() != tc.want {
				t.Errorf("writeChangePlan() wrote %q, want %q", out.String(), tc.want)
			}
		})
	}
}

// 期待値は CLI 仕様 4章の表示例から導く。
//
//trace:checks FR-005
//trace:checks FR-007
//trace:checks QR-002
//trace:checks INV-001
func TestWriteChangePlan_ActionsPermissions(t *testing.T) {
	target := mustReference(t, "owner/repo")
	criteria := setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, VerifiedAllowed: true, PatternsAllowed: []string{"example/*"}}
	policy := setting.AllowedActionsPolicyValue
	disabled := github.Unavailable[setting.Value](github.UnavailableActionsDisabled)
	notSelected := github.Unavailable[setting.Value](github.UnavailableNotSelected)
	const header = "Configuration: ./reco.yaml\n\nRepository: owner/repo\n"
	const possibleLoss = "    Side effect: GitHub may discard selected_actions if allowed_actions was \"selected\" before GitHub Actions was disabled (current values are unavailable)\n"
	cases := []struct {
		name     string
		settings []changeplan.SettingPlan
		want     string
	}{
		{
			name: "selected to all shows the side effect with the observed criteria under the known change",
			settings: []changeplan.SettingPlan{{
				Key: setting.AllowedActions, Observed: github.Observed(policy(setting.AllowedActionsSelected)), Desired: policy(setting.AllowedActionsAll),
				Result: changeplan.Mismatch, Applicability: changeplan.Applicable,
				SideEffects: []changeplan.SideEffect{{Key: setting.SelectedActions, Observed: github.Observed(setting.SelectedActionsCriteriaValue(criteria))}},
			}},
			want: header +
				"  actions.permissions.allowed_actions: selected -> all\n" +
				"    Side effect: GitHub discards the current selected_actions (github_owned_allowed: true, verified_allowed: true, patterns_allowed: [\"example/*\"])\n\n" +
				"Plan: 1 repository to update, 1 known change, 0 not-comparable settings\n",
		},
		{
			name: "disabled to local_only is a not-comparable setting that enables actions and may discard the criteria",
			settings: []changeplan.SettingPlan{{
				Key: setting.AllowedActions, Observed: disabled, Desired: policy(setting.AllowedActionsLocalOnly),
				Result: changeplan.NotComparable, Applicability: changeplan.Applicable,
				SideEffects: []changeplan.SideEffect{{Key: setting.SelectedActions, Observed: disabled}},
			}},
			want: header +
				"  actions.permissions.allowed_actions\n" +
				"    Current: unavailable (GitHub Actions is disabled)\n" +
				"    Desired: local_only (enables GitHub Actions)\n" +
				possibleLoss + "\n" +
				"Plan: 1 repository to update, 0 known changes, 1 not-comparable setting\n",
		},
		{
			name: "all to selected counts the policy as a known change and the criteria group as one not-comparable setting",
			settings: []changeplan.SettingPlan{
				{Key: setting.AllowedActions, Observed: github.Observed(policy(setting.AllowedActionsAll)), Desired: policy(setting.AllowedActionsSelected), Result: changeplan.Mismatch, Applicability: changeplan.Applicable},
				{Key: setting.SelectedActions, Observed: notSelected, Desired: setting.SelectedActionsCriteriaValue(criteria), Result: changeplan.NotComparable, Applicability: changeplan.Applicable},
			},
			want: header +
				"  actions.permissions.allowed_actions: all -> selected\n" +
				"  actions.permissions.selected_actions\n" +
				"    Current: unavailable (allowed_actions is not \"selected\")\n" +
				"    Desired: github_owned_allowed: true, verified_allowed: true, patterns_allowed: [\"example/*\"]\n\n" +
				"Plan: 1 repository to update, 1 known change, 1 not-comparable setting\n",
		},
		{
			name: "disabled to selected marks only the policy as enabling actions",
			settings: []changeplan.SettingPlan{
				{Key: setting.AllowedActions, Observed: disabled, Desired: policy(setting.AllowedActionsSelected), Result: changeplan.NotComparable, Applicability: changeplan.Applicable},
				{Key: setting.SelectedActions, Observed: disabled, Desired: setting.SelectedActionsCriteriaValue(criteria), Result: changeplan.NotComparable, Applicability: changeplan.Applicable},
			},
			want: header +
				"  actions.permissions.allowed_actions\n" +
				"    Current: unavailable (GitHub Actions is disabled)\n" +
				"    Desired: selected (enables GitHub Actions)\n" +
				"  actions.permissions.selected_actions\n" +
				"    Current: unavailable (GitHub Actions is disabled)\n" +
				"    Desired: github_owned_allowed: true, verified_allowed: true, patterns_allowed: [\"example/*\"]\n\n" +
				"Plan: 1 repository to update, 0 known changes, 2 not-comparable settings\n",
		},
		{
			name: "criteria group known change shows the current and desired values on separate lines",
			settings: []changeplan.SettingPlan{
				{Key: setting.AllowedActions, Observed: github.Observed(policy(setting.AllowedActionsSelected)), Desired: policy(setting.AllowedActionsSelected), Result: changeplan.Match},
				{
					Key:      setting.SelectedActions,
					Observed: github.Observed(setting.SelectedActionsCriteriaValue(setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, PatternsAllowed: []string{}})),
					Desired:  setting.SelectedActionsCriteriaValue(criteria), Result: changeplan.Mismatch, Applicability: changeplan.Applicable,
				},
			},
			want: header +
				"  actions.permissions.selected_actions:\n" +
				"    github_owned_allowed: true, verified_allowed: false, patterns_allowed: []\n" +
				"    -> github_owned_allowed: true, verified_allowed: true, patterns_allowed: [\"example/*\"]\n\n" +
				"Plan: 1 repository to update, 1 known change, 0 not-comparable settings\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			p := changeplan.Plan{Target: target, RepositoryID: 1, Settings: tc.settings}
			if err := writeChangePlan(&out, "./reco.yaml", p); err != nil {
				t.Fatalf("writeChangePlan() error = %v", err)
			}
			if out.String() != tc.want {
				t.Errorf("writeChangePlan() wrote %q, want %q", out.String(), tc.want)
			}
		})
	}
}

func TestSideEffectText_UnknownSettingPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("sideEffectText() did not panic for a side effect on another setting")
		}
	}()
	_ = sideEffectText(changeplan.SideEffect{Key: setting.AllowAutoMerge, Observed: github.Observed(setting.BoolValue(true))})
}

func TestWriteChangePlan_WriteFailure(t *testing.T) {
	target := mustReference(t, "owner/repo")

	t.Run("write failure is returned", func(t *testing.T) {
		p := changeplan.Plan{Target: target, Settings: []changeplan.SettingPlan{{
			Key: setting.AllowAutoMerge, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true),
			Result: changeplan.Mismatch, Applicability: changeplan.Applicable,
		}}}
		err := writeChangePlan(failingWriter{}, "./reco.yaml", p)
		if !errors.Is(err, errWrite) {
			t.Errorf("writeChangePlan() error = %v, want %v", err, errWrite)
		}
	})

	t.Run("no-change write failure is returned", func(t *testing.T) {
		err := writeChangePlan(failingWriter{}, "./reco.yaml", changeplan.Plan{Target: target})
		if !errors.Is(err, errWrite) {
			t.Errorf("writeChangePlan() error = %v, want %v", err, errWrite)
		}
	})
}

var errWrite = errors.New("write failed")

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }

const configSource = `repositories:
  - name: owner/repo
    general:
      allow_auto_merge: true
`

//trace:checks FR-007
func TestRun(t *testing.T) {
	newConfig := func(files fstest.MapFS, out *bytes.Buffer, baseURL *url.URL) Config {
		return Config{
			Token:         "test-token",
			ConfigDir:     files,
			ConfigName:    "reco.yaml",
			ConfigPath:    "./reco.yaml",
			Out:           out,
			GitHubOptions: []github.Option{github.WithBaseURL(baseURL)},
		}
	}

	t.Run("generated change plan is written to Out", func(t *testing.T) {
		var gotAuthorization string
		baseURL := newGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
			gotAuthorization = r.Header.Get("Authorization")
			_, _ = w.Write([]byte(`{"id": 1, "allow_auto_merge": false, "delete_branch_on_merge": false}`))
		})
		var out bytes.Buffer
		files := fstest.MapFS{"reco.yaml": {Data: []byte(configSource)}}

		if err := Run(context.Background(), newConfig(files, &out, baseURL)); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if !strings.Contains(gotAuthorization, "test-token") {
			t.Errorf("got Authorization %q, want it to contain the token", gotAuthorization)
		}
		for _, want := range []string{"false -> true", "Configuration: ./reco.yaml"} {
			if !strings.Contains(out.String(), want) {
				t.Errorf("got Out %q, want it to contain %q", out.String(), want)
			}
		}
	})

	t.Run("selection is passed to change planning", func(t *testing.T) {
		requested := false
		baseURL := newGitHubServer(t, func(http.ResponseWriter, *http.Request) { requested = true })
		var out bytes.Buffer
		files := fstest.MapFS{"reco.yaml": {Data: []byte(configSource)}}
		cfg := newConfig(files, &out, baseURL)
		selection := mustReference(t, "owner/other")
		cfg.Selection = &selection

		if err := Run(context.Background(), cfg); err == nil {
			t.Fatal("Run() error = nil, want an error")
		}
		if requested {
			t.Error("GitHub received a request, want none")
		}
	})

	t.Run("configuration path is passed to change planning", func(t *testing.T) {
		baseURL := newGitHubServer(t, func(http.ResponseWriter, *http.Request) {})
		var out bytes.Buffer

		err := Run(context.Background(), newConfig(fstest.MapFS{}, &out, baseURL))
		if err == nil || !strings.Contains(err.Error(), "./reco.yaml") {
			t.Errorf("Run() error = %v, want it to contain the configuration path", err)
		}
	})

	t.Run("change planning failure is returned without writing to Out", func(t *testing.T) {
		baseURL := newGitHubServer(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		var out bytes.Buffer
		files := fstest.MapFS{"reco.yaml": {Data: []byte(configSource)}}

		err := Run(context.Background(), newConfig(files, &out, baseURL))
		if !errors.Is(err, github.ErrNotFound) {
			t.Errorf("Run() error = %v, want %v", err, github.ErrNotFound)
		}
		if out.Len() != 0 {
			t.Errorf("got Out %q, want empty", out.String())
		}
	})
}

func newGitHubServer(t *testing.T, handler http.HandlerFunc) *url.URL {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	return u
}
