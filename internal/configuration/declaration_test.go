package configuration

import (
	"reflect"
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

func mustReference(t *testing.T, name string) repository.Reference {
	t.Helper()
	reference, err := repository.ParseReference(name)
	if err != nil {
		t.Fatalf("ParseReference(%q) error = %v", name, err)
	}
	return reference
}

// TestDocumentPermissions は、allowed_actions と selected_actions の組み合わせの規則と、判定の順序を確かめる。
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
		{name: "unknown key is rejected", src: selected + "enabled: true\n", wantPath: "P", wantReason: `unknown key "enabled"; expected allowed_actions, selected_actions`},
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
		// リポジトリごとの宣言
		{
			name:       "name alone is rejected for lacking both general and actions",
			src:        strings.Replace(second, "    general:\n      allow_auto_merge: false\n", "", 1),
			wantPath:   "repositories[1]",
			wantReason: "must have at least one of general, actions",
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
			name:       "unknown key in actions is rejected listing permissions and workflow",
			src:        second + "    actions:\n      secrets: {}\n",
			wantPath:   "repositories[1].actions",
			wantReason: `unknown key "secrets"; expected permissions, workflow`,
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
