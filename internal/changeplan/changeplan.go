// Package changeplan は、設定宣言と観測結果を照合し、変更計画を生成する。
package changeplan

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

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
// Environment そのものの計画は値を持たないため、Observed と Desired をゼロ値にし、ObservedEnvironment から Result を決める。
type SettingPlan struct {
	// Unit は、この計画の管理の単位。Environment の名前の綴りは宣言のまま。
	Unit setting.Unit
	// ObservedEnvironment は、Environment そのものの計画だけが持つ、計画時の関連リソースの観測結果。
	ObservedEnvironment github.EnvironmentObservation
	// Observed は計画時の設定の観測結果。観測不能のときだけ比較不能になり、その理由が比較できない理由になる。
	Observed      github.SettingObservation[setting.Value]
	Desired       setting.Value
	Result        ReconciliationResult
	Applicability Applicability
	// NotApplicableReason は、Applicability が NotApplicable のときだけ定まる。
	NotApplicableReason NotApplicableReason
	// SideEffects は変更予定に伴う副作用。副作用がなければ nil。
	SideEffects []SideEffect
	// SettingsWithoutSideEffect は、副作用を示さなかった設定。変更前提にだけ含める。当たらなければ nil。
	SettingsWithoutSideEffect []SettingWithoutSideEffect
	// ResumedSettings は、Actions を有効にする変更予定に添える再開する設定。当たらなければ nil。
	ResumedSettings []setting.Key
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

// NotApplicableError は、変更計画の適用不能な変更予定を、変更計画の並びで対象と設定を挙げた error を返す。なければ nil を返す。
// 理由の文言は変更計画の表示が持ち、利用者は表示の Applicability: の行で読むため、error には含めない。
func (p Plan) NotApplicableError() error {
	var found []string
	for _, r := range p.Repositories {
		for _, s := range r.NotApplicableChanges() {
			found = append(found, fmt.Sprintf("%s: %s is not applicable", r.Target, s.Unit.String()))
		}
	}
	if len(found) == 0 {
		return nil
	}
	return errors.New("plan cannot be applied: " + strings.Join(found, "; "))
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

// HasPlannedChanges は、変更予定があるかを返す。
func (r RepositoryPlan) HasPlannedChanges() bool {
	return slices.ContainsFunc(r.Settings, SettingPlan.IsPlannedChange)
}

// NotApplicableChanges は、適用不能な変更予定を計画の並びで返す。なければ nil を返す。
// 適用不能な変更予定を挙げる処理ごとに走査を組み立てると挙げる対象が食い違うため、挙げるときはここで取り出す。
func (r RepositoryPlan) NotApplicableChanges() []SettingPlan {
	var found []SettingPlan
	for _, s := range r.Settings {
		if s.Applicability == NotApplicable {
			found = append(found, s)
		}
	}
	return found
}

// EnablesActionsByPolicy は、この変更予定が Actions 許可ポリシーの変更として GitHub Actions を有効にするかを返す。
// Actions 許可ポリシーの期待値は Actions が有効であることを含むため、Actions が無効で観測不能だったポリシーの変更予定が当たる。
func (s SettingPlan) EnablesActionsByPolicy() bool {
	return s.Unit.Key == setting.AllowedActions && s.Result == NotComparable && s.Observed.UnavailableReason() == github.UnavailableActionsDisabled
}

// IsCreation は、この変更予定が Environment の作成かを返す。
func (s SettingPlan) IsCreation() bool {
	return s.Unit.Key == setting.EnvironmentResource && s.Result == Mismatch
}

// UnavailableReason は、比較不能な変更予定の観測不能の理由を返す。
// Environment そのものの計画は関連リソースの観測結果から、ほかは設定の観測結果から読む。どちらも観測不能でなければ panic する。
func (s SettingPlan) UnavailableReason() github.UnavailableReason {
	if s.Unit.Key == setting.EnvironmentResource {
		return s.ObservedEnvironment.UnavailableReason()
	}
	return s.Observed.UnavailableReason()
}

// IsPlannedChange は、この管理設定が変更予定 (不一致か比較不能の管理設定) かを返す。
// 変更予定かどうかを読む処理ごとに条件を組み立てると定義が食い違うため、ここだけで判定する。
func (s SettingPlan) IsPlannedChange() bool {
	return s.Result != Match
}

// generateRepositoryPlan は、一つの対象リポジトリを計画時に観測し、リポジトリの計画を生成する。
func generateRepositoryPlan(ctx context.Context, client *github.Client, target configuration.RepositoryDeclaration) (RepositoryPlan, error) {
	observed, err := client.ObserveRepository(ctx, target.Reference, managedAreas(target.Desired)...)
	if err != nil {
		return RepositoryPlan{}, err
	}
	settings := reconcile(target.Desired, observed)
	for i := range settings {
		settings[i].Applicability, settings[i].NotApplicableReason = judgeApplicability(settings[i], settings, observed)
		settings[i].SideEffects, settings[i].SettingsWithoutSideEffect = sideEffects(settings[i], observed, settings)
		settings[i].ResumedSettings = resumedSettings(settings[i], settings)
	}
	return RepositoryPlan{Target: target.Reference, RepositoryID: observed.RepositoryID, Settings: settings}, nil
}
