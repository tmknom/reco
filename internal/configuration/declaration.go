package configuration

import (
	"fmt"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
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
	entries, err := d.mappingEntries(n, path, []string{"name"}, []string{"general", "actions"})
	if err != nil {
		return RepositoryDeclaration{}, err
	}
	generalNode, hasGeneral := entries["general"]
	actionsNode, hasActions := entries["actions"]
	if !hasGeneral && !hasActions {
		return RepositoryDeclaration{}, invalid(n, path, "must have at least one of general, actions")
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
	var desired DesiredState
	if hasGeneral {
		if desired.General, err = d.general(generalNode, path+".general"); err != nil {
			return RepositoryDeclaration{}, err
		}
	}
	if hasActions {
		if desired.ActionsPermissions, desired.WorkflowPermissions, desired.ActionsAccess, err = d.actions(actionsNode, path+".actions"); err != nil {
			return RepositoryDeclaration{}, err
		}
	}
	return RepositoryDeclaration{Reference: reference, Desired: desired}, nil
}

// actions は、Actions の階層のうち reco が受け付ける permissions、workflow、access だけを読む。
// どれも任意のキーだが、空のマッピングは拒否されるため、成功すればどれかは必ずある。
func (d document) actions(n *yaml.Node, path string) (ActionsPermissions, WorkflowPermissions, ActionsAccess, error) {
	entries, err := d.mappingEntries(n, path, nil, []string{"permissions", "workflow", "access"})
	if err != nil {
		return ActionsPermissions{}, WorkflowPermissions{}, ActionsAccess{}, err
	}
	var permissions ActionsPermissions
	if value, ok := entries["permissions"]; ok {
		if permissions, err = d.permissions(value, path+".permissions"); err != nil {
			return ActionsPermissions{}, WorkflowPermissions{}, ActionsAccess{}, err
		}
	}
	var workflow WorkflowPermissions
	if value, ok := entries["workflow"]; ok {
		if workflow, err = d.workflow(value, path+".workflow"); err != nil {
			return ActionsPermissions{}, WorkflowPermissions{}, ActionsAccess{}, err
		}
	}
	var access ActionsAccess
	if value, ok := entries["access"]; ok {
		if access, err = d.access(value, path+".access"); err != nil {
			return ActionsPermissions{}, WorkflowPermissions{}, ActionsAccess{}, err
		}
	}
	return permissions, workflow, access, nil
}

// permissions は、Actions の有効状態、Actions 許可ポリシー、選択方式の Actions 許可条件、sha_pinning_required の組み合わせを読む。
// 組み合わせの違反は各キーの値を見ずに拒否し、組み合わせの誤りを値の誤りより先に示す。
func (d document) permissions(n *yaml.Node, path string) (ActionsPermissions, error) {
	entries, err := d.mappingEntries(n, path, nil, []string{"enabled", "allowed_actions", "selected_actions", "sha_pinning_required"})
	if err != nil {
		return ActionsPermissions{}, err
	}
	enabledNode, hasEnabled := entries["enabled"]
	_, hasAllowed := entries["allowed_actions"]
	_, hasSelected := entries["selected_actions"]
	shaPinningNode, hasSHAPinning := entries["sha_pinning_required"]
	var permissions ActionsPermissions
	if hasEnabled {
		enabled, err := d.boolValue(enabledNode, path+".enabled")
		if err != nil {
			return ActionsPermissions{}, err
		}
		var declared []string
		for _, k := range []string{"allowed_actions", "selected_actions", "sha_pinning_required"} {
			if _, ok := entries[k]; ok {
				declared = append(declared, k)
			}
		}
		if !enabled && len(declared) > 0 {
			return ActionsPermissions{}, invalid(n, path, "enabled false cannot be declared with %s", strings.Join(declared, ", "))
		}
		// allowed_actions とともに宣言した enabled true は読み捨てる。ActionsPermissions.Enabled を参照。
		if !hasAllowed {
			permissions.Enabled = &enabled
		}
	}
	if hasAllowed || hasSelected {
		if permissions.AllowedActions, permissions.SelectedActions, err = d.actionsPolicy(n, path, entries); err != nil {
			return ActionsPermissions{}, err
		}
	}
	if hasSHAPinning {
		required, err := d.boolValue(shaPinningNode, path+".sha_pinning_required")
		if err != nil {
			return ActionsPermissions{}, err
		}
		permissions.SHAPinningRequired = &required
	}
	return permissions, nil
}

// actionsPolicy は、Actions 許可ポリシーと、それに従う選択方式の Actions 許可条件の組み合わせを読む。
// 許可条件は、ポリシーが selected のときだけ返し、それ以外は nil を返す。
func (d document) actionsPolicy(n *yaml.Node, path string, entries map[string]*yaml.Node) (*setting.AllowedActionsPolicy, *setting.SelectedActionsCriteria, error) {
	allowedNode, hasAllowed := entries["allowed_actions"]
	selectedNode, hasSelected := entries["selected_actions"]
	if !hasAllowed {
		return nil, nil, invalid(n, path, `selected_actions requires allowed_actions "selected"`)
	}
	policy, err := d.allowedActionsPolicyValue(allowedNode, path+".allowed_actions")
	if err != nil {
		return nil, nil, err
	}
	switch {
	case policy == setting.AllowedActionsSelected && !hasSelected:
		return nil, nil, invalid(n, path, `allowed_actions "selected" requires selected_actions with github_owned_allowed, verified_allowed, patterns_allowed`)
	case policy != setting.AllowedActionsSelected && hasSelected:
		return nil, nil, invalid(selectedNode, path+".selected_actions", `selected_actions is allowed only when allowed_actions is "selected"; got allowed_actions %q`, policy)
	case !hasSelected:
		return &policy, nil, nil
	}
	criteria, err := d.selectedActions(selectedNode, path+".selected_actions")
	if err != nil {
		return nil, nil, err
	}
	return &policy, &criteria, nil
}

// selectedActions は、3 項目がそろった選択方式の Actions 許可条件を読む。
// 一部の項目だけの宣言は、残りの項目の期待値が決まらないため拒否し、足りない項目をすべて示す。
func (d document) selectedActions(n *yaml.Node, path string) (setting.SelectedActionsCriteria, error) {
	keys := []string{"github_owned_allowed", "verified_allowed", "patterns_allowed"}
	entries, err := d.mappingEntries(n, path, nil, keys)
	if err != nil {
		return setting.SelectedActionsCriteria{}, err
	}
	var missing []string
	for _, k := range keys {
		if _, ok := entries[k]; !ok {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return setting.SelectedActionsCriteria{}, invalid(n, path, "missing %s; selected_actions requires all of %s", strings.Join(missing, ", "), strings.Join(keys, ", "))
	}
	githubOwned, err := d.boolValue(entries["github_owned_allowed"], path+".github_owned_allowed")
	if err != nil {
		return setting.SelectedActionsCriteria{}, err
	}
	verified, err := d.boolValue(entries["verified_allowed"], path+".verified_allowed")
	if err != nil {
		return setting.SelectedActionsCriteria{}, err
	}
	patterns, err := d.patternsAllowedValue(entries["patterns_allowed"], path+".patterns_allowed")
	if err != nil {
		return setting.SelectedActionsCriteria{}, err
	}
	return setting.SelectedActionsCriteria{GitHubOwnedAllowed: githubOwned, VerifiedAllowed: verified, PatternsAllowed: patterns}, nil
}

func (d document) workflow(n *yaml.Node, path string) (WorkflowPermissions, error) {
	entries, err := d.mappingEntries(n, path, nil, []string{"default_workflow_permissions", "can_approve_pull_request_reviews"})
	if err != nil {
		return WorkflowPermissions{}, err
	}
	var workflow WorkflowPermissions
	if value, ok := entries["default_workflow_permissions"]; ok {
		v, err := d.permissionLevelValue(value, path+".default_workflow_permissions")
		if err != nil {
			return WorkflowPermissions{}, err
		}
		workflow.DefaultWorkflowPermissions = &v
	}
	if value, ok := entries["can_approve_pull_request_reviews"]; ok {
		v, err := d.boolValue(value, path+".can_approve_pull_request_reviews")
		if err != nil {
			return WorkflowPermissions{}, err
		}
		workflow.CanApprovePullRequestReviews = &v
	}
	return workflow, nil
}

// access は、リポジトリ外のワークフローからのアクセスを読む。
// キーは access_level だけのため必須として渡す。空のマッピングは mappingEntries が拒否する。
func (d document) access(n *yaml.Node, path string) (ActionsAccess, error) {
	entries, err := d.mappingEntries(n, path, []string{"access_level"}, nil)
	if err != nil {
		return ActionsAccess{}, err
	}
	level, err := d.accessLevelValue(entries["access_level"], path+".access_level")
	if err != nil {
		return ActionsAccess{}, err
	}
	return ActionsAccess{AccessLevel: &level}, nil
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
