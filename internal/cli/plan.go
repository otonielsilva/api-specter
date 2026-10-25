package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/otonielsilva/api-specter/internal/contract"
)

// planCmd implements "specter plan". It is named planCmd so it does not
// collide with the internal/plan package once that is imported here.
func planCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	fs.SetOutput(stderr)

	spec := fs.String("spec", "", "path to the OpenAPI document (JSON or YAML)")
	out := fs.String("out", "", "path of the plan file to write (default: stdout)")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("plan: %w: %w", ErrUsage, err)
	}

	if *spec == "" {
		fmt.Fprintln(stderr, "plan: --spec is required")
		fs.Usage()
		return fmt.Errorf("plan: --spec is required: %w", ErrUsage)
	}

	doc, err := contract.ParseOpenAPI(ctx, spec)
	if err != nil {
		return fmt.Errorf("plan: load spec  -> %s: %w", *spec, err)
	}

	fmt.Fprintf(stderr, "plan: loaded %d paths from %s\n", len(doc.Operations), *spec)

	_ = out
	_ = stdout
	return errors.New("plan: not implemented yet")
}
