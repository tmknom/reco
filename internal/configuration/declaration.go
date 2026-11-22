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
	entries, err := d.mappingEntries(n, path, []string{"name"}, []string{"general", "actions", "environments", "security"})
	if err != nil {
		return RepositoryDeclaration{}, err
	}
	generalNode, hasGeneral := entries["general"]
	actionsNode, hasActions := entries["actions"]
	environmentsNode, hasEnvironments := entries["environments"]
	securityNode, hasSecurity := entries["security"]
	if !hasGeneral && !hasActions && !hasEnvironments && !hasSecurity {
		return RepositoryDeclaration{}, invalid(n, path, "must have at least one of general, actions, environments, security")
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
		if desired.ActionsPermissions, desired.WorkflowPermissions, desired.ActionsAccess, desired.RepositoryVariables, err = d.actions(actionsNode, path+".actions"); err != nil {
			return RepositoryDeclaration{}, err
		}
	}
	if hasEnvironments {
		if desired.Environments, err = d.environments(environmentsNode, path+".environments"); err != nil {
			return RepositoryDeclaration{}, err
		}
	}
	if hasSecurity {
		if desired.Dependabot, desired.SecretScanning, desired.PrivateVulnerabilityReporting, err = d.security(securityNode, path+".security"); err != nil {
			return RepositoryDeclaration{}, err
		}
	}
	return RepositoryDeclaration{Reference: reference, Desired: desired}, nil
}

// environments は、Environment の宣言の並びを宣言の順のまま読む。
// 同じ Environment とみなす名前の宣言は一つに限る。GitHub は名前の英字の大小を区別しないため、大小だけが違う名前も重複にする。
func (d document) environments(n *yaml.Node, path string) ([]EnvironmentDeclaration, error) {
	items, err := d.sequenceItems(n, path)
	if err != nil {
		return nil, err
	}
	declarations := make([]EnvironmentDeclaration, 0, len(items))
	for i, item := range items {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		declaration, err := d.environment(item, itemPath)
		if err != nil {
			return nil, err
		}
		if j := slices.IndexFunc(declarations, func(seen EnvironmentDeclaration) bool { return seen.Name.EqualFold(declaration.Name) }); j >= 0 {
			return nil, invalid(item, itemPath, "duplicate name %q; same as %s[%d] (compared ignoring letter case)", declaration.Name.String(), path, j)
		}
		declarations = append(declarations, declaration)
	}
	return declarations, nil
}

func (d document) environment(n *yaml.Node, path string) (EnvironmentDeclaration, error) {
	entries, err := d.mappingEntries(n, path, []string{"name"}, []string{"deployment_branch_policy", "variables"})
	if err != nil {
		return EnvironmentDeclaration{}, err
	}
	namePath := path + ".name"
	name, err := d.stringValue(entries["name"], namePath)
	if err != nil {
		return EnvironmentDeclaration{}, err
	}
	environmentName, err := setting.ParseEnvironmentName(name)
	if err != nil {
		return EnvironmentDeclaration{}, invalid(entries["name"], namePath, "%v; got %q", err, name)
	}
	declaration := EnvironmentDeclaration{Name: environmentName}
	if value, ok := entries["deployment_branch_policy"]; ok {
		policy, err := d.deploymentBranchPolicy(value, path+".deployment_branch_policy")
		if err != nil {
			return EnvironmentDeclaration{}, err
		}
		declaration.DeploymentBranchPolicy = &policy
	}
	if value, ok := entries["variables"]; ok {
		if declaration.Variables, err = d.variables(value, path+".variables"); err != nil {
			return EnvironmentDeclaration{}, err
		}
	}
	return declaration, nil
}

