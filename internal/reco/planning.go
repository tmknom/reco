package reco

import (
	"context"
	"fmt"
	"slices"

	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
)

// changePlan は、対象リポジトリと、管理範囲内の設定ごとの計画をまとめた変更計画。
type changePlan struct {
	// target は宣言の name。
	target       repository.Reference
	repositoryID int64
	// settings は一致した設定も含む。変更予定とは result で区別する。
	settings []settingPlan
}

type settingPlan struct {
	// setting は YAML パスで表した設定。
	setting string
	current bool
	desired bool
	result  reconciliationResult
	// applicable は result が mismatch か notComparable のときの適用可能性。match では判断せず false。
	applicable bool
}

func (p changePlan) count(r reconciliationResult) int {
	n := 0
	for _, s := range p.settings {
		if s.result == r {
			n++
		}
	}
	return n
}

// 観測操作の失敗は、比較不能にせず、文言が対象名を含むため包み直さずに返す。
func planChanges(ctx context.Context, client *github.Client, d configuration.Declaration, selection *repository.Reference) (changePlan, error) {
	target, err := selectTarget(d, selection)
	if err != nil {
		return changePlan{}, err
	}
	observed, err := client.FetchRepository(ctx, target.Reference.Owner, target.Reference.Repo)
	if err != nil {
		return changePlan{}, err
	}
	settings := reconcile(target.General, observed.General)
	for i := range settings {
		settings[i].applicable = isApplicable(settings[i].result)
	}
	return changePlan{target: target.Reference, repositoryID: observed.ID, settings: settings}, nil
}

// 選択結果が一つに定まらなければエラーにする。一度に一つのリポジトリしか処理しないため。
func selectTarget(d configuration.Declaration, selection *repository.Reference) (configuration.RepositoryDeclaration, error) {
	if selection == nil {
		if len(d.Repositories) != 1 {
			return configuration.RepositoryDeclaration{}, fmt.Errorf("%d repositories are declared; select one repository (only one can be processed at a time)", len(d.Repositories))
		}
		return d.Repositories[0], nil
	}
	i := slices.IndexFunc(d.Repositories, func(r configuration.RepositoryDeclaration) bool {
		return r.Reference.EqualFold(*selection)
	})
	if i < 0 {
		return configuration.RepositoryDeclaration{}, fmt.Errorf("repository %s is not declared", selection)
	}
	return d.Repositories[i], nil
}

// isApplicable は、照合結果の設定を安全に適用できるかを返す。
// General は宣言された設定だけを部分更新するため、不一致は追加の条件なしに適用できる。
// 条件を確認できない比較不能は、適用不能として扱う。
func isApplicable(r reconciliationResult) bool {
	return r == mismatch
}
