package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/ncphillips/rex-diff-guide/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := cli.Run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "rgd:", err)
		os.Exit(1)
	}
}
