// Package plan は、plan コマンドの処理の流れと変更計画の表示を持つ。
package plan

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
)

// Config は Run の入力。
type Config struct {
	Token string
	// ConfigDir は設定ファイルのあるディレクトリ、ConfigName はその中のファイル名。
	ConfigDir  fs.FS
	ConfigName string
	// ConfigPath は利用者が指定したままの設定ファイルのパス。表示とエラー文言に使う。
	ConfigPath string
	// Selection は利用者が明示したリポジトリ指定。明示しなければ nil。
	Selection     *repository.Reference
	Out           io.Writer
	GitHubOptions []github.Option
}

// Run は変更計画を生成し、Out に書く。GitHub の設定は更新しない。
func Run(ctx context.Context, cfg Config) error {
	client := github.New(cfg.Token, cfg.GitHubOptions...)
	file := changeplan.ConfigurationFile{Dir: cfg.ConfigDir, Name: cfg.ConfigName, Path: cfg.ConfigPath}
	p, err := changeplan.Generate(ctx, client, file, cfg.Selection)
	if err != nil {
		return err
	}
	return writeChangePlan(cfg.Out, cfg.ConfigPath, p)
}

// writeChangePlan は変更計画を CLI 仕様の plan の形式で書く。
// 変更計画は対象を一つだけ持つため、Repositories checked と repository to update の数は 1 になる。
func writeChangePlan(w io.Writer, configPath string, p changeplan.Plan) error {
	changes := p.Count(changeplan.Mismatch)
	notComparables := p.Count(changeplan.NotComparable)
	if changes+notComparables == 0 {
		_, err := io.WriteString(w, "No changes.\nRepositories checked: 1\n")
		return err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Configuration: %s\n\nRepository: %s\n", configPath, p.Target)
	for _, s := range p.Settings {
		if s.Result == changeplan.Mismatch {
			fmt.Fprintf(&b, "  %s: %s -> %s\n", s.Key, s.Observed.String(), s.Desired.String())
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
