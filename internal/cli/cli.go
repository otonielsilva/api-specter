package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
)

// ErrUsage reports that the command line was not valid. main can map it to
// a different exit code than runtime failures.
var ErrUsage = errors.New("usage error")

// Run executes specter with the given arguments (without the program name).
// It takes its context and writers explicitly so tests can drive it without
// starting a process.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if len(args) == 0 {
		usage(stderr)
		return ErrUsage
	}

	switch args[0] {
	case "help", "-h", "--help":
		usage(stdout)
		return nil
	default:
		fmt.Fprintf(stderr, "specter: unknown command %q\n\n", args[0])
		usage(stderr)
		return ErrUsage
	}
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: specter <command> [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  help    show this message")
}
