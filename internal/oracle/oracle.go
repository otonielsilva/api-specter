package oracle

import "net/http"

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
