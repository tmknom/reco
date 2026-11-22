package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// privateRepositoryBody は repositoryBody の非公開のリポジトリである。
const privateRepositoryBody = `{
	"id": 1296269,
	"name": "repo",
	"full_name": "owner/repo",
	"owner": {"login": "owner"},
	"private": true,
	"html_url": "https://github.com/owner/repo",
	"allow_auto_merge": true,
	"delete_branch_on_merge": false,
	"allow_squash_merge": true,
	"allow_merge_commit": false,
	"allow_rebase_merge": true,
	"allow_update_branch": false,
	"permissions": {"admin": true, "push": true, "pull": true}
}`

//trace:checks FR-004
func TestDecodeRepository(t *testing.T) {
	const general = `"id": 1296269, "allow_auto_merge": true, "delete_branch_on_merge": false, "allow_squash_merge": true, "allow_merge_commit": false, "allow_rebase_merge": true, "allow_update_branch": false, "private": false`
	successes := []struct {
		name string
		body string
		want repositoryAttributes
	}{
		{
			name: "public repository is read with all fields",
			body: repositoryBody,
			want: repositoryAttributes{admin: true, securityAndAnalysis: json.RawMessage(repositorySecurityAndAnalysis)},
		},
		{name: "private repository is read with all fields", body: privateRepositoryBody, want: repositoryAttributes{private: true, admin: true}},
		{name: "false permissions.admin is no admin access", body: `{` + general + `, "permissions": {"admin": false, "pull": true}}`, want: repositoryAttributes{}},
		{name: "missing permissions is no admin access", body: `{` + general + `}`, want: repositoryAttributes{}},
		{name: "permissions without admin is no admin access", body: `{` + general + `, "permissions": {"pull": true}}`, want: repositoryAttributes{}},
		{
			name: "non-object security_and_analysis is carried uninterpreted",
			body: `{` + general + `, "security_and_analysis": "text"}`,
			want: repositoryAttributes{securityAndAnalysis: json.RawMessage(`"text"`)},
		},
	}
	for _, tc := range successes {
		t.Run(tc.name, func(t *testing.T) {
			got, attributes, err := decodeRepository([]byte(tc.body))
			if err != nil {
				t.Fatalf("decodeRepository() error = %v", err)
			}
			want := ObservationResult{RepositoryID: 1296269, General: General{
				AllowAutoMerge: true, DeleteBranchOnMerge: false, AllowSquashMerge: true, AllowMergeCommit: false, AllowRebaseMerge: true, AllowUpdateBranch: false,
			}}
			if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(attributes, tc.want) {
				t.Errorf("decodeRepository() = (%+v, %+v), want (%+v, %+v)", got, attributes, want, tc.want)
			}
		})
	}

	// 受理される本文から、対象のフィールド1つだけを欠かすか null にする。ほかのフィールドの欠落で先に失敗しないようにするため。
	const accepted = `"id": 1, "full_name": "o/r", "allow_auto_merge": true, "delete_branch_on_merge": true, ` +
		`"allow_squash_merge": true, "allow_merge_commit": true, "allow_rebase_merge": true, "allow_update_branch": true, "private": false`
	without := func(field string) string {
		return `{` + strings.Replace(accepted, fmt.Sprintf(`"%s": `, field), fmt.Sprintf(`"unrelated_%s": `, field), 1) + `}`
	}
	withNull := func(field string) string {
		return `{` + strings.Replace(accepted, fmt.Sprintf(`"%s": true`, field), fmt.Sprintf(`"%s": null`, field), 1) + `}`
	}
	failures := []struct {
		name        string
		body        string
		errContains string
	}{
		{name: "missing id is unexpected response", body: without("id"), errContains: "id"},
		{name: "missing allow_auto_merge is unexpected response", body: without("allow_auto_merge"), errContains: "allow_auto_merge"},
		{name: "missing delete_branch_on_merge is unexpected response", body: without("delete_branch_on_merge"), errContains: "delete_branch_on_merge"},
		{name: "missing allow_squash_merge is unexpected response", body: without("allow_squash_merge"), errContains: "allow_squash_merge"},
		{name: "missing allow_merge_commit is unexpected response", body: without("allow_merge_commit"), errContains: "allow_merge_commit"},
		{name: "missing allow_rebase_merge is unexpected response", body: without("allow_rebase_merge"), errContains: "allow_rebase_merge"},
		{name: "missing allow_update_branch is unexpected response", body: without("allow_update_branch"), errContains: "allow_update_branch"},
		{name: "null allow_auto_merge is unexpected response", body: withNull("allow_auto_merge"), errContains: "allow_auto_merge"},
		{name: "missing private is unexpected response", body: without("private"), errContains: "private"},
		{name: "non-JSON body is unexpected response", body: `<html>`},
		{name: "non-boolean permissions.admin is unexpected response", body: `{` + accepted + `, "permissions": {"admin": "yes"}}`},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := decodeRepository([]byte(tc.body))
			if !errors.Is(err, ErrUnexpectedResponse) {
				t.Fatalf("decodeRepository() error = %v, want %v", err, ErrUnexpectedResponse)
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Errorf("decodeRepository() error = %q, want it to contain %q", err, tc.errContains)
			}
		})
	}
}
