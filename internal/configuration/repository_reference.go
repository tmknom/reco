package configuration

import (
	"errors"
	"fmt"
	"strings"
)

// parseRepositoryReference は OWNER/REPO 形式の名前をリポジトリ指定に分ける。
// 文字を ASCII の英数字と -、_、. に限るのは、空白や制御文字に加え、URL やクローン用の表記の断片
// (:、@、%、?、#) と、URL パスのドットセグメントになる . と .. を後段に渡さないため。
func parseRepositoryReference(name string) (RepositoryReference, error) {
	owner, repo, found := strings.Cut(name, "/")
	if !found || strings.Contains(repo, "/") {
		return RepositoryReference{}, errors.New(`must be OWNER/REPO with exactly one "/"`)
	}
	if err := checkNameSegment("OWNER", owner); err != nil {
		return RepositoryReference{}, err
	}
	if err := checkNameSegment("REPO", repo); err != nil {
		return RepositoryReference{}, err
	}
	if strings.HasSuffix(strings.ToLower(repo), ".git") {
		return RepositoryReference{}, errors.New(`must not end with ".git"`)
	}
	return RepositoryReference{Owner: owner, Repo: repo}, nil
}

func checkNameSegment(label, segment string) error {
	switch {
	case segment == "":
		return fmt.Errorf("%s must not be empty", label)
	case strings.ContainsFunc(segment, isNotNameRune):
		return fmt.Errorf(`%s must consist of ASCII letters, digits, "-", "_" and "."`, label)
	case segment == "." || segment == "..":
		return fmt.Errorf(`%s must not be "." or ".."`, label)
	default:
		return nil
	}
}

func isNotNameRune(r rune) bool {
	switch {
	case 'a' <= r && r <= 'z', 'A' <= r && r <= 'Z', '0' <= r && r <= '9':
		return false
	case r == '-' || r == '_' || r == '.':
		return false
	default:
		return true
	}
}
