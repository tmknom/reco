package cmd

import (
	"testing"
)

func TestConfigFlag_Set(t *testing.T) {
	t.Run("value replaces the default path", func(t *testing.T) {
		f := newConfigFlag()
		if err := f.Set("custom.yaml"); err != nil {
			t.Fatalf("Set() error = %v", err)
		}
		if f.String() != "custom.yaml" {
			t.Errorf("String() = %q, want %q", f.String(), "custom.yaml")
		}
	})

	t.Run("empty value is rejected", func(t *testing.T) {
		f := newConfigFlag()
		if err := f.Set(""); err == nil {
			t.Errorf("Set(%q) error = nil, want error", "")
		}
	})
}

//trace:checks FR-001
func TestResolveConfigPath(t *testing.T) {
	const workDir = "/work"
	cases := []struct {
		name     string
		path     string
		wantDir  string
		wantName string
	}{
		{name: "default path is resolved against the working directory", path: "./reco.yaml", wantDir: "/work", wantName: "reco.yaml"},
		{name: "relative path to the parent is resolved against the working directory", path: "../custom.yaml", wantDir: "/", wantName: "custom.yaml"},
		{name: "absolute path ignores the working directory", path: "/etc/reco/custom.yaml", wantDir: "/etc/reco", wantName: "custom.yaml"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir, name := resolveConfigPath(workDir, tc.path)
			if dir != tc.wantDir || name != tc.wantName {
				t.Errorf("resolveConfigPath(%q, %q) = (%q, %q), want (%q, %q)", workDir, tc.path, dir, name, tc.wantDir, tc.wantName)
			}
		})
	}
}
