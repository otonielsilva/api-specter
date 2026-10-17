package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/otonielsilva/api-specter/internal/config"
	"github.com/otonielsilva/api-specter/internal/zoo"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		stop()
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	if err := config.LoadEnvFile(".env"); err != nil {
		return err
	}

	addr := config.ZooAddr()

	fmt.Fprintf(os.Stderr, "zoo listening on %s\n", addr)
	return zoo.Run(ctx, addr)
}
