// Package repository は、リポジトリを名前で指し示すリポジトリ指定を扱う。
package repository

import (
	"errors"
	"fmt"
	"strings"
)

// Reference は OWNER/REPO 形式のリポジトリ指定。
// フィールドを公開しないのは、ゼロ値のほかは ParseReference の検証を通った値しか作れない状態にし、
// 受け取る側が形式を検証し直さずに URL に組み込めるようにするため。
type Reference struct {
	owner string
	repo  string
}

// Owner は所有者名を返す。
func (r Reference) Owner() string { return r.owner }

// Repo はリポジトリ名を返す。
func (r Reference) Repo() string { return r.repo }

// String は OWNER/REPO 形式の名前を返す。
func (r Reference) String() string {
	return r.owner + "/" + r.repo
}

// EqualFold は、所有者名とリポジトリ名を大文字小文字を区別せずに比べる。GitHub が両者をそう扱うため。
// ParseReference が名前を ASCII に限るため、検証済みの名前では ASCII 英字だけの同一視になる。
func (r Reference) EqualFold(o Reference) bool {
	return strings.EqualFold(r.owner, o.owner) && strings.EqualFold(r.repo, o.repo)
}

// ParseReference は OWNER/REPO 形式の名前をリポジトリ指定に分ける。
// 文字を ASCII の英数字と -、_、. に限るのは、空白や制御文字に加え、URL やクローン用の表記の断片
// (:、@、%、?、#) と、URL パスのドットセグメントになる . と .. を後段に渡さないため。
func ParseReference(name string) (Reference, error) {
	owner, repo, found := strings.Cut(name, "/")
	if !found || strings.Contains(repo, "/") {
		return Reference{}, errors.New(`must be OWNER/REPO with exactly one "/"`)
	}
	if err := checkNameSegment("OWNER", owner); err != nil {
		return Reference{}, err
	}
	if err := checkNameSegment("REPO", repo); err != nil {
		return Reference{}, err
	}
	if strings.HasSuffix(strings.ToLower(repo), ".git") {
		return Reference{}, errors.New(`must not end with ".git"`)
	}
	return Reference{owner: owner, repo: repo}, nil
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
