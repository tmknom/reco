package configuration

import (
	"reflect"
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/setting"
)

// TestDocumentPermissions は、Actions の有効状態、allowed_actions、selected_actions、sha_pinning_required の組み合わせの規則と、判定の順序を確かめる。
// 拒否のケースは、受理される selected の入力 (selected) の条件を 1 つだけ変えて作る。
//
//trace:checks FR-001
func TestDocumentPermissions(t *testing.T) {
	const selected = "allowed_actions: selected\nselected_actions:\n  github_owned_allowed: true\n  verified_allowed: false\n  patterns_allowed: [a/*, b/*]\n"
	criteria := func(patterns []string) ActionsPermissions {
		return ActionsPermissions{
			AllowedActions:  new(setting.AllowedActionsSelected),
			SelectedActions: &setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, VerifiedAllowed: false, PatternsAllowed: patterns},
		}
	}
	cases := []struct {
		name string
		src  string
		want ActionsPermissions
		// wantPath は error の先頭に付く、問題の箇所のパス。パスの直後の " (" まで照合する。
		wantPath   string
		wantReason string
	}{
		{name: "all without selected_actions is read, leaving selected_actions omitted", src: "allowed_actions: all\n", want: ActionsPermissions{AllowedActions: new(setting.AllowedActionsAll)}},
		{name: "local_only without selected_actions is read, leaving selected_actions omitted", src: "allowed_actions: local_only\n", want: ActionsPermissions{AllowedActions: new(setting.AllowedActionsLocalOnly)}},
		{name: "selected with all three items is read as one group", src: selected, want: criteria([]string{"a/*", "b/*"})},
		{name: "empty patterns_allowed is read as non-nil empty", src: strings.Replace(selected, "[a/*, b/*]", "[]", 1), want: criteria([]string{})},
		{
			name:       "selected without selected_actions is rejected naming the three items",
			src:        "allowed_actions: selected\n",
			wantPath:   "P",
			wantReason: `allowed_actions "selected" requires selected_actions with github_owned_allowed, verified_allowed, patterns_allowed`,
		},
		{
			name:       "all with selected_actions is rejected at selected_actions",
			src:        strings.Replace(selected, "allowed_actions: selected", "allowed_actions: all", 1),
			wantPath:   "P.selected_actions",
			wantReason: `selected_actions is allowed only when allowed_actions is "selected"; got allowed_actions "all"`,
		},
		{
			name:       "local_only with selected_actions is rejected at selected_actions",
			src:        strings.Replace(selected, "allowed_actions: selected", "allowed_actions: local_only", 1),
			wantPath:   "P.selected_actions",
			wantReason: `got allowed_actions "local_only"`,
		},
		{
			name:       "selected_actions without allowed_actions is rejected",
			src:        strings.Replace(selected, "allowed_actions: selected\n", "", 1),
			wantPath:   "P",
			wantReason: `selected_actions requires allowed_actions "selected"`,
		},
		{
			name:       "all with invalid selected_actions contents is rejected by the combination, not the contents",
			src:        "allowed_actions: all\nselected_actions:\n  sha_pinning_required: true\n",
			wantPath:   "P.selected_actions",
			wantReason: "selected_actions is allowed only when",
		},
		{
			name:       "undefined allowed_actions with selected_actions is rejected by the value first",
			src:        strings.Replace(selected, "allowed_actions: selected", "allowed_actions: none", 1),
			wantPath:   "P.allowed_actions",
			wantReason: `must be all, local_only, or selected; got "none"`,
		},
		{name: "empty permissions is rejected", src: "{}\n", wantPath: "P", wantReason: "must not be empty"},
		{name: "unknown key is rejected", src: selected + "unknown: true\n", wantPath: "P", wantReason: `unknown key "unknown"; expected enabled, allowed_actions, selected_actions, sha_pinning_required`},
		{name: "enabled true alone is read as the enabled state", src: "enabled: true\n", want: ActionsPermissions{Enabled: new(true)}},
		{name: "enabled false alone is read as the enabled state", src: "enabled: false\n", want: ActionsPermissions{Enabled: new(false)}},
		{name: "enabled true with all is read the same as all alone", src: "enabled: true\nallowed_actions: all\n", want: ActionsPermissions{AllowedActions: new(setting.AllowedActionsAll)}},
		{name: "enabled true with selected is read the same as selected alone", src: "enabled: true\n" + selected, want: criteria([]string{"a/*", "b/*"})},
		{
			name:       "enabled false with selected_actions alone is rejected by the enabled combination first",
			src:        "enabled: false\n" + strings.Replace(selected, "allowed_actions: selected\n", "", 1),
			wantPath:   "P",
			wantReason: "enabled false cannot be declared with selected_actions",
		},
		{
			name:       "enabled false with undefined allowed_actions is rejected by the combination, not the value",
			src:        "enabled: false\nallowed_actions: none\n",
			wantPath:   "P",
			wantReason: "enabled false cannot be declared with allowed_actions",
		},
		{
			name:       "quoted enabled with allowed_actions is rejected at enabled",
			src:        "enabled: \"true\"\nallowed_actions: all\n",
			wantPath:   "P.enabled",
			wantReason: "must be true or false",
		},
		{
			name:       "enabled true with selected_actions alone is rejected for missing allowed_actions",
			src:        "enabled: true\n" + strings.Replace(selected, "allowed_actions: selected\n", "", 1),
			wantPath:   "P",
			wantReason: `selected_actions requires allowed_actions "selected"`,
		},
		{name: "sha_pinning_required alone is read without allowed_actions", src: "sha_pinning_required: true\n", want: ActionsPermissions{SHAPinningRequired: new(true)}},
		{name: "enabled true with sha_pinning_required reads both", src: "enabled: true\nsha_pinning_required: false\n", want: ActionsPermissions{Enabled: new(true), SHAPinningRequired: new(false)}},
		{name: "all with sha_pinning_required reads both", src: "allowed_actions: all\nsha_pinning_required: true\n", want: ActionsPermissions{AllowedActions: new(setting.AllowedActionsAll), SHAPinningRequired: new(true)}},
		{
			name:       "enabled false with sha_pinning_required is rejected naming it",
			src:        "enabled: false\nsha_pinning_required: true\n",
			wantPath:   "P",
			wantReason: "enabled false cannot be declared with sha_pinning_required",
		},
		{
			name:       "enabled false with every other key is rejected naming them in the configuration order",
			src:        "enabled: false\nsha_pinning_required: true\n" + selected,
			wantPath:   "P",
			wantReason: "enabled false cannot be declared with allowed_actions, selected_actions, sha_pinning_required",
		},
		{
			name:       "enabled false with quoted sha_pinning_required is rejected by the combination, not the value",
			src:        "enabled: false\nsha_pinning_required: \"true\"\n",
			wantPath:   "P",
			wantReason: "enabled false cannot be declared with sha_pinning_required",
		},
		{
			name:       "quoted sha_pinning_required is rejected at sha_pinning_required",
			src:        "sha_pinning_required: \"true\"\n",
			wantPath:   "P.sha_pinning_required",
			wantReason: "must be true or false",
		},
		{name: "duplicate allowed_actions is rejected", src: selected + "allowed_actions: all\n", wantPath: "P", wantReason: `duplicate key "allowed_actions"`},
		{
			name:       "selected_actions without a value is rejected rather than read as omitted",
			src:        "allowed_actions: selected\nselected_actions:\n",
			wantPath:   "P.selected_actions",
			wantReason: "must not be null",
		},
		{
			name:       "invalid selected_actions contents are rejected at selected_actions",
			src:        strings.Replace(selected, "verified_allowed: false\n  ", "", 1),
			wantPath:   "P.selected_actions",
			wantReason: "missing verified_allowed",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, err := doc.permissions(doc.root, "P")
			if tc.wantPath != "" {
				if err == nil {
					t.Fatalf("permissions() = %+v, want error", got)
				}
				if !strings.HasPrefix(err.Error(), tc.wantPath+" (") {
					t.Errorf("permissions() error = %q, want it to start with %q", err, tc.wantPath+" (")
				}
				if !strings.Contains(err.Error(), tc.wantReason) {
					t.Errorf("permissions() error = %q, want it to contain %q", err, tc.wantReason)
				}
				return
			}
			if err != nil {
				t.Fatalf("permissions() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("permissions() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

//trace:checks FR-001
func TestDocumentSelectedActions(t *testing.T) {
	const all = "github_owned_allowed: true\nverified_allowed: false\npatterns_allowed: [a/*]\n"
	cases := []struct {
		name       string
		src        string
		want       setting.SelectedActionsCriteria
		wantReason string
	}{
		{name: "three items are read into their own fields", src: all, want: setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, VerifiedAllowed: false, PatternsAllowed: []string{"a/*"}}},
		{name: "swapped bool values are read into their own fields", src: "github_owned_allowed: false\nverified_allowed: true\npatterns_allowed: []\n", want: setting.SelectedActionsCriteria{GitHubOwnedAllowed: false, VerifiedAllowed: true, PatternsAllowed: []string{}}},
		{name: "missing verified_allowed is rejected naming it", src: strings.Replace(all, "verified_allowed: false\n", "", 1), wantReason: "missing verified_allowed; selected_actions requires all of github_owned_allowed, verified_allowed, patterns_allowed"},
		{name: "only github_owned_allowed is rejected naming both missing items in definition order", src: "github_owned_allowed: true\n", wantReason: "missing verified_allowed, patterns_allowed; selected_actions requires all of"},
		{name: "unknown key is rejected", src: all + "sha_pinning_required: true\n", wantReason: `unknown key "sha_pinning_required"; expected github_owned_allowed, verified_allowed, patterns_allowed`},
		{name: "duplicate key is rejected", src: all + "verified_allowed: true\n", wantReason: `duplicate key "verified_allowed"`},
		{name: "empty mapping is rejected", src: "{}\n", wantReason: "must not be empty"},
		{name: "invalid github_owned_allowed is rejected at the item", src: strings.Replace(all, "github_owned_allowed: true", "github_owned_allowed: yes", 1), wantReason: "S.github_owned_allowed (line 1, column 23): must be true or false"},
		{name: "invalid bool is rejected at the item", src: strings.Replace(all, "verified_allowed: false", "verified_allowed: no", 1), wantReason: "S.verified_allowed (line 2, column 19): must be true or false"},
		{name: "invalid patterns is rejected at the item", src: strings.Replace(all, "[a/*]", "a/*", 1), wantReason: "S.patterns_allowed (line 3, column 19): must be a sequence"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, err := doc.selectedActions(doc.root, "S")
			if tc.wantReason != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantReason) {
					t.Errorf("selectedActions() error = %v, want it to contain %q", err, tc.wantReason)
				}
				return
			}
			if err != nil {
				t.Fatalf("selectedActions() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("selectedActions() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestDocumentDeclaration は、各階層で要求するキーの集合 (スキーマ) と、各段階へ渡すパスを確かめる。
// 各段階が持つ規則 (マッピングやシーケンスの形、真偽値、name の形式) は、段階ごとのテストが確かめる。
//
//trace:checks FR-001
func TestDocumentDeclaration(t *testing.T) {
	repositoryDeclaration := func(name string, general General) RepositoryDeclaration {
		return RepositoryDeclaration{
			Reference: mustReference(t, name),
			Desired:   DesiredState{General: general},
		}
	}
	first := repositoryDeclaration("owner/repo", General{AllowAutoMerge: new(true)})
	// second は 2 件目の宣言を持つ入力。失敗のケースは 2 件目の宣言を変えて作り、パスの添字を確かめる。
	second := minimal + `  - name: owner/second
    general:
      allow_auto_merge: false
`
	// withWorkflow は、2 件目の宣言に actions.workflow を足し、その中身を entries にした入力を作る。
	withWorkflow := func(entries string) string {
		return second + "    actions:\n      workflow:\n" + entries
	}
	actionsOnly := strings.Replace(second, "    general:\n      allow_auto_merge: false\n",
		"    actions:\n      workflow:\n        default_workflow_permissions: write\n        can_approve_pull_request_reviews: true\n", 1)
	cases := []struct {
		name string
		src  string
		want Declaration
		// wantPath は error の先頭に付く、問題の箇所のパス。パスの直後の " (" まで照合し、前置きや添字の誤りを部分一致で見逃さない。
		wantPath   string
		wantReason string
	}{
		{
			name: "allow_auto_merge alone is read, leaving delete_branch_on_merge omitted",
			src:  minimal,
			want: Declaration{Repositories: []RepositoryDeclaration{first}},
		},
		{
			name: "all repository declarations are read, keeping false as an explicit expected value",
			src:  second,
			want: Declaration{Repositories: []RepositoryDeclaration{
				first,
				repositoryDeclaration("owner/second", General{AllowAutoMerge: new(false)}),
			}},
		},
		{
			name: "delete_branch_on_merge alone is read, leaving allow_auto_merge omitted",
			src:  strings.Replace(second, "allow_auto_merge: false", "delete_branch_on_merge: true", 1),
			want: Declaration{Repositories: []RepositoryDeclaration{
				first,
				repositoryDeclaration("owner/second", General{DeleteBranchOnMerge: new(true)}),
			}},
		},
		{
			name: "both general settings are read into their own fields",
			src:  strings.Replace(second, "allow_auto_merge: false", "allow_auto_merge: true\n      delete_branch_on_merge: false", 1),
			want: Declaration{Repositories: []RepositoryDeclaration{
				first,
				repositoryDeclaration("owner/second", General{DeleteBranchOnMerge: new(false), AllowAutoMerge: new(true)}),
			}},
		},
		// ルート
		{
			name:       "unknown key at root is rejected",
			src:        minimal + "extra: true\n",
			wantPath:   "root",
			wantReason: `unknown key "extra"`,
		},
		{
			name:       "invalid repositories is rejected at repositories",
			src:        "repositories: []\n",
			wantPath:   "repositories",
			wantReason: "must not be empty",
		},
		{
			name: "default_workflow_permissions alone is read, leaving can_approve_pull_request_reviews omitted",
			src:  withWorkflow("        default_workflow_permissions: read\n"),
			want: Declaration{Repositories: []RepositoryDeclaration{
				first,
				{
					Reference: mustReference(t, "owner/second"),
					Desired: DesiredState{
						General:             General{AllowAutoMerge: new(false)},
						WorkflowPermissions: WorkflowPermissions{DefaultWorkflowPermissions: new(setting.PermissionRead)},
					},
				},
			}},
		},
		{
			name: "can_approve_pull_request_reviews alone is read, leaving default_workflow_permissions omitted",
			src:  withWorkflow("        can_approve_pull_request_reviews: false\n"),
			want: Declaration{Repositories: []RepositoryDeclaration{
				first,
				{
					Reference: mustReference(t, "owner/second"),
					Desired: DesiredState{
						General:             General{AllowAutoMerge: new(false)},
						WorkflowPermissions: WorkflowPermissions{CanApprovePullRequestReviews: new(false)},
					},
				},
			}},
		},
		{
			name: "actions without general is read, leaving every general setting omitted",
			src:  actionsOnly,
			want: Declaration{Repositories: []RepositoryDeclaration{
				first,
				{
					Reference: mustReference(t, "owner/second"),
					Desired: DesiredState{WorkflowPermissions: WorkflowPermissions{
						DefaultWorkflowPermissions:   new(setting.PermissionWrite),
						CanApprovePullRequestReviews: new(true),
					}},
				},
			}},
		},
		{
			name: "actions with variables alone is read, leaving the Actions permission settings omitted",
			src:  strings.Replace(second, "    general:\n      allow_auto_merge: false\n", "    actions:\n      variables:\n        - name: GO_VERSION\n          value: \"1.27\"\n", 1),
			want: Declaration{Repositories: []RepositoryDeclaration{
				first,
				{
					Reference: mustReference(t, "owner/second"),
					Desired: DesiredState{RepositoryVariables: []VariableDeclaration{
						{Name: mustVariableName(t, "GO_VERSION"), Desired: setting.VariableValue("1.27")},
					}},
				},
			}},
		},
		// リポジトリごとの宣言
		{
			name:       "name alone is rejected for lacking both general and actions",
			src:        strings.Replace(second, "    general:\n      allow_auto_merge: false\n", "", 1),
			wantPath:   "repositories[1]",
			wantReason: "must have at least one of general, actions, environments, security",
		},
		{
			name:       "missing name is rejected",
			src:        strings.Replace(second, "  - name: owner/second\n", "  -\n", 1),
			wantPath:   "repositories[1]",
			wantReason: `missing key "name"`,
		},
		{
			name:       "non-string name is rejected at the name",
			src:        strings.Replace(second, "owner/second", "42", 1),
			wantPath:   "repositories[1].name",
			wantReason: "must be a string",
		},
		{
			name:       "invalid name is rejected at the name with the value",
			src:        strings.Replace(second, "owner/second", "second", 1),
			wantPath:   "repositories[1].name",
			wantReason: `exactly one "/"; got "second"`,
		},
		{
			name:       "same name as an earlier declaration is rejected at the later declaration",
			src:        strings.Replace(second, "owner/second", "owner/repo", 1),
			wantPath:   "repositories[1]",
			wantReason: `duplicate name "owner/repo"; same as repositories[0]`,
		},
		{
			name:       "name differing only in letter case is rejected as a duplicate",
			src:        strings.Replace(second, "owner/second", "Owner/REPO", 1),
			wantPath:   "repositories[1]",
			wantReason: `duplicate name "Owner/REPO"; same as repositories[0]`,
		},
		// General
		{
			name:       "unknown key in general is rejected",
			src:        second + "      allow_squash_merge: true\n",
			wantPath:   "repositories[1].general",
			wantReason: `unknown key "allow_squash_merge"`,
		},
		{
			name:       "delete_branch_on_merge without a value is rejected rather than read as omitted",
			src:        second + "      delete_branch_on_merge:\n",
			wantPath:   "repositories[1].general.delete_branch_on_merge",
			wantReason: "must not be null",
		},
		{
			name:       "invalid allow_auto_merge is rejected at allow_auto_merge",
			src:        strings.Replace(second, "allow_auto_merge: false", "allow_auto_merge: FALSE", 1),
			wantPath:   "repositories[1].general.allow_auto_merge",
			wantReason: "must be true or false",
		},
		// Actions
		{
			name: "actions.permissions alone is read into ActionsPermissions, leaving workflow omitted",
			src:  second + "    actions:\n      permissions:\n        allowed_actions: local_only\n",
			want: Declaration{Repositories: []RepositoryDeclaration{
				first,
				{
					Reference: mustReference(t, "owner/second"),
					Desired: DesiredState{
						General:            General{AllowAutoMerge: new(false)},
						ActionsPermissions: ActionsPermissions{AllowedActions: new(setting.AllowedActionsLocalOnly)},
					},
				},
			}},
		},
		{
			name: "actions.permissions and actions.workflow are read into their own fields",
			src:  second + "    actions:\n      permissions:\n        allowed_actions: all\n      workflow:\n        can_approve_pull_request_reviews: true\n",
			want: Declaration{Repositories: []RepositoryDeclaration{
				first,
				{
					Reference: mustReference(t, "owner/second"),
					Desired: DesiredState{
						General:             General{AllowAutoMerge: new(false)},
						ActionsPermissions:  ActionsPermissions{AllowedActions: new(setting.AllowedActionsAll)},
						WorkflowPermissions: WorkflowPermissions{CanApprovePullRequestReviews: new(true)},
					},
				},
			}},
		},
		{
			name: "enabled false is read together with actions.workflow",
			src:  second + "    actions:\n      permissions:\n        enabled: false\n      workflow:\n        can_approve_pull_request_reviews: true\n",
			want: Declaration{Repositories: []RepositoryDeclaration{
				first,
				{
					Reference: mustReference(t, "owner/second"),
					Desired: DesiredState{
						General:             General{AllowAutoMerge: new(false)},
						ActionsPermissions:  ActionsPermissions{Enabled: new(false)},
						WorkflowPermissions: WorkflowPermissions{CanApprovePullRequestReviews: new(true)},
					},
				},
			}},
		},
		{
			name: "access alone is read into the actions access desired state",
			src:  second + "    actions:\n      access:\n        access_level: user\n",
			want: Declaration{Repositories: []RepositoryDeclaration{
				first,
				{
					Reference: mustReference(t, "owner/second"),
					Desired: DesiredState{
						General:       General{AllowAutoMerge: new(false)},
						ActionsAccess: ActionsAccess{AccessLevel: new(setting.AccessUser)},
					},
				},
			}},
		},
		{
			name:       "unknown key in actions is rejected listing permissions, workflow, and access",
			src:        second + "    actions:\n      secrets: {}\n",
			wantPath:   "repositories[1].actions",
			wantReason: `unknown key "secrets"; expected permissions, workflow, access`,
		},
		// Environments
		{
			name: "repository with only environments is accepted",
			src:  minimal + "  - name: owner/second\n    environments:\n      - name: staging\n        deployment_branch_policy:\n          mode: protected_branches\n",
			want: Declaration{Repositories: []RepositoryDeclaration{
				first,
				{
					Reference: mustReference(t, "owner/second"),
					Desired: DesiredState{Environments: []EnvironmentDeclaration{{
						Name:                   mustEnvironmentName(t, "staging"),
						DeploymentBranchPolicy: &setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyProtectedBranches},
					}}},
				},
			}},
		},
		{
			name:       "invalid environment name is rejected at the path of the repository and the item",
			src:        second + "    environments:\n      - name: staging\n      - name: café\n",
			wantPath:   "repositories[1].environments[1].name",
			wantReason: "must consist of printable ASCII characters",
		},
		// Security
		{
			name: "repository with only security is accepted",
			src:  minimal + "  - name: owner/second\n    security:\n      dependabot_alerts: true\n",
			want: Declaration{Repositories: []RepositoryDeclaration{
				first,
				{
					Reference: mustReference(t, "owner/second"),
					Desired:   DesiredState{Dependabot: Dependabot{Alerts: new(true)}},
				},
			}},
		},
		{
			name: "repository with only secret scanning settings is accepted",
			src:  minimal + "  - name: owner/second\n    security:\n      secret_scanning_push_protection: false\n",
			want: Declaration{Repositories: []RepositoryDeclaration{
				first,
				{
					Reference: mustReference(t, "owner/second"),
					Desired:   DesiredState{SecretScanning: SecretScanning{PushProtection: new(false)}},
				},
			}},
		},
		{
			name:       "invalid security is rejected at the path of the repository and security",
			src:        second + "    security:\n      dependabot_alerts: false\n      dependabot_security_updates: true\n",
			wantPath:   "repositories[1].security",
			wantReason: "dependabot_alerts false cannot be declared with dependabot_security_updates true",
		},
		// Actions access
		{
			name:       "unknown key in access is rejected",
			src:        second + "    actions:\n      access:\n        access_level: none\n        scope: all\n",
			wantPath:   "repositories[1].actions.access",
			wantReason: `unknown key "scope"; expected access_level`,
		},
		{
			name:       "organization access_level is rejected at actions.access.access_level",
			src:        second + "    actions:\n      access:\n        access_level: organization\n",
			wantPath:   "repositories[1].actions.access.access_level",
			wantReason: "must be none or user",
		},
		{
			name:       "empty variables is rejected at actions.variables",
			src:        second + "    actions:\n      variables: []\n",
			wantPath:   "repositories[1].actions.variables",
			wantReason: "must not be empty",
		},
		{
			name:       "invalid permissions is rejected at actions.permissions",
			src:        second + "    actions:\n      permissions:\n        allowed_actions: selected\n",
			wantPath:   "repositories[1].actions.permissions",
			wantReason: "requires selected_actions",
		},
		{
			name:       "empty actions is rejected",
			src:        second + "    actions: {}\n",
			wantPath:   "repositories[1].actions",
			wantReason: "must not be empty",
		},
		{
			name:       "actions without a value is rejected",
			src:        second + "    actions:\n",
			wantPath:   "repositories[1].actions",
			wantReason: "must not be null",
		},
		{
			name:       "duplicate key in actions is rejected",
			src:        withWorkflow("        default_workflow_permissions: read\n      workflow:\n        default_workflow_permissions: write\n"),
			wantPath:   "repositories[1].actions",
			wantReason: `duplicate key "workflow"`,
		},
		// Workflow permissions
		{
			name:       "empty workflow is rejected",
			src:        second + "    actions:\n      workflow: {}\n",
			wantPath:   "repositories[1].actions.workflow",
			wantReason: "must not be empty",
		},
		{
			name:       "workflow without a value is rejected",
			src:        second + "    actions:\n      workflow:\n",
			wantPath:   "repositories[1].actions.workflow",
			wantReason: "must not be null",
		},
		{
			name:       "unknown key in workflow is rejected",
			src:        withWorkflow("        default_workflow_permissions: read\n        sha_pinning_required: true\n"),
			wantPath:   "repositories[1].actions.workflow",
			wantReason: `unknown key "sha_pinning_required"; expected default_workflow_permissions, can_approve_pull_request_reviews`,
		},
		{
			name:       "duplicate key in workflow is rejected",
			src:        withWorkflow("        default_workflow_permissions: read\n        default_workflow_permissions: write\n"),
			wantPath:   "repositories[1].actions.workflow",
			wantReason: `duplicate key "default_workflow_permissions"`,
		},
		{
			name:       "default_workflow_permissions without a value is rejected rather than read as omitted",
			src:        withWorkflow("        default_workflow_permissions:\n"),
			wantPath:   "repositories[1].actions.workflow.default_workflow_permissions",
			wantReason: "must not be null",
		},
		{
			name:       "can_approve_pull_request_reviews without a value is rejected rather than read as omitted",
			src:        withWorkflow("        can_approve_pull_request_reviews:\n"),
			wantPath:   "repositories[1].actions.workflow.can_approve_pull_request_reviews",
			wantReason: "must not be null",
		},
		{
			name:       "quoted can_approve_pull_request_reviews is rejected by the same rule as general settings",
			src:        withWorkflow("        can_approve_pull_request_reviews: \"true\"\n"),
			wantPath:   "repositories[1].actions.workflow.can_approve_pull_request_reviews",
			wantReason: "must be true or false",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeSnippet(t, tc.src).declaration()
			if tc.wantPath != "" {
				if err == nil {
					t.Fatalf("declaration() = %+v, want error", got)
				}
				if !strings.HasPrefix(err.Error(), tc.wantPath+" (") {
					t.Errorf("declaration() error = %q, want it to start with %q", err, tc.wantPath+" (")
				}
				if !strings.Contains(err.Error(), tc.wantReason) {
					t.Errorf("declaration() error = %q, want it to contain %q", err, tc.wantReason)
				}
				return
			}
			if err != nil {
				t.Fatalf("declaration() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("declaration() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestDocumentSecurity は、security の各設定がそれぞれ独立に宣言できることと、依存する設定を有効にできない組み合わせの拒否、判定の順序を確かめる。
//
//trace:checks FR-001
func TestDocumentSecurity(t *testing.T) {
	cases := []struct {
		name                              string
		src                               string
		want                              Dependabot
		wantSecretScanning                SecretScanning
		wantPrivateVulnerabilityReporting PrivateVulnerabilityReporting
		// wantPath は error の先頭に付く、問題の箇所のパス。パスの直後の " (" まで照合する。
		wantPath   string
		wantReason string
	}{
		{name: "dependabot_alerts true alone is read, leaving dependabot_security_updates omitted", src: "dependabot_alerts: true\n", want: Dependabot{Alerts: new(true)}},
		{name: "dependabot_alerts false alone is read, leaving dependabot_security_updates omitted", src: "dependabot_alerts: false\n", want: Dependabot{Alerts: new(false)}},
		{name: "dependabot_security_updates true alone is read, leaving dependabot_alerts omitted", src: "dependabot_security_updates: true\n", want: Dependabot{SecurityUpdates: new(true)}},
		{name: "dependabot_security_updates false alone is read, leaving dependabot_alerts omitted", src: "dependabot_security_updates: false\n", want: Dependabot{SecurityUpdates: new(false)}},
		{name: "alerts true with updates true is read into their own fields", src: "dependabot_alerts: true\ndependabot_security_updates: true\n", want: Dependabot{Alerts: new(true), SecurityUpdates: new(true)}},
		{name: "alerts true with updates false is read into their own fields", src: "dependabot_alerts: true\ndependabot_security_updates: false\n", want: Dependabot{Alerts: new(true), SecurityUpdates: new(false)}},
		{name: "alerts false with updates false is read into their own fields", src: "dependabot_alerts: false\ndependabot_security_updates: false\n", want: Dependabot{Alerts: new(false), SecurityUpdates: new(false)}},
		{
			name:       "alerts false with updates true is rejected at security",
			src:        "dependabot_alerts: false\ndependabot_security_updates: true\n",
			wantPath:   "P",
			wantReason: "dependabot_alerts false cannot be declared with dependabot_security_updates true",
		},
		{name: "empty security is rejected", src: "{}\n", wantPath: "P", wantReason: "must not be empty"},
		{
			name:       "unknown key is rejected",
			src:        "dependabot_alerts: true\nsecret_scanning_validity_checks: true\n",
			wantPath:   "P",
			wantReason: `unknown key "secret_scanning_validity_checks"; expected dependabot_alerts, dependabot_security_updates, secret_scanning, secret_scanning_push_protection, private_vulnerability_reporting`,
		},
		{name: "secret_scanning true alone is read, leaving the push protection omitted", src: "secret_scanning: true\n", wantSecretScanning: SecretScanning{Enabled: new(true)}},
		{name: "secret_scanning false alone is read, leaving the push protection omitted", src: "secret_scanning: false\n", wantSecretScanning: SecretScanning{Enabled: new(false)}},
		{name: "push protection true alone is read, leaving secret_scanning omitted", src: "secret_scanning_push_protection: true\n", wantSecretScanning: SecretScanning{PushProtection: new(true)}},
		{name: "push protection false alone is read, leaving secret_scanning omitted", src: "secret_scanning_push_protection: false\n", wantSecretScanning: SecretScanning{PushProtection: new(false)}},
		{name: "secret_scanning true with push protection true is read into their own fields", src: "secret_scanning: true\nsecret_scanning_push_protection: true\n", wantSecretScanning: SecretScanning{Enabled: new(true), PushProtection: new(true)}},
		{name: "secret_scanning true with push protection false is read into their own fields", src: "secret_scanning: true\nsecret_scanning_push_protection: false\n", wantSecretScanning: SecretScanning{Enabled: new(true), PushProtection: new(false)}},
		{name: "secret_scanning false with push protection false is read into their own fields", src: "secret_scanning: false\nsecret_scanning_push_protection: false\n", wantSecretScanning: SecretScanning{Enabled: new(false), PushProtection: new(false)}},
		{
			name:       "secret_scanning false with push protection true is rejected at security",
			src:        "secret_scanning: false\nsecret_scanning_push_protection: true\n",
			wantPath:   "P",
			wantReason: "secret_scanning false cannot be declared with secret_scanning_push_protection true",
		},
		{
			name:               "dependabot and secret scanning settings are read together",
			src:                "dependabot_alerts: true\nsecret_scanning: true\n",
			want:               Dependabot{Alerts: new(true)},
			wantSecretScanning: SecretScanning{Enabled: new(true)},
		},
		{
			name:       "invalid secret_scanning value is rejected before the dependabot combination",
			src:        "dependabot_alerts: false\ndependabot_security_updates: true\nsecret_scanning: yes\n",
			wantPath:   "P.secret_scanning",
			wantReason: "must be true or false",
		},
		{
			name:       "invalid push protection value is rejected before the secret scanning combination",
			src:        "secret_scanning: false\nsecret_scanning_push_protection: \"true\"\n",
			wantPath:   "P.secret_scanning_push_protection",
			wantReason: "must be true or false",
		},
		{
			name:       "dependabot combination is rejected before the secret scanning combination",
			src:        "dependabot_alerts: false\ndependabot_security_updates: true\nsecret_scanning: false\nsecret_scanning_push_protection: true\n",
			wantPath:   "P",
			wantReason: "dependabot_alerts false cannot be declared with dependabot_security_updates true",
		},
		{name: "private_vulnerability_reporting true alone is read", src: "private_vulnerability_reporting: true\n", wantPrivateVulnerabilityReporting: PrivateVulnerabilityReporting{Enabled: new(true)}},
		{name: "private_vulnerability_reporting false alone is read", src: "private_vulnerability_reporting: false\n", wantPrivateVulnerabilityReporting: PrivateVulnerabilityReporting{Enabled: new(false)}},
		{
			name:                              "private_vulnerability_reporting has no combination rule with secret_scanning",
			src:                               "secret_scanning: false\nprivate_vulnerability_reporting: true\n",
			wantSecretScanning:                SecretScanning{Enabled: new(false)},
			wantPrivateVulnerabilityReporting: PrivateVulnerabilityReporting{Enabled: new(true)},
		},
		{
			name:       "invalid private_vulnerability_reporting value is rejected before the dependabot combination",
			src:        "dependabot_alerts: false\ndependabot_security_updates: true\nprivate_vulnerability_reporting: yes\n",
			wantPath:   "P.private_vulnerability_reporting",
			wantReason: "must be true or false",
		},
		{
			name:       "quoted dependabot_alerts is rejected at the value before the combination",
			src:        "dependabot_alerts: \"false\"\ndependabot_security_updates: true\n",
			wantPath:   "P.dependabot_alerts",
			wantReason: "must be true or false",
		},
		{
			name:       "quoted dependabot_security_updates is rejected at the value before the combination",
			src:        "dependabot_alerts: false\ndependabot_security_updates: \"true\"\n",
			wantPath:   "P.dependabot_security_updates",
			wantReason: "must be true or false",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, gotSecretScanning, gotPrivateVulnerabilityReporting, err := doc.security(doc.root, "P")
			if tc.wantPath != "" {
				if err == nil {
					t.Fatalf("security() = %+v, %+v, want error", got, gotSecretScanning)
				}
				if !strings.HasPrefix(err.Error(), tc.wantPath+" (") {
					t.Errorf("security() error = %q, want it to start with %q", err, tc.wantPath+" (")
				}
				if !strings.Contains(err.Error(), tc.wantReason) {
					t.Errorf("security() error = %q, want it to contain %q", err, tc.wantReason)
				}
				return
			}
			if err != nil {
				t.Fatalf("security() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("security() Dependabot = %+v, want %+v", got, tc.want)
			}
			if !reflect.DeepEqual(gotSecretScanning, tc.wantSecretScanning) {
				t.Errorf("security() SecretScanning = %+v, want %+v", gotSecretScanning, tc.wantSecretScanning)
			}
			if !reflect.DeepEqual(gotPrivateVulnerabilityReporting, tc.wantPrivateVulnerabilityReporting) {
				t.Errorf("security() PrivateVulnerabilityReporting = %+v, want %+v", gotPrivateVulnerabilityReporting, tc.wantPrivateVulnerabilityReporting)
			}
		})
	}
}

// TestDocumentEnvironments は、environments の並びが持つ規則 (空の拒否、同一とみなす名前の重複、宣言の並びの保持) を確かめる。
// 拒否のケースは、受理される入力 (base) の条件を 1 つだけ変えて作る。
//
//trace:checks FR-001
func TestDocumentEnvironments(t *testing.T) {
	const base = "- name: staging\n- name: production\n"
	declaration := func(name string) EnvironmentDeclaration {
		return EnvironmentDeclaration{Name: mustEnvironmentName(t, name)}
	}
	cases := []struct {
		name       string
		src        string
		want       []EnvironmentDeclaration
		wantPath   string
		wantReason string
	}{
		{name: "declarations are read in the declared order, not sorted by name", src: base, want: []EnvironmentDeclaration{declaration("staging"), declaration("production")}},
		{name: "names differing in more than letter case are both accepted", src: "- name: staging\n- name: staging2\n", want: []EnvironmentDeclaration{declaration("staging"), declaration("staging2")}},
		{name: "empty sequence is rejected", src: "[]\n", wantPath: "E", wantReason: "must not be empty"},
		{name: "same name as an earlier declaration is rejected at the later declaration", src: base + "- name: staging\n", wantPath: "E[2]", wantReason: `duplicate name "staging"; same as E[0] (compared ignoring letter case)`},
		{name: "name differing only in letter case is rejected as a duplicate naming the earlier index", src: base + "- name: PRODUCTION\n", wantPath: "E[2]", wantReason: `duplicate name "PRODUCTION"; same as E[1] (compared ignoring letter case)`},
		{name: "invalid item is rejected at the item", src: "- name: staging\n- deployment_branch_policy:\n    mode: all\n", wantPath: "E[1]", wantReason: `missing key "name"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, err := doc.environments(doc.root, "E")
			if tc.wantPath != "" {
				if err == nil {
					t.Fatalf("environments() = %+v, want error", got)
				}
				if !strings.HasPrefix(err.Error(), tc.wantPath+" (") {
					t.Errorf("environments() error = %q, want it to start with %q", err, tc.wantPath+" (")
				}
				if !strings.Contains(err.Error(), tc.wantReason) {
					t.Errorf("environments() error = %q, want it to contain %q", err, tc.wantReason)
				}
				return
			}
			if err != nil {
				t.Fatalf("environments() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("environments() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

//trace:checks FR-001
func TestDocumentVariables(t *testing.T) {
	const base = "- name: GO_VERSION\n  value: \"1.27\"\n- name: GO_ARCH\n  value: amd64\n"
	cases := []struct {
		name       string
		src        string
		want       []VariableDeclaration
		wantPath   string
		wantReason string
	}{
		{
			name: "declarations are read in the declared order, not sorted by name",
			src:  base,
			want: []VariableDeclaration{
				{Name: mustVariableName(t, "GO_VERSION"), Desired: setting.VariableValue("1.27")},
				{Name: mustVariableName(t, "GO_ARCH"), Desired: setting.VariableValue("amd64")},
			},
		},
		{name: "empty sequence is rejected", src: "[]\n", wantPath: "V", wantReason: "must not be empty"},
		{name: "same name as an earlier declaration is rejected at the later declaration", src: base + "- name: GO_VERSION\n  value: \"1.28\"\n", wantPath: "V[2]", wantReason: `duplicate name "GO_VERSION"; same as V[0]`},
		{name: "invalid item is rejected at the item", src: "- name: GO_VERSION\n  value: \"1.27\"\n- name: GO_ARCH\n", wantPath: "V[1]", wantReason: `missing key "value"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, err := doc.variables(doc.root, "V")
			if tc.wantPath != "" {
				if err == nil {
					t.Fatalf("variables() = %+v, want error", got)
				}
				if !strings.HasPrefix(err.Error(), tc.wantPath+" (") {
					t.Errorf("variables() error = %q, want it to start with %q", err, tc.wantPath+" (")
				}
				if !strings.Contains(err.Error(), tc.wantReason) {
					t.Errorf("variables() error = %q, want it to contain %q", err, tc.wantReason)
				}
				return
			}
			if err != nil {
				t.Fatalf("variables() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("variables() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestDocumentVariable は、一つの変数の宣言が持つ規則 (キーの集合、name の形式、value の空文字列) を確かめる。
// 拒否のケースは、受理される入力 (base) の条件を 1 つだけ変えて作る。
//
//trace:checks FR-001
func TestDocumentVariable(t *testing.T) {
	const base = "name: GO_VERSION\nvalue: \"1.27\"\n"
	cases := []struct {
		name       string
		src        string
		want       VariableDeclaration
		wantPath   string
		wantReason string
	}{
		{name: "name and value are read into the declaration", src: base, want: VariableDeclaration{Name: mustVariableName(t, "GO_VERSION"), Desired: setting.VariableValue("1.27")}},
		{name: "value of spaces is read without trimming", src: strings.Replace(base, `"1.27"`, `" "`, 1), want: VariableDeclaration{Name: mustVariableName(t, "GO_VERSION"), Desired: setting.VariableValue(" ")}},
		{name: "empty string value is rejected", src: strings.Replace(base, `"1.27"`, `""`, 1), wantPath: "V.value", wantReason: "must not be an empty string"},
		{name: "missing name is rejected", src: "value: \"1.27\"\n", wantPath: "V", wantReason: `missing key "name"`},
		{name: "missing value is rejected", src: "name: GO_VERSION\n", wantPath: "V", wantReason: `missing key "value"`},
		{name: "state is rejected as an unknown key", src: base + "state: absent\n", wantPath: "V", wantReason: `unknown key "state"; expected name, value`},
		{name: "name against the naming rule is rejected at the name", src: strings.Replace(base, "GO_VERSION", "go_version", 1), wantPath: "V.name", wantReason: `must be upper case; write "GO_VERSION"; got "go_version"`},
		{name: "unquoted boolean name is rejected as not a string without a quoting hint", src: strings.Replace(base, "GO_VERSION", "TRUE", 1), wantPath: "V.name", wantReason: "must be a string"},
		{name: "unquoted number value is rejected with a quoting hint", src: strings.Replace(base, `"1.27"`, "1.27", 1), wantPath: "V.value", wantReason: `must be a string; got 1.27; write "1.27" in quotes`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, err := doc.variable(doc.root, "V")
			if tc.wantPath != "" {
				if err == nil {
					t.Fatalf("variable() = %+v, want error", got)
				}
				if !strings.HasPrefix(err.Error(), tc.wantPath+" (") {
					t.Errorf("variable() error = %q, want it to start with %q", err, tc.wantPath+" (")
				}
				// 理由の後ろに直し方が続かないことも確かめるため、末尾で比べる
				if !strings.HasSuffix(err.Error(), tc.wantReason) {
					t.Errorf("variable() error = %q, want it to end with %q", err, tc.wantReason)
				}
				return
			}
			if err != nil {
				t.Fatalf("variable() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("variable() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestDocumentEnvironment は、一つの Environment の宣言が持つ規則 (name の形式、キーの集合、デプロイ元制限の省略) を確かめる。
// 拒否のケースは、受理される入力 (base) の条件を 1 つだけ変えて作る。
//
//trace:checks FR-001
func TestDocumentEnvironment(t *testing.T) {
	const base = "name: staging\ndeployment_branch_policy:\n  mode: all\n"
	cases := []struct {
		name       string
		src        string
		want       EnvironmentDeclaration
		wantPath   string
		wantReason string
	}{
		{
			name: "name with deployment_branch_policy is read into both fields",
			src:  base,
			want: EnvironmentDeclaration{Name: mustEnvironmentName(t, "staging"), DeploymentBranchPolicy: &setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyAll}},
		},
		{
			name: "name alone is read, leaving deployment_branch_policy omitted",
			src:  "name: staging\n",
			want: EnvironmentDeclaration{Name: mustEnvironmentName(t, "staging")},
		},
		{name: "missing name is rejected", src: "deployment_branch_policy:\n  mode: all\n", wantPath: "E", wantReason: `missing key "name"`},
		{name: "non-string name is rejected", src: strings.Replace(base, "staging", "42", 1), wantPath: "E.name", wantReason: "must be a string"},
		{name: "non-ASCII name is rejected at the name", src: strings.Replace(base, "staging", "café", 1), wantPath: "E.name", wantReason: `must consist of printable ASCII characters; got "café"`},
		{name: "unknown key is rejected", src: base + "variables: {}\n", wantPath: "E", wantReason: `unknown key "variables"; expected name, deployment_branch_policy`},
		{name: "invalid deployment_branch_policy is rejected at deployment_branch_policy", src: strings.Replace(base, "mode: all", "mode: custom", 1), wantPath: "E.deployment_branch_policy.mode", wantReason: "must be all, protected_branches, or selected"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, err := doc.environment(doc.root, "E")
			if tc.wantPath != "" {
				if err == nil {
					t.Fatalf("environment() = %+v, want error", got)
				}
				if !strings.HasPrefix(err.Error(), tc.wantPath+" (") {
					t.Errorf("environment() error = %q, want it to start with %q", err, tc.wantPath+" (")
				}
				if !strings.Contains(err.Error(), tc.wantReason) {
					t.Errorf("environment() error = %q, want it to contain %q", err, tc.wantReason)
				}
				return
			}
			if err != nil {
				t.Fatalf("environment() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("environment() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestDocumentDeploymentBranchPolicy は、デプロイ元制限の宣言の方式と patterns の組み合わせを確かめる。
//
//trace:checks FR-001
func TestDocumentDeploymentBranchPolicy(t *testing.T) {
	const base = "mode: selected\npatterns:\n  - type: branch\n    name: main\n"
	main := setting.DeploymentPattern{Type: setting.DeploymentPatternBranch, Name: "main"}
	cases := []struct {
		name       string
		src        string
		want       setting.DeploymentBranchPolicy
		wantReason string
	}{
		{name: "all is read without patterns", src: "mode: all\n", want: setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyAll}},
		{name: "protected_branches is read without patterns", src: "mode: protected_branches\n", want: setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyProtectedBranches}},
		{name: "selected with patterns is read into mode and patterns", src: base, want: setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicySelected, Patterns: []setting.DeploymentPattern{main}}},
		{name: "selected with empty patterns is read as an empty set", src: "mode: selected\npatterns: []\n", want: setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicySelected, Patterns: []setting.DeploymentPattern{}}},
		{name: "selected without patterns is rejected at the policy", src: "mode: selected\n", wantReason: `P (line 1, column 1): mode "selected" requires patterns`},
		{name: "selected with null patterns is rejected", src: "mode: selected\npatterns:\n", wantReason: "P.patterns (line 2, column 10): must not be null or empty"},
		{name: "all with patterns is rejected at patterns", src: "mode: all\npatterns:\n  - type: branch\n    name: main\n", wantReason: `P.patterns (line 3, column 3): patterns is allowed only when mode is "selected"; got mode "all"`},
		{name: "protected_branches with empty patterns is rejected at patterns", src: "mode: protected_branches\npatterns: []\n", wantReason: `P.patterns (line 2, column 11): patterns is allowed only when mode is "selected"; got mode "protected_branches"`},
		{name: "all with invalid patterns is rejected by the combination", src: "mode: all\npatterns:\n  - main\n", wantReason: `patterns is allowed only when mode is "selected"`},
		{name: "unknown mode with patterns is rejected at the mode", src: "mode: custom\npatterns: []\n", wantReason: `P.mode (line 1, column 7): must be all, protected_branches, or selected; got "custom"`},
		{name: "non-string mode is rejected", src: "mode: 1\n", wantReason: "P.mode (line 1, column 7): must be a string"},
		{name: "only patterns is rejected for the missing mode", src: "patterns: []\n", wantReason: `missing key "mode"`},
		{name: "unknown key is rejected", src: base + "id: 1\n", wantReason: `unknown key "id"; expected mode, patterns`},
		{name: "empty mapping is rejected", src: "{}\n", wantReason: "must not be empty"},
		{name: "invalid pattern is reported at its path", src: "mode: selected\npatterns:\n  - type: Branch\n    name: main\n", wantReason: "P.patterns[0].type (line 3, column 11): must be branch or tag"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, err := doc.deploymentBranchPolicy(doc.root, "P")
			if tc.wantReason != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantReason) {
					t.Errorf("deploymentBranchPolicy() error = %v, want it to contain %q", err, tc.wantReason)
				}
				return
			}
			if err != nil {
				t.Fatalf("deploymentBranchPolicy() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("deploymentBranchPolicy() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestDocumentDeploymentPatterns は、個別パターンの一覧の並び、空集合、重複の判定を確かめる。
//
//trace:checks FR-001
func TestDocumentDeploymentPatterns(t *testing.T) {
	branch := func(name string) setting.DeploymentPattern {
		return setting.DeploymentPattern{Type: setting.DeploymentPatternBranch, Name: name}
	}
	tag := func(name string) setting.DeploymentPattern {
		return setting.DeploymentPattern{Type: setting.DeploymentPatternTag, Name: name}
	}
	cases := []struct {
		name       string
		src        string
		want       []setting.DeploymentPattern
		wantReason string
	}{
		{name: "two patterns are read in declaration order", src: "- {type: branch, name: release/*}\n- {type: tag, name: main}\n", want: []setting.DeploymentPattern{branch("release/*"), tag("main")}},
		{name: "empty sequence is read as no patterns", src: "[]\n", want: []setting.DeploymentPattern{}},
		{name: "non-sequence is rejected", src: "type: branch\n", wantReason: "S (line 1, column 1): must be a sequence"},
		{name: "duplicate pair is rejected at the later element", src: "- {type: branch, name: main}\n- {type: branch, name: main}\n", wantReason: `S[1] (line 2, column 3): duplicate pattern branch "main"; same as S[0]`},
		{name: "same name with different type is accepted", src: "- {type: branch, name: main}\n- {type: tag, name: main}\n", want: []setting.DeploymentPattern{branch("main"), tag("main")}},
		{name: "names differing only in letter case are accepted", src: "- {type: branch, name: main}\n- {type: branch, name: MAIN}\n", want: []setting.DeploymentPattern{branch("main"), branch("MAIN")}},
		{name: "escaped spelling of the same name is a duplicate", src: "- {type: branch, name: main}\n- {type: branch, name: \"\\x6dain\"}\n", wantReason: `S[1] (line 2, column 3): duplicate pattern branch "main"; same as S[0]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, err := doc.deploymentPatterns(doc.root, "S")
			if tc.wantReason != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantReason) {
					t.Errorf("deploymentPatterns() error = %v, want it to contain %q", err, tc.wantReason)
				}
				return
			}
			if err != nil {
				t.Fatalf("deploymentPatterns() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("deploymentPatterns() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestDocumentDeploymentPattern は、個別パターン 1 件の type と name の規則を確かめる。
// 拒否のケースは、受理される入力 (base) の条件を 1 つだけ変えて作る。
//
//trace:checks FR-001
func TestDocumentDeploymentPattern(t *testing.T) {
	const base = "type: branch\nname: main\n"
	cases := []struct {
		name       string
		src        string
		want       setting.DeploymentPattern
		wantReason string
	}{
		{name: "branch is read", src: base, want: setting.DeploymentPattern{Type: setting.DeploymentPatternBranch, Name: "main"}},
		{name: "tag is read", src: "type: tag\nname: main\n", want: setting.DeploymentPattern{Type: setting.DeploymentPatternTag, Name: "main"}},
		{name: "missing type is rejected without defaulting to branch", src: "name: main\n", wantReason: `missing key "type"`},
		{name: "missing name is rejected", src: "type: branch\n", wantReason: `missing key "name"`},
		{name: "type with different letter case is rejected", src: "type: Branch\nname: main\n", wantReason: `P.type (line 1, column 7): must be branch or tag; got "Branch"`},
		{name: "non-string type is rejected", src: "type: 1\nname: main\n", wantReason: "P.type (line 1, column 7): must be a string"},
		{name: "empty name is rejected", src: "type: branch\nname: \"\"\n", wantReason: `P.name (line 2, column 7): must not be empty or have leading or trailing whitespace; got ""`},
		{name: "name with leading space is rejected", src: "type: branch\nname: \" main\"\n", wantReason: `must not be empty or have leading or trailing whitespace; got " main"`},
		{name: "name with trailing space is rejected", src: "type: branch\nname: \"main \"\n", wantReason: `must not be empty or have leading or trailing whitespace; got "main "`},
		{name: "name surrounded by no-break spaces is rejected", src: "type: branch\nname: \"\\u00a0main\\u00a0\"\n", wantReason: "must not be empty or have leading or trailing whitespace"},
		{name: "name with inner space is rejected", src: "type: branch\nname: \"rel ease\"\n", wantReason: `P.name (line 2, column 7): must not contain ASCII spaces or C0 control characters; got "rel ease"`},
		{name: "name with inner control character is rejected", src: "type: branch\nname: \"rel\\x1fease\"\n", wantReason: "must not contain ASCII spaces or C0 control characters"},
		{name: "name with inner no-break space is read as written", src: "type: branch\nname: \"rel\\u00a0ease\"\n", want: setting.DeploymentPattern{Type: setting.DeploymentPatternBranch, Name: "rel\u00a0ease"}},
		{name: "name with exclamation mark and DEL is read as written", src: "type: branch\nname: \"rel!\\x7fease\"\n", want: setting.DeploymentPattern{Type: setting.DeploymentPatternBranch, Name: "rel!\x7fease"}},
		{name: "non-string name is rejected", src: "type: branch\nname: 1.0\n", wantReason: "P.name (line 2, column 7): must be a string"},
		{name: "unknown key is rejected", src: base + "id: 1\n", wantReason: `unknown key "id"; expected type, name`},
		{name: "non-mapping element is rejected", src: "main\n", wantReason: "P (line 1, column 1): must be a mapping"},
		{name: "name is read as written", src: "type: branch\nname: Release/v*.*\n", want: setting.DeploymentPattern{Type: setting.DeploymentPatternBranch, Name: "Release/v*.*"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := decodeSnippet(t, tc.src)
			got, err := doc.deploymentPattern(doc.root, "P")
			if tc.wantReason != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantReason) {
					t.Errorf("deploymentPattern() error = %v, want it to contain %q", err, tc.wantReason)
				}
				return
			}
			if err != nil {
				t.Fatalf("deploymentPattern() error = %v", err)
			}
			if got != tc.want {
				t.Errorf("deploymentPattern() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func mustVariableName(t *testing.T, name string) setting.VariableName {
	t.Helper()
	variableName, err := setting.ParseVariableName(name)
	if err != nil {
		t.Fatalf("ParseVariableName(%q) error = %v", name, err)
	}
	return variableName
}
