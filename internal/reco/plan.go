package reco

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
)

// PlanConfig は Plan の入力。
type PlanConfig struct {
	Token string
	// ConfigDir は設定ファイルのあるディレクトリ、ConfigName はその中のファイル名。
	// fs.FS の名前は先頭の / や .. を含められないため、パスをディレクトリと名前に分けて受ける。
	ConfigDir  fs.FS
	ConfigName string
	// ConfigPath は利用者が指定したままの設定ファイルのパス。表示とエラー文言に使う。
	ConfigPath string
	// Selection は利用者が明示したリポジトリ指定。nil なら宣言された全リポジトリを対象にする。
	Selection     *repository.Reference
	Out           io.Writer
	GitHubOptions []github.Option
}

// Plan は設定宣言を検証し、選択した対象の観測結果と照合して、変更計画を Out に書く。
func Plan(ctx context.Context, cfg PlanConfig) error {
	declaration, err := configuration.Load(cfg.ConfigDir, cfg.ConfigName)
	if err != nil {
		return fmt.Errorf("load configuration %s: %w", cfg.ConfigPath, err)
	}
	client := github.New(cfg.Token, cfg.GitHubOptions...)
	plan, err := planChanges(ctx, client, declaration, cfg.Selection)
	if err != nil {
		return err
	}
	return writeChangePlan(cfg.Out, cfg.ConfigPath, plan)
}

// writeChangePlan は変更計画を CLI 仕様の plan の形式で書く。
// 変更計画は対象を一つだけ持つため、Repositories checked と repository to update の数は 1 になる。
func writeChangePlan(w io.Writer, configPath string, p changePlan) error {
	changes := p.count(mismatch)
	notComparables := p.count(notComparable)
	if changes+notComparables == 0 {
		_, err := io.WriteString(w, "No changes.\nRepositories checked: 1\n")
		return err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Configuration: %s\n\nRepository: %s\n", configPath, p.target)
	for _, s := range p.settings {
		if s.result == mismatch {
			fmt.Fprintf(&b, "  %s: %t -> %t\n", s.setting, s.current, s.desired)
		}
	}
	fmt.Fprintf(&b, "\nPlan: %s to update, %s, %s\n",
		pluralize(1, "repository", "repositories"),
		pluralize(changes, "known change", "known changes"),
		pluralize(notComparables, "not-comparable setting", "not-comparable settings"))
	_, err := io.WriteString(w, b.String())
	return err
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}
