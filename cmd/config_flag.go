package cmd

import (
	"errors"
	"path/filepath"
)

const defaultConfigPath = "./reco.yaml"

// configFlag は --config の値で、pflag.Value を満たす。
// 空のパスを flag の解析中に拒否するのは、エラーに pflag が flag 名を添えるため。
type configFlag struct {
	path string
}

func newConfigFlag() *configFlag {
	return &configFlag{path: defaultConfigPath}
}

func (f *configFlag) String() string { return f.path }

// Type は help に --config PATH と表示させる。
func (f *configFlag) Type() string { return "PATH" }

func (f *configFlag) Set(s string) error {
	if s == "" {
		return errors.New("must not be empty")
	}
	f.path = s
	return nil
}

func resolveConfigPath(workDir, path string) (dir, name string) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(workDir, path)
	}
	return filepath.Dir(path), filepath.Base(path)
}
