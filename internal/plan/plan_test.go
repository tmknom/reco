package plan

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/githubtest"
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

var errWrite = errors.New("write failed")

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }

const repositoryRoute = "GET /repos/owner/repo"

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
			Configuration: configuration.File{Dir: files, Name: "reco.yaml", Path: "./reco.yaml"},
			Out:           out,
			GitHubOptions: []github.Option{github.WithBaseURL(baseURL)},
		}
	}

	t.Run("generated change plan is written to Out", func(t *testing.T) {
		server := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
			repositoryRoute: githubtest.OK(githubtest.RepositoryBody{ID: 1}.JSON()),
		}))
		var out bytes.Buffer
		files := fstest.MapFS{"reco.yaml": {Data: []byte(configSource)}}

		if err := Run(context.Background(), newConfig(files, &out, server.URL())); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		requests := server.Requests()
		if len(requests) == 0 {
			t.Fatal("GitHub received no request, want at least one")
		}
		for _, r := range requests {
			if !strings.Contains(r.Authorization, "test-token") {
				t.Errorf("got Authorization %q, want it to contain the token", r.Authorization)
			}
		}
		for _, want := range []string{"false -> true", "Configuration: ./reco.yaml"} {
			if !strings.Contains(out.String(), want) {
				t.Errorf("got Out %q, want it to contain %q", out.String(), want)
			}
		}
	})

	t.Run("not-applicable change plan is written to Out in full before the error is returned", func(t *testing.T) {
		server := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
			repositoryRoute: githubtest.OK(githubtest.RepositoryBody{ID: 1}.JSON()),
			"GET /repos/owner/repo/actions/permissions": githubtest.OK(githubtest.ActionsDisabledBody),
		}))
		var out bytes.Buffer
		files := fstest.MapFS{"reco.yaml": {Data: []byte("repositories:\n  - name: owner/repo\n    actions:\n      permissions:\n        sha_pinning_required: true\n")}}

		if err := Run(context.Background(), newConfig(files, &out, server.URL())); err == nil {
			t.Error("Run() error = nil, want an error")
		}
		if !strings.HasSuffix(out.String(), "\nPlan cannot be applied.\n") {
			t.Errorf("got Out %q, want the whole change plan ending with Plan cannot be applied.", out.String())
		}
	})

	t.Run("write failure is returned", func(t *testing.T) {
		server := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
			repositoryRoute: githubtest.OK(githubtest.RepositoryBody{ID: 1}.JSON()),
		}))
		files := fstest.MapFS{"reco.yaml": {Data: []byte(configSource)}}
		cfg := newConfig(files, nil, server.URL())
		cfg.Out = failingWriter{}

		if err := Run(context.Background(), cfg); !errors.Is(err, errWrite) {
			t.Errorf("Run() error = %v, want %v", err, errWrite)
		}
	})

	t.Run("selection is passed to change planning", func(t *testing.T) {
		server := githubtest.NewServer(t, githubtest.Routes(t, nil))
		var out bytes.Buffer
		files := fstest.MapFS{"reco.yaml": {Data: []byte(configSource)}}
		cfg := newConfig(files, &out, server.URL())
		selection := mustReference(t, "owner/other")
		cfg.Selection = &selection

		if err := Run(context.Background(), cfg); err == nil {
			t.Fatal("Run() error = nil, want an error")
		}
		if len(server.Requests()) != 0 {
			t.Errorf("got requests %v, want none", server.RequestLines())
		}
	})

	t.Run("change planning failure is returned without writing to Out", func(t *testing.T) {
		server := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
			repositoryRoute: {Status: http.StatusNotFound},
		}))
		var out bytes.Buffer
		files := fstest.MapFS{"reco.yaml": {Data: []byte(configSource)}}

		err := Run(context.Background(), newConfig(files, &out, server.URL()))
		if !errors.Is(err, github.ErrNotFound) {
			t.Errorf("Run() error = %v, want %v", err, github.ErrNotFound)
		}
		if out.Len() != 0 {
			t.Errorf("got Out %q, want empty", out.String())
		}
	})
}

//trace:checks FR-006
//trace:checks FR-007
func TestNotApplicableError(t *testing.T) {
	zeta, alpha := mustReference(t, "owner/zeta"), mustReference(t, "owner/alpha")
	match := changeplan.SettingPlan{Key: setting.AllowAutoMerge, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(true), Result: changeplan.Match}
	applicable := changeplan.SettingPlan{Key: setting.AllowAutoMerge, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: changeplan.Mismatch, Applicability: changeplan.Applicable}
	notApplicable := changeplan.SettingPlan{
		Key: setting.SHAPinningRequired, Observed: github.Unavailable[setting.Value](github.UnavailableActionsDisabled), Desired: setting.BoolValue(true),
		Result: changeplan.NotComparable, Applicability: changeplan.NotApplicable, NotApplicableReason: changeplan.NotApplicableActionsRemainDisabled,
	}
	cases := []struct {
		name         string
		repositories []changeplan.RepositoryPlan
		want         string
	}{
		{
			name:         "plan without not-applicable changes has no error",
			repositories: []changeplan.RepositoryPlan{{Target: zeta, Settings: []changeplan.SettingPlan{match, applicable}}},
		},
		{
			name: "not-applicable settings in two repositories are all named in the change plan order",
			repositories: []changeplan.RepositoryPlan{
				{Target: zeta, Settings: []changeplan.SettingPlan{applicable, notApplicable}},
				{Target: alpha, Settings: []changeplan.SettingPlan{notApplicable}},
			},
			want: "plan cannot be applied: owner/zeta: actions.permissions.sha_pinning_required is not applicable; owner/alpha: actions.permissions.sha_pinning_required is not applicable",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := notApplicableError(changeplan.Plan{Repositories: tc.repositories})
			if tc.want == "" {
				if err != nil {
					t.Errorf("notApplicableError() = %v, want nil", err)
				}
				return
			}
			if err == nil || err.Error() != tc.want {
				t.Errorf("notApplicableError() = %v, want %q", err, tc.want)
			}
		})
	}
}
