package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

const declaredRepository = `repositories:
  - name: owner/repo
    general:
      allow_auto_merge: true
`

const declaredTwoRepositories = `repositories:
  - name: owner/repo
    general:
      allow_auto_merge: true
  - name: owner/second
    general:
      allow_auto_merge: true
`

func writeConfig(t *testing.T, dir, name, src string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}
