package cmd

import "testing"

//trace:checks FR-003
func TestSelectToken(t *testing.T) {
	cases := []struct {
		name        string
		ghToken     string
		githubToken string
		want        string
		wantErr     bool
	}{
		{name: "GH_TOKEN takes precedence over GITHUB_TOKEN", ghToken: "gh", githubToken: "github", want: "gh"},
		{name: "empty GH_TOKEN falls back to GITHUB_TOKEN", ghToken: "", githubToken: "github", want: "github"},
		{name: "whitespace-only GH_TOKEN is used without falling back", ghToken: " ", githubToken: "github", want: " "},
		{name: "both empty is an error", ghToken: "", githubToken: "", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GH_TOKEN", tc.ghToken)
			t.Setenv("GITHUB_TOKEN", tc.githubToken)
			got, err := selectToken()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("selectToken() = %q, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("selectToken() error = %v", err)
			}
			if got != tc.want {
				t.Errorf("selectToken() = %q, want %q", got, tc.want)
			}
		})
	}
}
