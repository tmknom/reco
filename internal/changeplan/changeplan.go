// Package changeplan は、設定宣言と観測結果を照合し、変更計画を生成する。
package changeplan

import (
	"context"
	"slices"

	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

// Plan は変更計画。
type Plan struct {
	// Repositories は、選択した対象リポジトリごとのリポジトリの計画。一つ以上を持ち、並びは設定宣言の並び。
	Repositories []RepositoryPlan
}

// RepositoryPlan は、一つの対象リポジトリについてのリポジトリの計画。
type RepositoryPlan struct {
	// Target は宣言の name。
	Target repository.Reference
	// RepositoryID は計画時の観測で得た、Target が指す実体の ID。承認されると、変更前提の同一性の基準になる。
	RepositoryID repository.ID
	// Settings は一致した設定も含む。変更予定とは Result で区別する。
	// 並びは CLI 仕様が定める変更予定の表示順。
	Settings []SettingPlan
}

// SettingPlan は、一つの管理設定についての計画。
// Result は Observed と Desired から決まる。判定は Result で行い、Observed は値と理由の取り出しに使う。
type SettingPlan struct {
	Key setting.Key
	// Observed は計画時の設定の観測結果。観測不能のときだけ比較不能になり、その理由が比較できない理由になる。
	Observed      github.SettingObservation[setting.Value]
	Desired       setting.Value
	Result        ReconciliationResult
	Applicability Applicability
	// SideEffects は変更予定に伴う副作用。副作用がなければ nil。
	SideEffects []SideEffect
}

// EnablesActions は、この変更予定が GitHub Actions を有効にするかを返す。
// Actions 許可ポリシーの期待値は Actions が有効であることを含むため、Actions が無効で観測不能だったポリシーの変更予定が当たる。
func (s SettingPlan) EnablesActions() bool {
	return s.Key == setting.AllowedActions && s.Result == NotComparable && s.Observed.UnavailableReason() == github.UnavailableActionsDisabled
}

// Count は、照合結果が result の設定の数を返す。
func (r RepositoryPlan) Count(result ReconciliationResult) int {
	n := 0
	for _, s := range r.Settings {
		if s.Result == result {
			n++
		}
	}
	return n
}

// IsPlannedChange は、この管理設定が変更予定 (不一致か比較不能の管理設定) かを返す。
// 変更予定かどうかを読む処理ごとに条件を組み立てると定義が食い違うため、ここだけで判定する。
func (s SettingPlan) IsPlannedChange() bool {
	return s.Result != Match
}

// HasPlannedChanges は、変更予定があるかを返す。
func (r RepositoryPlan) HasPlannedChanges() bool {
	return slices.ContainsFunc(r.Settings, SettingPlan.IsPlannedChange)
}

// Generate は、selection で選んだ対象の変更計画を生成する。selection が nil なら宣言された全リポジトリを選ぶ。
// 設定ファイルの読み込みと観測操作の失敗は、文言が設定ファイルのパスか対象名を含むため包み直さずに返す。
// 観測操作の失敗は比較不能にしない。
func Generate(ctx context.Context, client *github.Client, file configuration.File, selection *repository.Reference) (Plan, error) {
	declaration, err := file.Load()
	if err != nil {
		return Plan{}, err
	}
	targets, err := selectTargets(declaration, selection)
	if err != nil {
		return Plan{}, err
	}
	repositories := make([]RepositoryPlan, 0, len(targets))
	for _, target := range targets {
		r, err := generateRepositoryPlan(ctx, client, target)
		if err != nil {
			return Plan{}, err
		}
		repositories = append(repositories, r)
	}
	return Plan{Repositories: repositories}, nil
}

// generateRepositoryPlan は、一つの対象リポジトリを計画時に観測し、リポジトリの計画を生成する。
func generateRepositoryPlan(ctx context.Context, client *github.Client, target configuration.RepositoryDeclaration) (RepositoryPlan, error) {
	observed, err := client.ObserveRepository(ctx, target.Reference, managedAreas(target.Desired)...)
	if err != nil {
		return RepositoryPlan{}, err
	}
	settings := reconcile(target.Desired, observed)
	for i := range settings {
		settings[i].Applicability = judgeApplicability(settings[i])
		settings[i].SideEffects = sideEffects(settings[i], observed)
	}
	return RepositoryPlan{Target: target.Reference, RepositoryID: observed.RepositoryID, Settings: settings}, nil
}
