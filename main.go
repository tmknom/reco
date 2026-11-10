package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

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
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		Version: version,
		WorkDir: wd,
	}
	return cmd.Execute(ctx, os.Args[1:], env)
}
