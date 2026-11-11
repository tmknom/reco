// Package changeplan は、変更計画の生成を持つ。
package changeplan

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
)

// Plan は、対象リポジトリと、管理範囲内の設定ごとの計画をまとめた変更計画。
type Plan struct {
	// Target は宣言の name。
	Target       repository.Reference
	RepositoryID int64
	// Settings は一致した設定も含む。変更予定とは Result で区別する。
	Settings []SettingPlan
}

// SettingPlan は、一つの管理設定についての計画。
type SettingPlan struct {
	// Setting は YAML パスで表した設定。
	Setting       string
	Current       bool
	Desired       bool
	Result        ReconciliationResult
	Applicability Applicability
}

// Count は、照合結果が r の設定の数を返す。
func (p Plan) Count(r ReconciliationResult) int {
	n := 0
	for _, s := range p.Settings {
		if s.Result == r {
			n++
		}
	}
	return n
}

// ConfigurationFile は、設定宣言を読み込む設定ファイル。
type ConfigurationFile struct {
	// Dir は設定ファイルのあるディレクトリ、Name はその中のファイル名。
	// fs.FS の名前は先頭の / や .. を含められないため、パスをディレクトリと名前に分けて受ける。
	Dir  fs.FS
	Name string
	// Path は利用者が指定したままの設定ファイルのパス。エラー文言に使う。
	Path string
}

// Generate は、selection で選んだ対象の変更計画を生成する。
// 観測操作の失敗は、比較不能にせず、文言が対象名を含むため包み直さずに返す。
func Generate(ctx context.Context, client *github.Client, file ConfigurationFile, selection *repository.Reference) (Plan, error) {
	declaration, err := configuration.Load(file.Dir, file.Name)
	if err != nil {
		return Plan{}, fmt.Errorf("load configuration %s: %w", file.Path, err)
	}
	target, err := selectTarget(declaration, selection)
	if err != nil {
		return Plan{}, err
	}
	observed, err := client.FetchRepository(ctx, target.Reference)
	if err != nil {
		return Plan{}, err
	}
	settings := reconcile(target.General, observed.General)
	for i := range settings {
		settings[i].Applicability = judgeApplicability(settings[i].Result)
	}
	return Plan{Target: target.Reference, RepositoryID: observed.ID, Settings: settings}, nil
}
