package oracle

import (
	"fmt"
	"net/http"
)

// Exchange is one request and the response it got. Response is nil and Err
// is set when the transport failed before any response arrived.
type Exchange struct {
	// Operation is the id of the operation, "METHOD /path".
	Operation string
	Request   *http.Request
	Response  *http.Response
	Err       error
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
	Check(ex Exchange) []Finding
}

// simpleName is the name SimpleOracle reports under.
const simpleName = "simple"

// SimpleOracle judges only GET exchanges: a GET is fine when it gets a 2xx
// response, anything else is a finding. It has no state.
type SimpleOracle struct{}

// Compile-time check that SimpleOracle implements Oracle.
var _ Oracle = SimpleOracle{}

// Name returns the name that goes in Finding.Oracle.
func (SimpleOracle) Name() string {
	return simpleName
}

// Check returns a finding when a GET got no response or a non-2xx one. It
// returns nil for other methods and for exchanges whose request was never
// built, which is a fault of specter and not of the API.
func (SimpleOracle) Check(ex Exchange) []Finding {
	if ex.Request == nil || ex.Request.Method != http.MethodGet {
		return nil
	}

	operation := ex.Operation
	if operation == "" {
		operation = ex.Request.Method + " " + ex.Request.URL.Path
	}

	if ex.Response == nil {
		return []Finding{{
			Oracle:    simpleName,
			Operation: operation,
			Message:   fmt.Sprintf("no response: %v", ex.Err),
		}}
	}

	if code := ex.Response.StatusCode; code < 200 || code > 299 {
		return []Finding{{
			Oracle:    simpleName,
			Operation: operation,
			Message:   fmt.Sprintf("expected a 2xx response, got %d", code),
		}}
	}

	return nil
}
