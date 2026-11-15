package executor

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/otonielsilva/api-specter/internal/contract"
	"github.com/otonielsilva/api-specter/internal/oracle"
	"github.com/otonielsilva/api-specter/internal/plan"
)

// ErrNotImplemented is returned while the executor is only a skeleton.
var ErrNotImplemented = errors.New("executor: not implemented")

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
	// Oracles judge every exchange.
	Oracles []oracle.Oracle
}

// Executor runs a plan against a target API.
type Executor struct {
	cfg Config
}

// New returns an Executor for cfg.
func New(cfg Config) *Executor {
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
func (e *Executor) Run(_ context.Context, _ *plan.Plan, _ *contract.Contract) ([]oracle.Finding, error) {
	return nil, ErrNotImplemented
}
