package cmd

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/tmknom/reco/internal/configuration"
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

// file は、--config のパスを workDir を基準に解決した設定ファイルを返す。
func (f *configFlag) file(workDir string) configuration.File {
	dir, name := resolveConfigPath(workDir, f.path)
	return configuration.File{Dir: os.DirFS(dir), Name: name, Path: f.path}
}

func resolveConfigPath(workDir, path string) (dir, name string) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(workDir, path)
	}
	return filepath.Dir(path), filepath.Base(path)
}
