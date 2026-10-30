package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/otonielsilva/api-specter/internal/contract"
	"github.com/otonielsilva/api-specter/internal/plan"
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

	doc, err := contract.ParseOpenAPI(ctx, *spec)
	if err != nil {
		return fmt.Errorf("plan: %w", err)
	}

	raw, err := os.ReadFile(*spec)
	if err != nil {
		return fmt.Errorf("plan: read spec %s: %w", *spec, err)
	}
	p := plan.Build(doc, *spec, raw)

	fmt.Fprintf(stderr, "plan created %d paths from %s\n", len(doc.Operations), *spec)

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("plan: marshal: %w", err)
	}
	data = append(data, '\n')

	if *out == "" {
		_, err := stdout.Write(data)
		return err
	}
	return os.WriteFile(*out, data, 0o600) //nolint:gosec // the path comes from --out on purpose
}
