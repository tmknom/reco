package configuration

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

func (d document) declaration() (Declaration, error) {
	entries, err := d.mappingEntries(d.root, "root", "repositories")
	if err != nil {
		return Declaration{}, err
	}
	items, err := d.sequenceItems(entries["repositories"], "repositories")
	if err != nil {
		return Declaration{}, err
	}
	repositories := make([]RepositoryDeclaration, 0, len(items))
	// GitHub は所有者名とリポジトリ名を大文字小文字を区別せずに扱うため、小文字にそろえた名前で重複を判定する。
	// 名前は ASCII に限られているため、strings.ToLower は ASCII 英字だけの同一視になる。
	seen := make(map[string]int, len(items))
	for i, item := range items {
		path := fmt.Sprintf("repositories[%d]", i)
		r, err := d.repositoryDeclaration(item, path)
		if err != nil {
			return Declaration{}, err
		}
		name := r.Reference.Owner + "/" + r.Reference.Repo
		key := strings.ToLower(name)
		if j, ok := seen[key]; ok {
			return Declaration{}, invalid(item, path, "duplicate name %q; same as repositories[%d] (compared ignoring letter case)", name, j)
		}
		seen[key] = i
		repositories = append(repositories, r)
	}
	return Declaration{Repositories: repositories}, nil
}

func (d document) repositoryDeclaration(n *yaml.Node, path string) (RepositoryDeclaration, error) {
	entries, err := d.mappingEntries(n, path, "name", "general")
	if err != nil {
		return RepositoryDeclaration{}, err
	}
	namePath := path + ".name"
	name, err := d.stringValue(entries["name"], namePath)
	if err != nil {
		return RepositoryDeclaration{}, err
	}
	reference, err := parseRepositoryReference(name)
	if err != nil {
		return RepositoryDeclaration{}, invalid(entries["name"], namePath, "%v; got %q", err, name)
	}
	general, err := d.general(entries["general"], path+".general")
	if err != nil {
		return RepositoryDeclaration{}, err
	}
	return RepositoryDeclaration{Reference: reference, General: general}, nil
}

func (d document) general(n *yaml.Node, path string) (General, error) {
	entries, err := d.mappingEntries(n, path, "allow_auto_merge")
	if err != nil {
		return General{}, err
	}
	allowAutoMerge, err := d.boolValue(entries["allow_auto_merge"], path+".allow_auto_merge")
	if err != nil {
		return General{}, err
	}
	return General{AllowAutoMerge: &allowAutoMerge}, nil
}
