package oracle

import (
	"fmt"
	"strconv"

	"github.com/otonielsilva/api-specter/internal/contract"
)

// contractName is the name ContractOracle reports under.
const contractName = "contract"

// ContractOracle judges an exchange against the responses that the contract
// documents for the operation. It has no state.
//
// Each exchange gets one Category (see classify). Only the ones that are not
// CategoryExpected become findings. SimpleOracle stays as the baseline that
// ignores the contract.
type ContractOracle struct{}

// Name returns the name that goes in Finding.Oracle.
func (ContractOracle) Name() string {
	return contractName
}

// Check returns no finding when the exchange is CategoryExpected or cannot be
// judged, and one finding, with its Category, otherwise. The message names the
// status, or the transport error, but never the body: it comes from the target.
func (ContractOracle) Check(ex *Exchange) []Finding {
	cat, ok := classify(ex)
	if !ok || cat == CategoryExpected {
		return nil
	}

	f := Finding{
		Oracle:    contractName,
		Operation: ex.Operation.BuildID(),
		Category:  cat,
	}
	switch cat {
	case CategoryNoResponse:
		f.Message = fmt.Sprintf("no response: %v", ex.Err)
	case CategoryServerError:
		f.Message = fmt.Sprintf("server error: status %d", ex.Response.StatusCode)
	default:
		// CategoryUndocumented: the other categories are handled above.
		f.Message = fmt.Sprintf("status %d is not documented for this operation", ex.Response.StatusCode)
	}

	return []Finding{f}
}

// classify says what category ex is. ok is false when there is nothing to
// judge: a nil exchange, or one without Operation or without Request, which
// means specter did not build the request and the fault is not the API's.
//
// The order of the rules matters:
//  1. no Response (and a Request that was sent): CategoryNoResponse;
//  2. a 5xx: CategoryServerError, even if the contract documents it;
//  3. a status that documented reports: CategoryExpected;
//  4. anything else: CategoryUndocumented.
//
// A body that failed to read (ex.Err with a Response) does not change the
// category: the status was received and is what is judged here.
func classify(ex *Exchange) (cat Category, ok bool) {
	if ex == nil || ex.Operation == nil || ex.Request == nil {
		return "", false
	}
	if ex.Response == nil {
		return CategoryNoResponse, true
	}
	if ex.Response.StatusCode >= 500 && ex.Response.StatusCode <= 599 {
		return CategoryServerError, true
	}
	if !documented(ex.Operation.Responses, ex.Response.StatusCode) {
		return CategoryUndocumented, true
	}
	return CategoryExpected, true
}

// documented reports whether responses, the Responses of an operation, lists
// status. The keys are as OpenAPI writes them and are searched in this order:
// the exact status ("404"), the class of the status ("4XX", with a capital X as
// the specification says) and "default", which matches any status. A key whose
// value is nil still counts. A nil or empty map documents nothing.
//
// It only answers whether the status is listed. Whether a 5xx is an error even
// when listed is a rule of classify.
func documented(responses map[string]*contract.Response, status int) bool {
	// Reading a nil map is safe in Go and finds nothing, so there is no nil
	// check.
	if _, found := responses[strconv.Itoa(status)]; found {
		return true
	}

	// 404 / 100 is 4: integer division gives the class of the status.
	class := status / 100
	if _, found := responses[strconv.Itoa(class)+"XX"]; found {
		return true
	}

	_, found := responses["default"]
	return found
}
