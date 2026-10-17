package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/otonielsilva/api-specter/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	err := cli.Run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	if err == nil {
		return
	}

	if !errors.Is(err, cli.ErrUsage) {
		fmt.Fprintln(os.Stderr, err)
	}

	if errors.Is(err, cli.ErrUsage) {
		stop()
		os.Exit(2)
	}
	stop()
	os.Exit(1)
}
