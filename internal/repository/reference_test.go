package repository

import (
	"strings"
	"testing"
)

//trace:checks FR-001
func TestParseReference(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		wantOwner string
		wantRepo  string
		wantErr   string
	}{
		{
			name:      "OWNER/REPO is split into owner and repo",
			input:     "owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		{
			name:      "ASCII letters, digits, hyphen, underscore and dot are accepted",
			input:     "Owner-1/Repo_2.x",
			wantOwner: "Owner-1",
			wantRepo:  "Repo_2.x",
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
			got, err := ParseReference(tc.input)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("ParseReference(%q) error = %v, want it to contain %q", tc.input, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseReference(%q) error = %v", tc.input, err)
			}
			if got.Owner() != tc.wantOwner || got.Repo() != tc.wantRepo {
				t.Errorf("ParseReference(%q) = owner %q, repo %q, want owner %q, repo %q", tc.input, got.Owner(), got.Repo(), tc.wantOwner, tc.wantRepo)
			}
		})
	}
}

func TestReference_String(t *testing.T) {
	got := Reference{owner: "owner", repo: "repo"}.String()
	if got != "owner/repo" {
		t.Errorf("String() = %q, want %q", got, "owner/repo")
	}
}

//trace:checks FR-002
func TestReference_EqualFold(t *testing.T) {
	base := Reference{owner: "owner", repo: "repo"}
	cases := []struct {
		name  string
		other Reference
		want  bool
	}{
		{name: "owner differing only in letter case is equal", other: Reference{owner: "Owner", repo: "repo"}, want: true},
		{name: "repo differing only in letter case is equal", other: Reference{owner: "owner", repo: "REPO"}, want: true},
		{name: "different owner is not equal", other: Reference{owner: "other", repo: "repo"}, want: false},
		{name: "different repo is not equal", other: Reference{owner: "owner", repo: "other"}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := base.EqualFold(tc.other); got != tc.want {
				t.Errorf("%v.EqualFold(%v) = %t, want %t", base, tc.other, got, tc.want)
			}
		})
	}
}
