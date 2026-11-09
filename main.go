package main

import (
	"context"
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

	env := cmd.Environment{
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		Version: version,
	}
	return cmd.Execute(ctx, os.Args[1:], env)
}
