package changeplan

import (
	"fmt"
	"slices"

	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/repository"
)

// selectTargets は、対象リポジトリにするリポジトリごとの宣言を設定宣言の並びで返す。
func selectTargets(d configuration.Declaration, selection *repository.Reference) ([]configuration.RepositoryDeclaration, error) {
	if selection == nil {
		return d.Repositories, nil
	}
	i := slices.IndexFunc(d.Repositories, func(r configuration.RepositoryDeclaration) bool {
		return r.Reference.EqualFold(*selection)
	})
	if i < 0 {
		return nil, fmt.Errorf("repository %s is not declared", selection)
	}
	return d.Repositories[i : i+1], nil
}
