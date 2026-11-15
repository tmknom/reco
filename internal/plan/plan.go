// Package plan は、plan コマンドの処理の流れと変更計画の表示を持つ。
package plan

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/changeplandisplay"
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
// CLI 仕様は変更不要の対象を確認した件数でだけ示すため、変更予定のないリポジトリの計画には節を書かない。
// リポジトリの計画を持たない変更計画は changeplan が生成しない。何も確認していないのに No changes. と示さないよう、不変条件の破れとして panic する。
func writeChangePlan(w io.Writer, configPath string, p changeplan.Plan) error {
	if len(p.Repositories) == 0 {
		panic("plan: change plan has no repository plan")
	}
	var sections strings.Builder
	repositories, changes, notComparables := 0, 0, 0
	for _, r := range p.Repositories {
		c, n := r.Count(changeplan.Mismatch), r.Count(changeplan.NotComparable)
		if c+n == 0 {
			continue
		}
		repositories++
		changes += c
		notComparables += n
		fmt.Fprintf(&sections, "\nRepository: %s\n", r.Target)
		for _, s := range r.Settings {
			sections.WriteString(changeplandisplay.PlannedChangeText(s))
		}
	}
	if repositories == 0 {
		_, err := fmt.Fprintf(w, "No changes.\nRepositories checked: %d\n", len(p.Repositories))
		return err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Configuration: %s\n%s", configPath, sections.String())
	fmt.Fprintf(&b, "\nPlan: %s to update, %s, %s\n",
		pluralize(repositories, "repository", "repositories"),
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
