package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/otonielsilva/api-specter/internal/contract"
	"github.com/otonielsilva/api-specter/internal/generator"
	"github.com/otonielsilva/api-specter/internal/oracle"
	"github.com/otonielsilva/api-specter/internal/plan"
)

// ErrNotImplemented is returned while the executor is only a skeleton.
var ErrNotImplemented = errors.New("executor: not implemented")

// ErrUnknownOperation is returned when the plan lists an operation that the
// contract does not have, which means the plan is stale.
var ErrUnknownOperation = errors.New("operation not in the contract")

// defaultRequestTimeout is the deadline of a request when
// Config.RequestTimeout is not set.
const defaultRequestTimeout = 10 * time.Second

// ErrNoBaseURL is returned when Config.BaseURL is nil.
var ErrNoBaseURL = errors.New("executor: no base URL")

// Config holds what a run needs besides the plan and the contract.
type Config struct {
	// BaseURL is the API under test.
	BaseURL *url.URL
	// Seed makes the random choices of a run reproducible.
	Seed int64
	// Budget is the maximum number of requests; 0 means no limit.
	Budget int
	// Client sends the requests. A nil Client means a default one with a
	// timeout, never http.DefaultClient.
	Client *http.Client
	// RequestTimeout is the deadline of each request. Zero or negative means
	// defaultRequestTimeout.
	RequestTimeout time.Duration
	// Oracles judge every exchange.
	Oracles []oracle.Oracle
	// IncludeDestructive sends the operations tagged plan.TagDestructive,
	// such as every DELETE. Without it they are skipped.
	IncludeDestructive bool
}

// Executor runs a plan against a target API.
type Executor struct {
	cfg Config
}

// New returns an Executor for cfg.
func New(cfg Config) *Executor {
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: 30 * time.Second}
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = defaultRequestTimeout
	}
	return &Executor{cfg: cfg}
}

// Run walks the operations of the plan in its Order and returns what the
// oracles found.
//
// For each operation id in the order, Run will:
//
//  1. skip it when its Operator is not Enabled;
//  2. look up the operation in the contract, which has the parameters and
//     schemas;
//  3. fill each parameter: from a binding captured by an earlier response
//     when the plan has a Requirement for it, from a seed when it is in
//     SeedsNeeded, else from a generated value;
//  4. build the request against Config.BaseURL and send it with Config.Client
//     under a per-request deadline derived from ctx;
//  5. capture the values the plan expects from the response, so the
//     operations that depend on this one can use them;
//  6. hand the exchange (request and response) to every oracle and collect
//     the findings;
//  7. stop when Config.Budget requests were sent or ctx is cancelled.
//
// A failed request is not a failed run: transport errors and non-2xx
// responses are exchanges for the oracles to judge. Run returns an error
// only when it cannot continue.
func (e *Executor) Run(ctx context.Context, p *plan.Plan, c *contract.Contract) ([]oracle.Finding, error) {
	// Check the config before using it: a nil BaseURL would panic in do.
	if e.cfg.BaseURL == nil {
		return nil, ErrNoBaseURL
	}

	// Not an error: the requests are still sent, but nothing judges them.
	if len(e.cfg.Oracles) == 0 {
		slog.Warn("no oracles configured, responses will not be judged")
	}

	// Index the contract once so each operation is a map lookup, not a scan
	// of the slice.
	byID := make(map[string]contract.Operation, len(c.Operations))
	for _, op := range c.Operations {
		byID[op.BuildID()] = op
	}
	findings := []oracle.Finding{}

	for _, id := range p.Order {
		// Stop when the run is cancelled; otherwise every remaining request
		// would fail and be reported as a finding.
		if err := ctx.Err(); err != nil {
			return findings, fmt.Errorf("executor: %w", err)
		}

		operator := p.Operations[id]
		if !operator.Enabled {
			continue
		}

		if !e.cfg.IncludeDestructive && slices.Contains(operator.Tags, plan.TagDestructive) {
			slog.Debug("skipping destructive operation, use --include-destructive", "operation", id)
			continue
		}

		// A missing operation means the plan is stale against the spec.
		op, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("executor: %w: %q", ErrUnknownOperation, id)
		}

		if strings.Contains(op.Path, "{") {
			slog.Debug("skipping operation", "path", op.Path)
			continue
		}

		exchange := e.do(ctx, op)
		if exchange.Err != nil {
			// Not fatal: a transport error or a timeout is an exchange, and
			// the oracles judge it like any other.
			slog.Debug("request failed", "method", op.Method, "path", op.Path, "error", exchange.Err)
		}

		for _, orc := range e.cfg.Oracles {
			findings = append(findings, orc.Check(exchange)...)
		}
	}

	return findings, nil
}

func (e *Executor) do(ctx context.Context, op contract.Operation) *oracle.Exchange {
	// One deadline per request, so a slow endpoint cannot stall the run.
	ctx, cancel := context.WithTimeout(ctx, e.cfg.RequestTimeout)
	defer cancel()

	u := (*url.URL).JoinPath(e.cfg.BaseURL, op.Path)

	// The contract says whether the operation takes a body. When the minimal
	// value cannot be built, nothing is sent: the exchange has no Request, so
	// the oracles ignore it, because the fault is specter's and not the API's.
	var body io.Reader
	schema, hasBody := op.Bodies[jsonMedia]
	if hasBody {
		raw, err := minimalJSON(schema)
		if err != nil {
			return oracle.NewExchange(&op, nil, nil, fmt.Errorf("build body: %w", err))
		}
		body = bytes.NewReader(raw)
	} else if op.BodyRequired {
		// The body is required but not in a media type we can build (v0 only
		// does JSON). Sending nothing would only get a 4xx, so skip it.
		return oracle.NewExchange(&op, nil, nil, fmt.Errorf("build body: %w", ErrUnsupportedMedia))
	}

	r, err := http.NewRequestWithContext(ctx, string(op.Method), u.String(), body)
	if err != nil {
		return oracle.NewExchange(&op, nil, nil, err)
	}
	if hasBody {
		r.Header.Set("Content-Type", jsonMedia)
	}

	slog.Debug("calling endpoint", "method", op.Method, "url", u.String())

	// Get all required headers
	for _, param := range op.Params {
		// Only sending required parameters
		if !param.Required {
			continue
		}
		if param.In == contract.ParamInHeader {
			r.Header.Set(param.Name, e.createRamdomValue(param.Schema))
		}
		if param.In == contract.ParamInQuery {
			slog.Debug("skipping query parameter", "name", param.Name)
		}
	}

	start := time.Now()
	resp, err := e.cfg.Client.Do(r)
	if err != nil {
		return oracle.NewExchange(&op, r, nil, err)
	}
	defer resp.Body.Close()

	exchange := oracle.NewExchange(&op, r, resp, nil)

	slog.Debug("response received",
		"method", op.Method, "url", u.String(),
		"status", resp.StatusCode, "duration", time.Since(start))

	return exchange
}

// jsonMedia is the only request media type the executor knows how to build.
const jsonMedia = "application/json"

// ErrUnsupportedMedia means an operation requires a request body in a media
// type other than JSON, which the executor cannot build yet.
var ErrUnsupportedMedia = errors.New("required request body is not application/json")

// minimalJSON returns the minimal value of schema encoded as JSON.
func minimalJSON(schema *contract.Schema) ([]byte, error) {
	value, err := generator.Minimal(schema)
	if err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

func (e *Executor) createRamdomValue(_ *contract.Schema) string {
	return "Dummyvalue"
}
