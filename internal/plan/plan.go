// Package plan は、plan コマンドの処理の流れと変更計画の表示を持つ。
package plan

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/display"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
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
		writePlannedChange(&b, s)
	}
	fmt.Fprintf(&b, "\nPlan: %s to update, %s, %s\n",
		pluralize(1, "repository", "repositories"),
		pluralize(changes, "known change", "known changes"),
		pluralize(notComparables, "not-comparable setting", "not-comparable settings"))
	_, err := io.WriteString(w, b.String())
	return err
}

// writePlannedChange は、変更予定を CLI 仕様の確認済みの差分か比較不能な設定予定の形で書き、副作用をその下に添える。一致は書かない。
// 比較不能の現在値は確認できていないため、確認済みの差分の形 (現在値 -> 期待値) で書かない。
func writePlannedChange(b *strings.Builder, s changeplan.SettingPlan) {
	switch s.Result {
	case changeplan.Match:
		return
	case changeplan.Mismatch:
		observed, _ := s.Observed.Value()
		if s.Key == setting.SelectedActions {
			// 3項目を並べた値は長く、1行では -> を見つけにくいため、現在値と期待値を別の行に書く
			fmt.Fprintf(b, "  %s:\n    %s\n    -> %s\n", s.Key, observed.String(), s.Desired.String())
		} else {
			fmt.Fprintf(b, "  %s: %s -> %s\n", s.Key, observed.String(), s.Desired.String())
		}
	case changeplan.NotComparable:
		desired := s.Desired.String()
		if s.EnablesActions() {
			desired += " (enables GitHub Actions)"
		}
		fmt.Fprintf(b, "  %s\n    Current: %s\n    Desired: %s\n", s.Key, display.UnavailableText(s.Observed.UnavailableReason()), desired)
	}
	for _, effect := range s.SideEffects {
		fmt.Fprintf(b, "    Side effect: %s\n", sideEffectText(effect))
	}
}

// sideEffectText は、副作用を CLI 仕様の Side effect: の文言で返す。
// 副作用は、Actions 許可ポリシーの変更で選択方式の Actions 許可条件が失われることだけのため、文言はその設定に限り、
// ほかの設定の副作用は誤った文言で黙って示さないよう、不変条件の破れとして panic する。
// 失われる値を観測できなかったときは、値を推測せずに失われる可能性だけを示す。
func sideEffectText(effect changeplan.SideEffect) string {
	if effect.Key != setting.SelectedActions {
		panic("plan: no side effect wording for " + string(effect.Key))
	}
	if lost, ok := effect.Observed.Value(); ok {
		return "GitHub discards the current selected_actions (" + lost.String() + ")"
	}
	return `GitHub may discard selected_actions if allowed_actions was "selected" before GitHub Actions was disabled (current values are unavailable)`
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}
