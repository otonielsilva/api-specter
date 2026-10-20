package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/otonielsilva/api-specter/internal/contract"
	"github.com/otonielsilva/api-specter/internal/plan"
)

// runCmd implements "specter run".
func runCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)

	planFile := fs.String("plan", "", "path to the plan file")
	spec := fs.String("spec", "", "path to the OpenAPI document (overrides spec.path in the plan)")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("run: %w: %w", ErrUsage, err)
	}

	if *planFile == "" {
		fmt.Fprintln(stderr, "run: --plan is required")
		fs.Usage()
		return fmt.Errorf("run: --plan is required: %w", ErrUsage)
	}

	p, err := loadPlan(*planFile)
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}

	specPath := specLocation(*planFile, p.Spec.Path, *spec)

	doc, err := contract.ParseOpenAPI(ctx, &specPath)
	if err != nil {
		return fmt.Errorf("run: load spec %s: %w", specPath, err)
	}

	fmt.Fprintf(stdout, "plan %s: %d operations; spec %s: %d paths\n",
		*planFile, len(p.Order), specPath, doc.Paths.Len())

	return nil
}

// loadPlan opens the plan file and decodes it.
func loadPlan(path string) (*plan.Plan, error) {
	f, err := os.Open(path) //nolint:gosec // the path comes from --plan on purpose
	if err != nil {
		return nil, fmt.Errorf("open plan: %w", err)
	}
	defer f.Close()

	p, err := plan.Load(f)
	if err != nil {
		return nil, fmt.Errorf("load plan %s: %w", path, err)
	}

	return p, nil
}

// specLocation decides which spec file to read: the --spec flag wins; else
// the path stored in the plan, resolved from the plan's directory unless it
// is absolute.
func specLocation(planFile, inPlan, override string) string {
	switch {
	case override != "":
		return override
	case filepath.IsAbs(inPlan):
		return inPlan
	default:
		return filepath.Join(filepath.Dir(planFile), inPlan)
	}
}
