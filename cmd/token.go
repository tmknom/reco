package cmd

import (
	"errors"
	"os"
)

// selectToken は CLI 仕様の認証情報の選択規則に従ってトークンを選ぶ。
// 値をトリムしないのは、空白だけの値も利用者が明示した値として扱い、補完しないため。
func selectToken() (string, error) {
	if token := os.Getenv("GH_TOKEN"); token != "" {
		return token, nil
	}
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token, nil
	}
	return "", errors.New("GH_TOKEN or GITHUB_TOKEN must be set to a non-empty token")
}
