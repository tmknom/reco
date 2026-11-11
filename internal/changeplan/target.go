package changeplan

import (
	"fmt"
	"slices"

	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/repository"
)

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
