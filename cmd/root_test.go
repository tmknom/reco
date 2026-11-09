package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/github"
)

func runExecute(t *testing.T, version string, opts []github.Option, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	env := Environment{Stdout: &out, Stderr: &errOut, Version: version}
	code = execute(context.Background(), args, env, opts...)
	return code, out.String(), errOut.String()
}

// errorRecords は、stderr の JSON 行のうち ERROR レベルのレコードを返す。使い方などの JSON でない行は読み飛ばす。
func errorRecords(t *testing.T, stderr string) []map[string]any {
	t.Helper()
	var got []map[string]any
	for line := range strings.SplitSeq(stderr, "\n") {
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("got invalid JSON line %q: %v", line, err)
		}
		if m["level"] == "ERROR" {
			got = append(got, m)
		}
	}
	return got
}

func TestExecute_Success(t *testing.T) {
	cases := []struct {
		name    string
		version string
		args    []string
		stdout  string
	}{
		{name: "no arguments shows help on stdout", args: nil, stdout: "Usage:"},
		{name: "--help shows help on stdout", args: []string{"--help"}, stdout: "Usage:"},
		// 引数のない root は RunE でも help を出すため、位置引数を添えて help flag として効いたことを確かめる
		{name: "-h shows help on stdout", args: []string{"foo", "-h"}, stdout: "Usage:"},
		{name: "--help ignores positional arguments", args: []string{"foo", "--help"}, stdout: "Usage:"},
		{name: "--version prints the embedded version", version: "v1.2.3", args: []string{"--version"}, stdout: "v1.2.3"},
		{name: "-v prints the embedded version", version: "v1.2.3", args: []string{"-v"}, stdout: "v1.2.3"},
		{name: "--version ignores positional arguments", version: "v1.2.3", args: []string{"foo", "--version"}, stdout: "v1.2.3"},
		{name: "--version without embedded version prints dev", version: "", args: []string{"--version"}, stdout: "dev"},
		{name: "--log-level is accepted", args: []string{"--log-level", "debug"}, stdout: "Usage:"},
		{name: "show --help shows show usage without --repo", args: []string{"show", "--help"}, stdout: "reco show"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := runExecute(t, tc.version, nil, tc.args...)
			if code != 0 {
				t.Errorf("got exit code %d, want 0 (stderr %q)", code, stderr)
			}
			if !strings.Contains(stdout, tc.stdout) {
				t.Errorf("got stdout %q, want it to contain %q", stdout, tc.stdout)
			}
			if stderr != "" {
				t.Errorf("got stderr %q, want empty", stderr)
			}
		})
	}
}

// 他のテストは接続先を差し替えるため execute を呼ぶ。
func TestExecute_PassesArgsAndEnvironment(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Execute(context.Background(), []string{"--version"}, Environment{Stdout: &out, Stderr: &errOut, Version: "v1.2.3"})
	if code != 0 {
		t.Errorf("got exit code %d, want 0 (stderr %q)", code, errOut.String())
	}
	if !strings.Contains(out.String(), "v1.2.3") {
		t.Errorf("got stdout %q, want it to contain %q", out.String(), "v1.2.3")
	}
	if errOut.String() != "" {
		t.Errorf("got stderr %q, want empty", errOut.String())
	}
}

func TestExecute_UsageError(t *testing.T) {
	cases := []struct {
		name string
		args []string
		// token は、エラーの対象として stderr にちょうど 1 回だけ現れる語。cobra のエラー表示が重なると 2 回になる。
		// 使い方の文面にも現れる語のときは空にする。
		token string
	}{
		{name: "unknown flag", args: []string{"--invalid"}, token: "--invalid"},
		{name: "flag without value", args: []string{"--log-level"}},
		{name: "uppercase log level is rejected", args: []string{"--log-level", "INFO"}, token: "INFO"},
		{name: "log level with surrounding space is rejected", args: []string{"--log-level", "info "}},
		{name: "unknown log level name is rejected", args: []string{"--log-level", "verbose"}, token: "verbose"},
		{name: "invalid log level with --help is rejected", args: []string{"--log-level", "verbose", "--help"}, token: "verbose"},
		{name: "extra positional argument", args: []string{"foo"}, token: "foo"},
		{name: "completion command is not provided", args: []string{"completion"}, token: "completion"},
		// help は使い方の文面にも現れるため token を空にする
		{name: "help command is not provided", args: []string{"help"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := runExecute(t, "", nil, tc.args...)
			if code != 1 {
				t.Errorf("got exit code %d, want 1", code)
			}
			if stdout != "" {
				t.Errorf("got stdout %q, want empty", stdout)
			}
			records := errorRecords(t, stderr)
			if len(records) != 1 || records[0]["msg"] != "command failed" {
				t.Errorf("got ERROR records %v, want one command failed record", records)
			}
			if !strings.Contains(stderr, "Usage:") {
				t.Errorf("got stderr %q, want usage", stderr)
			}
			if tc.token != "" && strings.Count(stderr, tc.token) != 1 {
				t.Errorf("got stderr %q, want %q exactly once", stderr, tc.token)
			}
		})
	}
}

// CLI 仕様のコマンド一覧が外部契約のため、help に cobra の自動で加えるコマンドや話題が現れないことを確かめる。
func TestExecute_HelpListsOnlySpecifiedCommands(t *testing.T) {
	_, stdout, _ := runExecute(t, "", nil, "--help")
	for _, unwanted := range []string{"Additional help topics", "Help about any command"} {
		if strings.Contains(stdout, unwanted) {
			t.Errorf("got stdout %q, want it not to contain %q", stdout, unwanted)
		}
	}
}