// deploymentBranchPolicy は、デプロイ元制限の方式と、方式が selected のときの個別パターンの一覧を読む。
func (d document) deploymentBranchPolicy(n *yaml.Node, path string) (setting.DeploymentBranchPolicy, error) {
	entries, err := d.mappingEntries(n, path, []string{"mode"}, []string{"patterns"})
	if err != nil {
		return setting.DeploymentBranchPolicy{}, err
	}
	modePath := path + ".mode"
	s, err := d.stringValue(entries["mode"], modePath)
	if err != nil {
		return setting.DeploymentBranchPolicy{}, err
	}
	mode, ok := setting.ParseDeploymentBranchPolicyMode(s)
	if !ok {
		return setting.DeploymentBranchPolicy{}, invalid(entries["mode"], modePath, "must be all, protected_branches, or selected; got %q", s)
	}
	patternsNode, hasPatterns := entries["patterns"]
	switch {
	case mode == setting.DeploymentBranchPolicySelected && !hasPatterns:
		return setting.DeploymentBranchPolicy{}, invalid(n, path, `mode "selected" requires patterns`)
	case mode != setting.DeploymentBranchPolicySelected && hasPatterns:
		return setting.DeploymentBranchPolicy{}, invalid(patternsNode, path+".patterns", `patterns is allowed only when mode is "selected"; got mode %q`, mode)
	case !hasPatterns:
		return setting.DeploymentBranchPolicy{Mode: mode}, nil
	}
	patterns, err := d.deploymentPatterns(patternsNode, path+".patterns")
	if err != nil {
		return setting.DeploymentBranchPolicy{}, err
	}
	return setting.DeploymentBranchPolicy{Mode: mode, Patterns: patterns}, nil
}

// deploymentPatterns は、個別パターンの一覧を宣言の順のまま返す。
// 空のシーケンスは「個別パターンなし」の期待値で、省略と区別するため、空でも nil にしない。
func (d document) deploymentPatterns(n *yaml.Node, path string) ([]setting.DeploymentPattern, error) {
	items, err := d.sequenceContent(n, path)
	if err != nil {
		return nil, err
	}
	patterns := make([]setting.DeploymentPattern, 0, len(items))
	for i, item := range items {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		pattern, err := d.deploymentPattern(item, itemPath)
		if err != nil {
			return nil, err
		}
		if j := slices.Index(patterns, pattern); j >= 0 {
			return nil, invalid(item, itemPath, "duplicate pattern %s %q; same as %s[%d]", pattern.Type, pattern.Name, path, j)
		}
		patterns = append(patterns, pattern)
	}
	return patterns, nil
}

// deploymentPattern は、種類と名前の組である個別パターン 1 件を読む。
func (d document) deploymentPattern(n *yaml.Node, path string) (setting.DeploymentPattern, error) {
	entries, err := d.mappingEntries(n, path, []string{"type", "name"}, nil)
	if err != nil {
		return setting.DeploymentPattern{}, err
	}
	typePath := path + ".type"
	typeString, err := d.stringValue(entries["type"], typePath)
	if err != nil {
		return setting.DeploymentPattern{}, err
	}
	patternType, ok := setting.ParseDeploymentPatternType(typeString)
	if !ok {
		return setting.DeploymentPattern{}, invalid(entries["type"], typePath, "must be branch or tag; got %q", typeString)
	}
	namePath := path + ".name"
	name, err := d.stringValue(entries["name"], namePath)
	if err != nil {
		return setting.DeploymentPattern{}, err
	}
	if name == "" || strings.TrimSpace(name) != name {
		return setting.DeploymentPattern{}, invalid(entries["name"], namePath, "must not be empty or have leading or trailing whitespace; got %q", name)
	}
	if strings.ContainsFunc(name, func(r rune) bool { return r <= ' ' }) {
		return setting.DeploymentPattern{}, invalid(entries["name"], namePath, "must not contain ASCII spaces or C0 control characters; got %q", name)
	}
	return setting.DeploymentPattern{Type: patternType, Name: name}, nil
}

