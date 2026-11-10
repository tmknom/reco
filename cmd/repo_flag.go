package cmd

import (
	"errors"

	"github.com/tmknom/reco/internal/repository"
)

// repoFlag は --repo の値で、pflag.Value を満たす。
// flag の解析中に形式を検証するのは、不正な値を処理の開始前に使い方の誤りとするため。
type repoFlag struct {
	reference repository.Reference
	set       bool
}

// String は未設定なら空を返す。pflag は登録時の String() を既定値として help に出し、空のときだけ省くため。
func (f *repoFlag) String() string {
	if !f.set {
		return ""
	}
	return f.reference.String()
}

// Type は help に --repo OWNER/REPO と表示させる。
func (f *repoFlag) Type() string { return "OWNER/REPO" }

func (f *repoFlag) selection() *repository.Reference {
	if !f.set {
		return nil
	}
	return &f.reference
}

// Set は 2 回目の指定を拒否する。pflag の既定は後の値で上書きするが、CLI 仕様は --repo を一回の実行に一つだけ認めるため。
func (f *repoFlag) Set(s string) error {
	if f.set {
		return errors.New("may be specified only once")
	}
	reference, err := repository.ParseReference(s)
	if err != nil {
		return err
	}
	f.reference = reference
	f.set = true
	return nil
}
