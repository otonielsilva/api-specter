// Command specter is a contract-driven HTTP API fuzzer.
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
	err := cli.Run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()

	switch {
	case err == nil:
		return
	case errors.Is(err, cli.ErrUsage):
		os.Exit(2)
	default:
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
