package configuration

import (
	"fmt"
	"slices"

	"go.yaml.in/yaml/v3"

	"github.com/tmknom/reco/internal/repository"
)

func (d document) declaration() (Declaration, error) {
	entries, err := d.mappingEntries(d.root, "root", []string{"repositories"}, nil)
	if err != nil {
		return Declaration{}, err
	}
	items, err := d.sequenceItems(entries["repositories"], "repositories")
	if err != nil {
		return Declaration{}, err
	}
	repositories := make([]RepositoryDeclaration, 0, len(items))
	for i, item := range items {
		path := fmt.Sprintf("repositories[%d]", i)
		r, err := d.repositoryDeclaration(item, path)
		if err != nil {
			return Declaration{}, err
		}
		if j := slices.IndexFunc(repositories, func(seen RepositoryDeclaration) bool { return seen.Reference.EqualFold(r.Reference) }); j >= 0 {
			return Declaration{}, invalid(item, path, "duplicate name %q; same as repositories[%d] (compared ignoring letter case)", r.Reference.String(), j)
		}
		repositories = append(repositories, r)
	}
	return Declaration{Repositories: repositories}, nil
}

func (d document) repositoryDeclaration(n *yaml.Node, path string) (RepositoryDeclaration, error) {
	entries, err := d.mappingEntries(n, path, []string{"name", "general"}, nil)
	if err != nil {
		return RepositoryDeclaration{}, err
	}
	namePath := path + ".name"
	name, err := d.stringValue(entries["name"], namePath)
	if err != nil {
		return RepositoryDeclaration{}, err
	}
	reference, err := repository.ParseReference(name)
	if err != nil {
		return RepositoryDeclaration{}, invalid(entries["name"], namePath, "%v; got %q", err, name)
	}
	general, err := d.general(entries["general"], path+".general")
	if err != nil {
		return RepositoryDeclaration{}, err
	}
	return RepositoryDeclaration{Reference: reference, Desired: DesiredState{General: general}}, nil
}

func (d document) general(n *yaml.Node, path string) (General, error) {
	entries, err := d.mappingEntries(n, path, nil, []string{"delete_branch_on_merge", "allow_auto_merge"})
	if err != nil {
		return General{}, err
	}
	var general General
	if value, ok := entries["delete_branch_on_merge"]; ok {
		v, err := d.boolValue(value, path+".delete_branch_on_merge")
		if err != nil {
			return General{}, err
		}
		general.DeleteBranchOnMerge = &v
	}
	if value, ok := entries["allow_auto_merge"]; ok {
		v, err := d.boolValue(value, path+".allow_auto_merge")
		if err != nil {
			return General{}, err
		}
		general.AllowAutoMerge = &v
	}
	return general, nil
}