// actions は、Actions の階層のうち reco が受け付ける permissions、workflow、access、variables だけを読む。
// どれも任意のキーだが、空のマッピングは拒否されるため、成功すればどれかは必ずある。
func (d document) actions(n *yaml.Node, path string) (ActionsPermissions, WorkflowPermissions, ActionsAccess, []VariableDeclaration, error) {
	entries, err := d.mappingEntries(n, path, nil, []string{"permissions", "workflow", "access", "variables"})
	if err != nil {
		return ActionsPermissions{}, WorkflowPermissions{}, ActionsAccess{}, nil, err
	}
	var permissions ActionsPermissions
	if value, ok := entries["permissions"]; ok {
		if permissions, err = d.permissions(value, path+".permissions"); err != nil {
			return ActionsPermissions{}, WorkflowPermissions{}, ActionsAccess{}, nil, err
		}
	}
	var workflow WorkflowPermissions
	if value, ok := entries["workflow"]; ok {
		if workflow, err = d.workflow(value, path+".workflow"); err != nil {
			return ActionsPermissions{}, WorkflowPermissions{}, ActionsAccess{}, nil, err
		}
	}
	var access ActionsAccess
	if value, ok := entries["access"]; ok {
		if access, err = d.access(value, path+".access"); err != nil {
			return ActionsPermissions{}, WorkflowPermissions{}, ActionsAccess{}, nil, err
		}
	}
	var variables []VariableDeclaration
	if value, ok := entries["variables"]; ok {
		if variables, err = d.variables(value, path+".variables"); err != nil {
			return ActionsPermissions{}, WorkflowPermissions{}, ActionsAccess{}, nil, err
		}
	}
	return permissions, workflow, access, variables, nil
}

// variables は、変数の宣言の並びを宣言の順のまま読む。
// 宣言の名前は英小文字を含まないため、同じ変数を指す二つの名前は綴りも同じになり、重複は == で判定できる。
func (d document) variables(n *yaml.Node, path string) ([]VariableDeclaration, error) {
	items, err := d.sequenceItems(n, path)
	if err != nil {
		return nil, err
	}
	declarations := make([]VariableDeclaration, 0, len(items))
	for i, item := range items {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		declaration, err := d.variable(item, itemPath)
		if err != nil {
			return nil, err
		}
		if j := slices.IndexFunc(declarations, func(seen VariableDeclaration) bool { return seen.Name == declaration.Name }); j >= 0 {
			return nil, invalid(item, itemPath, "duplicate name %q; same as %s[%d]", declaration.Name.String(), path, j)
		}
		declarations = append(declarations, declaration)
	}
	return declarations, nil
}

