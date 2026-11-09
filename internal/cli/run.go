package cli

import (
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/otonielsilva/api-specter/internal/contract"
	"github.com/otonielsilva/api-specter/internal/plan"
)

// runCmd implements "specter run".
func runCmd(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)

	planFile := fs.String("plan", "", "path to the plan file")
	spec := fs.String("spec", "", "path to the OpenAPI document (overrides spec.path in the plan)")
	baseURL := fs.String("base-url", "", "base URL of the API under test (absolute http or https URL)")
	seed := fs.Int64("seed", 0, "seed for random choices (0 picks one from the clock and prints it)")
	budget := fs.Int("budget", 0, "maximum number of requests (0 means no limit)")

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

	if *baseURL == "" {
		fmt.Fprintln(stderr, "run: --base-url is required")
		fs.Usage()
		return fmt.Errorf("run: --base-url is required: %w", ErrUsage)
	}

	base, err := parseBaseURL(*baseURL)
	if err != nil {
		fmt.Fprintf(stderr, "run: --base-url: %v\n", err)
		fs.Usage()
		return fmt.Errorf("run: --base-url: %w: %w", ErrUsage, err)
	}

	if *budget < 0 {
		fmt.Fprintln(stderr, "run: --budget must not be negative")
		fs.Usage()
		return fmt.Errorf("run: --budget must not be negative: %w", ErrUsage)
	}

	p, err := loadPlan(*planFile)
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}

	err = plan.Validate(p) // Validate order of the plan file
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}

	specPath := specLocation(*planFile, p.Spec.Path, *spec)

	doc, err := contract.ParseOpenAPI(ctx, specPath)
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}

	// 0 means "pick one". The seed is printed so a failing run can be
	// replayed with --seed.
	runSeed := cmp.Or(*seed, time.Now().UnixNano())

	fmt.Fprintf(stdout, "plan %s: %d operations; spec %s: %d paths; target %s; seed %d; budget %d\n",
		*planFile, len(p.Order), specPath, len(doc.Operations), base, runSeed, *budget)

	return nil
}

// parseBaseURL checks that raw is an absolute http or https URL with a host.
// url.Parse alone is not enough: it accepts "localhost:8080" and reads
// "localhost" as the scheme, so the scheme and the host are checked here.
func parseBaseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse %q: %w", raw, err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("%q is not an absolute http or https URL", raw)
	}
	return u, nil
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
