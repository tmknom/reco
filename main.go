package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/term"

	"github.com/tmknom/reco/cmd"
)

// version は、ビルド時に -ldflags "-X main.version=..." で埋め込む。
var version = ""

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "reco: get working directory: %v\n", err)
		return cmd.ExitFailure
	}

	env := cmd.Environment{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Stdin:  os.Stdin,
		// ModeCharDevice は /dev/null などの端末でない文字デバイスでも真になるため、端末の判定に使わない
		StdinIsTerminal: term.IsTerminal(int(os.Stdin.Fd())),
		Version:         version,
		WorkDir:         wd,
		Getenv:          os.Getenv,
	}
	return cmd.Execute(ctx, os.Args[1:], env)
}
