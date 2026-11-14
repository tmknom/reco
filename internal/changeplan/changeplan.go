// Package changeplan は、設定宣言と観測結果を照合し、変更計画を生成する。
package changeplan

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

// Plan は変更計画。対象の選択が一つのリポジトリに限るため、リポジトリの計画を兼ねる。
type Plan struct {
	// Target は宣言の name。
	Target repository.Reference
	// RepositoryID は計画時の観測で得た、Target が指す実体の ID。承認されると、変更前提の同一性の基準になる。
	RepositoryID repository.ID
	// Settings は一致した設定も含む。変更予定とは Result で区別する。
	// 並びは CLI 仕様が定める変更予定の表示順。
	Settings []SettingPlan
}

// SettingPlan は、一つの管理設定についての計画。
type SettingPlan struct {
	Key setting.Key
	// Observed は計画時の観測値。比較不能の設定では値がない。
	Observed      setting.Value
	Desired       setting.Value
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
	observed, err := client.ObserveRepository(ctx, target.Reference, managedAreas(target.Desired)...)
	if err != nil {
		return Plan{}, err
	}
	settings := reconcile(target.Desired, observed)
	for i := range settings {
		settings[i].Applicability = judgeApplicability(settings[i].Result)
	}
	return Plan{Target: target.Reference, RepositoryID: observed.RepositoryID, Settings: settings}, nil
}
