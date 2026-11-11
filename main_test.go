package main

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// cmd のテストは Environment.Getenv を差し替えるため、main が実際の環境変数を渡す配線は実行ファイルでしか確かめられない。
// plan はトークンを選んでから設定ファイルを読むため、存在しない設定ファイルを指定すれば、どちらのエラーになるかで
// 環境変数が読まれたかを外部 API に接続せずに見分けられる。
func TestRun_ReadsTokenFromProcessEnvironment(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "reco")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", bin, ".") //nolint:gosec // bin はテストが作る一時ディレクトリの中のパスで、外部入力を含まない
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	const selectionFailure = "GH_TOKEN or GITHUB_TOKEN must be set"
	configPath := filepath.Join(t.TempDir(), "reco.yaml")
	cases := []struct {
		name string
		// env は、実行ファイルに渡す環境変数のすべて。nil は親プロセスの環境変数を引き継ぐため、空でも nil にしない。
		env []string
		// want は、どちらのエラーで止まったかを示す出力の一部。設定ファイルの読み込みの失敗は、文言を決める package に
		// 結び付けないよう、テストが渡したパスで見分ける。
		want string
	}{
		{name: "GH_TOKEN in the process environment passes token selection and fails loading the configuration", env: []string{"GH_TOKEN=token"}, want: configPath},
		{name: "no token in the process environment fails token selection", env: []string{}, want: selectionFailure},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			run := exec.CommandContext(t.Context(), bin, "plan", "--config", configPath) //nolint:gosec // bin はこのテストがビルドした実行ファイルで、外部入力を含まない
			run.Env = tc.env
			out, err := run.CombinedOutput()
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
				t.Fatalf("got error %v, want exit code 1 (output %q)", err, out)
			}
			if !strings.Contains(string(out), tc.want) {
				t.Errorf("got output %q, want it to contain %q", out, tc.want)
			}
		})
	}
}
