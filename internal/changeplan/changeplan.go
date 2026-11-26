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
type SettingPlan struct {
	// Unit は、この計画の管理の単位。Environment と Ruleset の名前の綴りは宣言のまま。
	Unit setting.Unit
	// Observed は計画時の設定の観測結果。関連リソースそのものの計画では、存在の観測結果である。
	// 観測不能のときだけ比較不能になり、その理由が比較できない理由になる。
	// 省略できる項目を省略した管理設定では、その項目を除いた値である。
	Observed github.SettingObservation[setting.Value]
	// EnvironmentIdentity は、Environment そのものの計画で、計画時に存在を観測した Environment の ID。承認されると変更前提の同一性の基準になる。
	// ほかの計画と、存在しない Environment の計画ではゼロ値。
	EnvironmentIdentity EnvironmentIdentity
	// RulesetIdentity は、Ruleset そのものの計画で、計画時に存在を観測した Ruleset の ID。承認されると変更前提の同一性の基準になる。
	// ほかの計画と、存在しない Ruleset の計画ではゼロ値。
	RulesetIdentity RulesetIdentity
	Desired         setting.Value
	Result          ReconciliationResult
	Applicability   Applicability
	// SideEffects は変更予定に伴う副作用。副作用がなければ nil。
	SideEffects []SideEffect
	// SettingsExpectedUnaffected は、副作用を示さなかった設定。変更前提にだけ含める。当たらなければ nil。
	// 設定の識別を持たない Environment の自己承認の禁止は並べず、requiredReviewersChangeOf が担当者の集合の基準に伴わせる。
	SettingsExpectedUnaffected []SettingExpectedUnaffected
	// ResumedSettings は、Actions を有効にする変更予定に添える再開する設定。当たらなければ nil。
	ResumedSettings []setting.Key
	// UninterpretedParts は、値が解釈しない部分を含むことを理由に比較不能な計画でだけ、その解釈しなかったものを観測の並びで持つ。ほかの計画では nil。
	// 比較できない理由に伴うもので、適用可能性の判断の産物ではないため、Applicability ではなくここに置く。
	UninterpretedParts []github.UninterpretedPart
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
		if s.Applicability.IsNotApplicable() {
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

// IsConfirmedCreation は、この変更予定が確認済みの差分の作成 (存在しないと確かめた関連リソースの不一致) かを返す。
// 属する先が存在しないために比較不能で、先に属する先を作成する作成は含まない。
// 作成かどうかを読む処理ごとに条件を組み立てると、存在しないことの宣言を受け付ける種類で削除を作成と取り違えるため、ここだけで判定する。
// 存在しないことは関連リソースの観測値にだけ現れるため、Key で関連リソースに限らない。
// 不一致は観測値があるときにだけ生じるため、観測値のない不一致は Equal が不変条件の破れとして panic する。
func (s SettingPlan) IsConfirmedCreation() bool {
	if s.Result != Mismatch {
		return false
	}
	observed, _ := s.Observed.Value()
	return observed.Equal(setting.Absent())
}

// IsEnvironmentCreation は、この変更予定が Environment の作成かを返す。
// Environment はリポジトリに属するため、その作成は常に確認済みの差分である。
func (s SettingPlan) IsEnvironmentCreation() bool {
	return s.Unit.Key == setting.EnvironmentResource && s.IsConfirmedCreation()
}

// IsPlannedChange は、この管理設定が変更予定 (不一致か比較不能の管理設定) かを返す。
// 変更予定かどうかを読む処理ごとに条件を組み立てると定義が食い違うため、ここだけで判定する。
// 照合結果のない計画は、変更不要とも変更予定とも決められないため、不変条件の破れとして panic する。
func (s SettingPlan) IsPlannedChange() bool {
	if s.Result == 0 {
		panic(fmt.Sprintf("changeplan: %q has no reconciliation result", s.Unit.Key))
	}
	return s.Result != Match
}

// generateRepositoryPlan は、一つの対象リポジトリを計画時に観測し、リポジトリの計画を生成する。
func generateRepositoryPlan(ctx context.Context, client *github.Client, target configuration.RepositoryDeclaration) (RepositoryPlan, error) {
	named := github.NamedResources{Environments: managedEnvironments(target.DesiredState), Rulesets: managedRulesets(target.DesiredState)}
	observed, err := client.ObserveRepositoryForNamedResources(ctx, target.Reference, named, managedAreas(target.DesiredState)...)
	if err != nil {
		return RepositoryPlan{}, err
	}
	settings := reconcile(target.DesiredState, observed)
	for i := range settings {
		settings[i].Applicability = judgeApplicability(settings[i], settings, observed)
		settings[i].SideEffects, settings[i].SettingsExpectedUnaffected = sideEffects(settings[i], observed, settings)
		settings[i].ResumedSettings = resumedSettings(settings[i], settings)
	}
	return RepositoryPlan{Target: target.Reference, RepositoryID: observed.RepositoryID, Settings: settings}, nil
}