// variable は、存在し値を伴うことを宣言する一つの変数を読む。
func (d document) variable(n *yaml.Node, path string) (VariableDeclaration, error) {
	entries, err := d.mappingEntries(n, path, []string{"name", "value"}, nil)
	if err != nil {
		return VariableDeclaration{}, err
	}
	namePath := path + ".name"
	name, err := d.stringValue(entries["name"], namePath)
	if err != nil {
		return VariableDeclaration{}, err
	}
	variableName, err := setting.ParseVariableName(name)
	if err != nil {
		return VariableDeclaration{}, invalid(entries["name"], namePath, "%v; got %q", err, name)
	}
	valuePath := path + ".value"
	value, err := d.quotableStringValue(entries["value"], valuePath)
	if err != nil {
		return VariableDeclaration{}, err
	}
	// GitHub は空文字列の値の作成も更新も拒否する (YAML 設定仕様の変数の値)
	if value == "" {
		return VariableDeclaration{}, invalid(entries["value"], valuePath, "must not be an empty string")
	}
	return VariableDeclaration{Name: variableName, Desired: setting.VariableValue(value)}, nil
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

// general は general の設定を読む。マージ方式の組み合わせの規則は 3 つの値で決まるため、すべての値の誤りを先に示す。
func (d document) general(n *yaml.Node, path string) (General, error) {
	entries, err := d.mappingEntries(n, path, nil, []string{"delete_branch_on_merge", "allow_auto_merge", "allow_squash_merge", "allow_merge_commit", "allow_rebase_merge", "allow_update_branch"})
	if err != nil {
		return General{}, err
	}
	var general General
	for _, field := range []struct {
		key string
		dst **bool
	}{
		{"delete_branch_on_merge", &general.DeleteBranchOnMerge},
		{"allow_auto_merge", &general.AllowAutoMerge},
		{"allow_squash_merge", &general.AllowSquashMerge},
		{"allow_merge_commit", &general.AllowMergeCommit},
		{"allow_rebase_merge", &general.AllowRebaseMerge},
		{"allow_update_branch", &general.AllowUpdateBranch},
	} {
		value, ok := entries[field.key]
		if !ok {
			continue
		}
		v, err := d.boolValue(value, path+"."+field.key)
		if err != nil {
			return General{}, err
		}
		*field.dst = &v
	}
	if disablesEveryMergeMethod(general) {
		return General{}, invalid(n, path, "allow_squash_merge, allow_merge_commit, and allow_rebase_merge cannot all be false; GitHub requires at least one merge method")
	}
	return general, nil
}

// disablesEveryMergeMethod は、3 つのマージ方式をすべて宣言し、すべて false としているかを返す。
// GitHub はこの状態を受け付けないため (YAML 設定仕様の General)、観測によらず拒否する。
func disablesEveryMergeMethod(general General) bool {
	desired := DesiredState{General: general}
	for _, key := range setting.MergeMethods() {
		if v, ok := desired.value(key); !ok || v.Bool() {
			return false
		}
	}
	return true
}

// security は security の設定を読む。組み合わせの規則は 2 つの値で決まるため、すべての値の誤りを先に示す。
func (d document) security(n *yaml.Node, path string) (Dependabot, SecretScanning, PrivateVulnerabilityReporting, error) {
	entries, err := d.mappingEntries(n, path, nil, []string{"dependabot_alerts", "dependabot_security_updates", "secret_scanning", "secret_scanning_push_protection", "private_vulnerability_reporting"})
	if err != nil {
		return Dependabot{}, SecretScanning{}, PrivateVulnerabilityReporting{}, err
	}
	var dependabot Dependabot
	var secretScanning SecretScanning
	var privateVulnerabilityReporting PrivateVulnerabilityReporting
	for _, field := range []struct {
		key string
		dst **bool
	}{
		{"dependabot_alerts", &dependabot.Alerts},
		{"dependabot_security_updates", &dependabot.SecurityUpdates},
		{"secret_scanning", &secretScanning.Enabled},
		{"secret_scanning_push_protection", &secretScanning.PushProtection},
		{"private_vulnerability_reporting", &privateVulnerabilityReporting.Enabled},
	} {
		value, ok := entries[field.key]
		if !ok {
			continue
		}
		v, err := d.boolValue(value, path+"."+field.key)
		if err != nil {
			return Dependabot{}, SecretScanning{}, PrivateVulnerabilityReporting{}, err
		}
		*field.dst = &v
	}
	if disablesPrerequisiteOfEnabled(dependabot.Alerts, dependabot.SecurityUpdates) {
		return Dependabot{}, SecretScanning{}, PrivateVulnerabilityReporting{}, invalid(n, path, "dependabot_alerts false cannot be declared with dependabot_security_updates true")
	}
	if disablesPrerequisiteOfEnabled(secretScanning.Enabled, secretScanning.PushProtection) {
		return Dependabot{}, SecretScanning{}, PrivateVulnerabilityReporting{}, invalid(n, path, "secret_scanning false cannot be declared with secret_scanning_push_protection true")
	}
	return dependabot, secretScanning, privateVulnerabilityReporting, nil
}

// disablesPrerequisiteOfEnabled は、前提の設定を false、依存する設定を true と宣言しているかを返す。
// GitHub はこの状態を保てないため (YAML 設定仕様の Security)、どちらの組でも拒否する。
func disablesPrerequisiteOfEnabled(prerequisite, dependent *bool) bool {
	return prerequisite != nil && !*prerequisite && dependent != nil && *dependent
}
