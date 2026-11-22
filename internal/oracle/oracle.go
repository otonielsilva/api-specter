package oracle

import (
	"fmt"
	"io"
	"net/http"

	"github.com/otonielsilva/api-specter/internal/contract"
)

// Exchange is one request and the response it got. Response is nil and Err
// is set when the transport failed before any response arrived.
type Exchange struct {
	// Operation is the id of the operation, "METHOD /path".
	Operation *contract.Operation
	Request   *http.Request
	Response  *http.Response
	Body      []byte
	Err       error
	Truncated bool
}

// maxBodySize is the most bytes of a response body that an Exchange keeps.
const maxBodySize = 3 * 1024 * 1024

// NewExchange returns the Exchange of one call to operation: req is the
// request that was sent, resp the response and err the error of the
// transport, if any.
//
// It reads the body of resp, once, into Exchange.Body, so every oracle and the
// executor can use it without reading the stream again. It reads at most
// maxBodySize bytes: a longer body is cut and Truncated is set. When the read
// fails, Body keeps what was read and the error goes to Err, unless err was
// already set, because the transport error happened first.
//
// NewExchange does not close resp.Body, which stays with the caller. It must
// run before the request context is cancelled, or the read fails. When resp
// is nil there is nothing to read and the Exchange only carries err.
func NewExchange(operation *contract.Operation, req *http.Request, resp *http.Response, err error) *Exchange {
	ex := &Exchange{Operation: operation, Request: req, Response: resp, Err: err}
	if resp != nil {
		// Read one byte more than the limit: LimitReader stops at the limit
		// without an error, so only the extra byte says the body was cut.
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBodySize+1))
		if len(body) > maxBodySize {
			body = body[:maxBodySize]
			ex.Truncated = true
		}
		ex.Body = body

		// A transport error that came in has priority: it happened first.
		if readErr != nil && ex.Err == nil {
			ex.Err = fmt.Errorf("read response body: %w", readErr)
		}
	}
	return ex
}

// Finding is something an oracle judged to be wrong with an exchange.
type Finding struct {
	// Oracle is the name of the oracle that reported it.
	Oracle    string
	Operation string
	Message   string
}

// Oracle judges one exchange. It never sends requests.
type Oracle interface {
	// Name identifies the oracle in reports.
	Name() string
	// Check returns the findings for ex, or nil when the exchange is fine.
	Check(ex *Exchange) []Finding
}

// simpleName is the name SimpleOracle reports under.
const simpleName = "simple"

// SimpleOracle judges every exchange: it is fine when it gets a 2xx
// response, anything else is a finding. It has no state.
type SimpleOracle struct{}

// Name returns the name that goes in Finding.Oracle.
func (SimpleOracle) Name() string {
	return simpleName
}

// Check returns a finding when a request got no response or a non-2xx one.
// It returns nil for exchanges whose request was never built, which is a
// fault of specter and not of the API.
func (SimpleOracle) Check(ex *Exchange) []Finding {
	if ex == nil || ex.Operation == nil || ex.Request == nil {
		return nil
	}

	if ex.Response == nil {
		return []Finding{{
			Oracle:    simpleName,
			Operation: ex.Operation.BuildID(),
			Message:   fmt.Sprintf("no response: %v", ex.Err),
		}}
	}

	if code := ex.Response.StatusCode; code < 200 || code > 299 {
		return []Finding{{
			Oracle:    simpleName,
			Operation: ex.Operation.BuildID(),
			Message:   fmt.Sprintf("expected a 2xx response, got %d", code),
		}}
	}

	return nil
}
