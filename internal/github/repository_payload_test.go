package github

import (
	"errors"
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
	"delete_branch_on_merge": false
}`

//trace:checks FR-004
func TestDecodeRepository(t *testing.T) {
	successes := []struct {
		name        string
		body        string
		wantPrivate bool
	}{
		{name: "public repository is read with all fields", body: repositoryBody, wantPrivate: false},
		{name: "private repository is read with all fields", body: privateRepositoryBody, wantPrivate: true},
	}
	for _, tc := range successes {
		t.Run(tc.name, func(t *testing.T) {
			got, private, err := decodeRepository([]byte(tc.body))
			if err != nil {
				t.Fatalf("decodeRepository() error = %v", err)
			}
			want := ObservationResult{RepositoryID: 1296269, General: General{AllowAutoMerge: true, DeleteBranchOnMerge: false}}
			if got != want || private != tc.wantPrivate {
				t.Errorf("decodeRepository() = (%+v, %t), want (%+v, %t)", got, private, want, tc.wantPrivate)
			}
		})
	}

	failures := []struct {
		name        string
		body        string
		errContains string
	}{
		{name: "missing id is unexpected response", body: `{"full_name": "o/r", "allow_auto_merge": true, "delete_branch_on_merge": true, "private": false}`, errContains: "id"},
		{name: "missing allow_auto_merge is unexpected response", body: `{"id": 1, "full_name": "o/r", "delete_branch_on_merge": true, "private": false}`, errContains: "allow_auto_merge"},
		{name: "missing delete_branch_on_merge is unexpected response", body: `{"id": 1, "full_name": "o/r", "allow_auto_merge": true, "private": false}`, errContains: "delete_branch_on_merge"},
		{name: "null allow_auto_merge is unexpected response", body: `{"id": 1, "full_name": "o/r", "allow_auto_merge": null, "delete_branch_on_merge": true, "private": false}`, errContains: "allow_auto_merge"},
		{name: "missing private is unexpected response", body: `{"id": 1, "full_name": "o/r", "allow_auto_merge": true, "delete_branch_on_merge": true}`, errContains: "private"},
		{name: "non-JSON body is unexpected response", body: `<html>`},
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
