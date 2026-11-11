package cmd

import "testing"

//trace:checks FR-003
func TestSelectToken(t *testing.T) {
	cases := []struct {
		name    string
		env     map[string]string
		want    string
		wantErr bool
	}{
		{name: "GH_TOKEN takes precedence over GITHUB_TOKEN", env: map[string]string{"GH_TOKEN": "gh", "GITHUB_TOKEN": "github"}, want: "gh"},
		{name: "empty GH_TOKEN falls back to GITHUB_TOKEN", env: map[string]string{"GH_TOKEN": "", "GITHUB_TOKEN": "github"}, want: "github"},
		{name: "whitespace-only GH_TOKEN is used without falling back", env: map[string]string{"GH_TOKEN": " ", "GITHUB_TOKEN": "github"}, want: " "},
		{name: "both empty is an error", env: map[string]string{"GH_TOKEN": "", "GITHUB_TOKEN": ""}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := selectToken(fakeGetenv(tc.env))
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
