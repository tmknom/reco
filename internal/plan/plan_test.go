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

// TestPresentPlan は、apply が承認の前に行う論理的な設定変更の導出を、plan も通すことを確かめる。
// 導けない変更計画は Change Planning の不変条件が破れたときにしか生じないため、計画を直接組み立てて渡す。
//
//trace:checks FR-007
func TestPresentPlan(t *testing.T) {
	cfg := Config{Configuration: configuration.File{Path: "./reco.yaml"}, Out: &bytes.Buffer{}}
	plan := func(s changeplan.SettingPlan) changeplan.Plan {
		return changeplan.Plan{Repositories: []changeplan.RepositoryPlan{{Target: mustReference(t, "owner/repo"), RepositoryID: 1, Settings: []changeplan.SettingPlan{s}}}}
	}

	// 適用可能性の判断がない変更予定は、適用不能として挙がらず、導出で error になる
	t.Run("planned change without an applicability judgment returns the error of the derivation", func(t *testing.T) {
		unjudged := changeplan.SettingPlan{
			Unit: setting.Unit{Key: setting.AllowAutoMerge}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: changeplan.Mismatch,
		}
		if err := presentPlan(cfg, plan(unjudged)); err == nil {
			t.Error("presentPlan() error = nil, want an error")
		}
	})

	// apply がまだ実行しない既存の Ruleset の値の変更でも、plan は変更計画を示して終える (CLI 仕様の apply のエラー終了と plan の終了コード)
	t.Run("ruleset value update returns no error", func(t *testing.T) {
		name, err := setting.ParseRulesetName("protect")
		if err != nil {
			t.Fatalf("ParseRulesetName() error = %v", err)
		}
		ruleset := func(enforcement setting.RulesetEnforcement) setting.Value {
			return setting.RulesetValue(setting.Ruleset{
				Target: setting.RulesetTargetBranch, Enforcement: enforcement,
				Conditions:   setting.RulesetConditions{RefName: &setting.RefNameCondition{Include: []string{"~DEFAULT_BRANCH"}, Exclude: []string{}}},
				BypassActors: []setting.BypassActor{}, Rules: []setting.RulesetRule{setting.DeletionRule()},
			})
		}
		update := changeplan.SettingPlan{
			Unit: setting.Unit{Key: setting.RulesetResource, Ruleset: name}, Observed: github.Observed(ruleset(setting.RulesetEnforcementDisabled)),
			Desired: ruleset(setting.RulesetEnforcementActive), Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
		}
		if err := presentPlan(cfg, plan(update)); err != nil {
			t.Errorf("presentPlan() error = %v, want nil", err)
		}
	})

	// 遷移先が selected のポリシーの変更予定を伴わない、比較不能な選択方式の Actions 許可条件の変更予定は、
	// 表示はできるが、論理的な設定変更を導くと対になるポリシーの変更予定がなく panic する
	t.Run("planned change that cannot form a logical change panics", func(t *testing.T) {
		criteria := changeplan.SettingPlan{
			Unit: setting.Unit{Key: setting.SelectedActions}, Observed: github.Unavailable[setting.Value](github.UnavailableNotSelected),
			Desired: setting.SelectedActionsCriteriaValue(setting.SelectedActionsCriteria{VerifiedAllowed: true}), Result: changeplan.NotComparable, Applicability: changeplan.Applicable(),
		}
		defer func() {
			if recover() == nil {
				t.Error("presentPlan() did not panic")
			}
		}()
		_ = presentPlan(cfg, plan(criteria))
	})
}

func mustReference(t *testing.T, name string) repository.Reference {
	t.Helper()
	reference, err := repository.ParseReference(name)
	if err != nil {
		t.Fatalf("ParseReference(%q) error = %v", name, err)
	}
	return reference
}
