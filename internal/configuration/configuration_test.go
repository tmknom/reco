package configuration

import "testing"

// minimal は受理される最小の宣言。宣言全体を入力にするテストの基準にする。
const minimal = `repositories:
  - name: owner/repo
    general:
      allow_auto_merge: true
`

// decodeSnippet は、検証の各段階に渡す YAML 文書を src から作る。
func decodeSnippet(t *testing.T, src string) document {
	t.Helper()
	doc, err := decodeDocument([]byte(src))
	if err != nil {
		t.Fatalf("decodeDocument(%q) error = %v", src, err)
	}
	return doc
}
