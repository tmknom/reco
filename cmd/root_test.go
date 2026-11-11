package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/github"
)

func runExecute(t *testing.T, env Environment, opts []github.Option, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	env.Stdout = &out
	env.Stderr = &errOut
	code = execute(context.Background(), args, env, opts...)
	return code, out.String(), errOut.String()
}

// errorRecords は、stderr の JSON 行のうち ERROR レベルのレコードを返す。
func errorRecords(t *testing.T, stderr string) []map[string]any {
	t.Helper()
	var got []map[string]any
	for line := range strings.SplitSeq(stderr, "\n") {
		if line == "" {
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
		{name: "-h shows help on stdout", args: []string{"-h"}, stdout: "Usage:"},
		{name: "--version prints the embedded version", version: "v1.2.3", args: []string{"--version"}, stdout: "v1.2.3"},
		{name: "-v prints the embedded version", version: "v1.2.3", args: []string{"-v"}, stdout: "v1.2.3"},
		{name: "--version without embedded version prints dev", version: "", args: []string{"--version"}, stdout: "dev"},
		{name: "show --help shows show usage without --repo", args: []string{"show", "--help"}, stdout: "reco show"},
		{name: "plan --help shows plan usage with the default --config", args: []string{"plan", "--help"}, stdout: "(default ./reco.yaml)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := runExecute(t, Environment{Version: tc.version}, nil, tc.args...)
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

func TestExecute_InvalidInput(t *testing.T) {
	cases := []struct {
		name string
		args []string
		// token は、エラー文言が対象として含む語。
		token string
	}{
		{name: "unknown flag", args: []string{"--invalid"}, token: "--invalid"},
		{name: "flag without value", args: []string{"--log-level"}, token: "--log-level"},
		{name: "unknown log level", args: []string{"--log-level", "verbose"}, token: "verbose"},
		{name: "unknown command", args: []string{"foo"}, token: "foo"},
		// 未定義のコマンドは flag の解析より前にエラーになるため、解析が済んでから失敗する必須 flag の欠落で確かめる
		{name: "log level above error does not hide the failure", args: []string{"--log-level", "error+4", "show"}, token: "repo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := runExecute(t, Environment{}, nil, tc.args...)
			if code != 1 {
				t.Errorf("got exit code %d, want 1", code)
			}
			if stdout != "" {
				t.Errorf("got stdout %q, want empty", stdout)
			}
			// errorRecords は JSON でない行で失敗するため、cobra が自分でエラーや使い方を書けばここで検出する
			records := errorRecords(t, stderr)
			if len(records) != 1 || records[0]["msg"] != "command failed" {
				t.Fatalf("got ERROR records %v, want one command failed record", records)
			}
			if msg, _ := records[0]["error"].(string); !strings.Contains(msg, tc.token) {
				t.Errorf("got error %q, want it to contain %q", msg, tc.token)
			}
		})
	}
}

func TestNewRootCmd_LogLevel(t *testing.T) {
	level := new(slog.LevelVar)
	root := newRootCmd("", "", level)
	root.SetArgs([]string{"--log-level", "debug"})
	root.SetOut(io.Discard)
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := level.Level(); got != slog.LevelDebug {
		t.Errorf("got level %v, want %v", got, slog.LevelDebug)
	}
}
