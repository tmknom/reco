package configuration

import (
	"reflect"
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/repository"
)

func mustReference(t *testing.T, name string) repository.Reference {
	t.Helper()
	reference, err := repository.ParseReference(name)
	if err != nil {
		t.Fatalf("ParseReference(%q) error = %v", name, err)
	}
	return reference
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
		// リポジトリごとの宣言
		{
			name:       "actions is rejected as an unknown key",
			src:        second + "    actions:\n      workflow: {}\n",
			wantPath:   "repositories[1]",
			wantReason: `unknown key "actions"`,
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
