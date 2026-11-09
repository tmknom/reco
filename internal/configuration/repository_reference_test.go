package configuration

import (
	"strings"
	"testing"
)

//trace:checks FR-001
func TestParseRepositoryReference(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    RepositoryReference
		wantErr string
	}{
		{
			name:  "OWNER/REPO is split into owner and repo",
			input: "owner/repo",
			want:  RepositoryReference{Owner: "owner", Repo: "repo"},
		},
		{
			name:  "ASCII letters, digits, hyphen, underscore and dot are accepted",
			input: "Owner-1/Repo_2.x",
			want:  RepositoryReference{Owner: "Owner-1", Repo: "Repo_2.x"},
		},
		{
			name:    "name without slash is rejected",
			input:   "owner",
			wantErr: `exactly one "/"`,
		},
		{
			name:    "name with two slashes is rejected",
			input:   "owner/repo/x",
			wantErr: `exactly one "/"`,
		},
		{
			name:    "URL is rejected",
			input:   "https://github.com/owner/repo",
			wantErr: `exactly one "/"`,
		},
		{
			name:    "empty OWNER is rejected",
			input:   "/repo",
			wantErr: "OWNER must not be empty",
		},
		{
			name:    "empty REPO is rejected",
			input:   "owner/",
			wantErr: "REPO must not be empty",
		},
		{
			name:    "space is rejected",
			input:   "own er/repo",
			wantErr: "OWNER must consist of",
		},
		{
			name:    "control character is rejected",
			input:   "owner/re\u0001po",
			wantErr: "REPO must consist of",
		},
		{
			name:    "non-ASCII letter is rejected",
			input:   "owner/répo",
			wantErr: "REPO must consist of",
		},
		{
			name:    "clone notation with scp-like host is rejected",
			input:   "git@github.com:owner/repo",
			wantErr: "OWNER must consist of",
		},
		{
			name:    "single dot REPO is rejected",
			input:   "owner/.",
			wantErr: `REPO must not be "." or ".."`,
		},
		{
			name:    "dot segment REPO is rejected",
			input:   "owner/..",
			wantErr: `REPO must not be "." or ".."`,
		},
		{
			name:    ".git suffix is rejected",
			input:   "owner/repo.git",
			wantErr: `must not end with ".git"`,
		},
		{
			name:    "uppercase .GIT suffix is rejected",
			input:   "owner/repo.GIT",
			wantErr: `must not end with ".git"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseRepositoryReference(tc.input)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("parseRepositoryReference(%q) error = %v, want it to contain %q", tc.input, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseRepositoryReference(%q) error = %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("parseRepositoryReference(%q) = %+v, want %+v", tc.input, got, tc.want)
			}
		})
	}
}
