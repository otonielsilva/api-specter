package oracle

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/otonielsilva/api-specter/internal/contract"
)

// responses builds the Responses of an operation from the keys that the
// OpenAPI document would have, each with an empty documented response.
func responses(keys ...string) map[string]*contract.Response {
	m := make(map[string]*contract.Response, len(keys))
	for _, k := range keys {
		m[k] = &contract.Response{}
	}
	return m
}

// exchangeWith returns an exchange of GET /animals that got status, for an
// operation whose contract documents the given response keys.
func exchangeWith(status int, keys ...string) *Exchange {
	op := &contract.Operation{Method: contract.GET, Path: "/animals", Responses: responses(keys...)}
	return &Exchange{
		Operation: op,
		Request:   request(http.MethodGet),
		Response:  &http.Response{StatusCode: status, Body: http.NoBody}, //nolint:bodyclose // http.NoBody needs no Close
	}
}

// documented only says whether a status is listed. The keys are searched as
// the specification writes them: the exact status, then the class ("4XX"),
// then "default".
func TestDocumented(t *testing.T) {
	tests := []struct {
		name      string
		responses map[string]*contract.Response
		status    int
		want      bool
	}{
		// Exact status.
		{name: "exact status", responses: responses("200"), status: 200, want: true},
		{name: "exact status among others", responses: responses("200", "404"), status: 404, want: true},
		{name: "status that is not listed", responses: responses("200"), status: 404, want: false},
		{name: "a close status is not the same status", responses: responses("200"), status: 201, want: false},

		// Class of the status.
		{name: "2XX first status", responses: responses("2XX"), status: 200, want: true},
		{name: "2XX last status", responses: responses("2XX"), status: 299, want: true},
		{name: "2XX does not cover 3xx", responses: responses("2XX"), status: 300, want: false},
		{name: "2XX does not cover 199", responses: responses("2XX"), status: 199, want: false},
		{name: "4XX covers a 404", responses: responses("4XX"), status: 404, want: true},
		{name: "4XX does not cover a 2xx", responses: responses("4XX"), status: 200, want: false},
		{name: "5XX covers a 503", responses: responses("5XX"), status: 503, want: true},
		// OpenAPI writes the X in capital. A lowercase key is not one of its
		// keys, and is not guessed at.
		{name: "lowercase class is not a key", responses: responses("4xx"), status: 404, want: false},

		// default matches any status that is not listed.
		{name: "default covers any status", responses: responses("default"), status: 418, want: true},
		{name: "default covers a 404", responses: responses("200", "default"), status: 404, want: true},
		{name: "default covers a 5xx", responses: responses("default"), status: 503, want: true},

		// A key counts even when its value is nil.
		{name: "key with a nil response", responses: map[string]*contract.Response{"404": nil}, status: 404, want: true},

		// Nothing documented.
		{name: "nil map", responses: nil, status: 200, want: false},
		{name: "empty map", responses: map[string]*contract.Response{}, status: 200, want: false},
		{name: "keys that are not statuses", responses: responses("success", "20"), status: 200, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := documented(tt.responses, tt.status); got != tt.want {
				t.Errorf("documented(%v, %d) = %v, want %v", keysOf(tt.responses), tt.status, got, tt.want)
			}
		})
	}
}

func keysOf(m map[string]*contract.Response) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// classify gives each exchange one category. The rules go in order: no
// response, then 5xx (even when documented), then documented, then the rest.
func TestClassify(t *testing.T) {
	// A request that was sent and got no response: the transport failed.
	transportFailed := &Exchange{
		Operation: &contract.Operation{Method: contract.GET, Path: "/animals", Responses: responses("200")},
		Request:   request(http.MethodGet),
		Err:       errors.New("connection refused"),
	}

	// The status was received but reading the body failed: the status is what
	// is judged.
	bodyFailed := exchangeWith(200, "200")
	bodyFailed.Err = errors.New("unexpected EOF")

	tests := []struct {
		name   string
		ex     *Exchange
		want   Category
		wantOK bool // false means there is nothing to judge
	}{
		// CategoryExpected: the contract lists the status, whatever it is.
		{name: "200 listed", ex: exchangeWith(200, "200"), want: CategoryExpected, wantOK: true},
		{name: "201 covered by 2XX", ex: exchangeWith(201, "2XX"), want: CategoryExpected, wantOK: true},
		{name: "204 listed", ex: exchangeWith(204, "204"), want: CategoryExpected, wantOK: true},
		{name: "404 listed is expected, not an error", ex: exchangeWith(404, "200", "404"), want: CategoryExpected, wantOK: true},
		{name: "400 covered by 4XX", ex: exchangeWith(400, "200", "4XX"), want: CategoryExpected, wantOK: true},
		{name: "302 listed", ex: exchangeWith(302, "302"), want: CategoryExpected, wantOK: true},
		{name: "any status covered by default", ex: exchangeWith(418, "200", "default"), want: CategoryExpected, wantOK: true},
		{name: "404 with only default", ex: exchangeWith(404, "default"), want: CategoryExpected, wantOK: true},
		{name: "body that failed to read does not change the status", ex: bodyFailed, want: CategoryExpected, wantOK: true},

		// CategoryUndocumented: a status the contract does not list.
		{name: "418 not listed", ex: exchangeWith(418, "200", "404"), want: CategoryUndocumented, wantOK: true},
		{name: "200 when only 201 is listed", ex: exchangeWith(200, "201"), want: CategoryUndocumented, wantOK: true},
		{name: "400 when only 404 is listed", ex: exchangeWith(400, "200", "404"), want: CategoryUndocumented, wantOK: true},
		{name: "302 not listed", ex: exchangeWith(302, "200"), want: CategoryUndocumented, wantOK: true},
		{name: "operation with no responses at all", ex: exchangeWith(200), want: CategoryUndocumented, wantOK: true},

		// CategoryServerError: any 5xx, and it wins over a documented one.
		{name: "500 not listed", ex: exchangeWith(500, "200"), want: CategoryServerError, wantOK: true},
		{name: "500 listed", ex: exchangeWith(500, "200", "500"), want: CategoryServerError, wantOK: true},
		{name: "503 covered by 5XX", ex: exchangeWith(503, "5XX"), want: CategoryServerError, wantOK: true},
		{name: "503 covered by default", ex: exchangeWith(503, "default"), want: CategoryServerError, wantOK: true},
		{name: "599 is a 5xx", ex: exchangeWith(599, "200"), want: CategoryServerError, wantOK: true},
		{name: "499 is not a 5xx", ex: exchangeWith(499, "200"), want: CategoryUndocumented, wantOK: true},

		// CategoryNoResponse.
		{name: "transport failed", ex: transportFailed, want: CategoryNoResponse, wantOK: true},

		// Nothing to judge: specter did not send the request.
		{name: "nil exchange", ex: nil, wantOK: false},
		{name: "no operation", ex: &Exchange{Request: request(http.MethodGet), Response: &http.Response{StatusCode: 200}}, wantOK: false}, //nolint:bodyclose // no body is read
		{name: "no request, as when the body cannot be built", ex: &Exchange{Operation: operation(http.MethodPost), Err: errors.New("build body")}, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := classify(tt.ex)

			if ok != tt.wantOK {
				t.Fatalf("classify() ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("classify() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Check turns every category except CategoryExpected into one finding.
func TestContractOracleCheck(t *testing.T) {
	transportFailed := &Exchange{
		Operation: &contract.Operation{Method: contract.GET, Path: "/animals", Responses: responses("200")},
		Request:   request(http.MethodGet),
		Err:       errors.New("connection refused"),
	}

	tests := []struct {
		name         string
		ex           *Exchange
		wantCategory Category // empty means no finding
		wantInMsg    string   // text the message must have, to help whoever reads the report
	}{
		{name: "expected is not a finding", ex: exchangeWith(200, "200")},
		{name: "a documented 404 is not a finding", ex: exchangeWith(404, "200", "404")},
		{name: "undocumented status", ex: exchangeWith(418, "200"), wantCategory: CategoryUndocumented, wantInMsg: "418"},
		{name: "server error", ex: exchangeWith(500, "200"), wantCategory: CategoryServerError, wantInMsg: "500"},
		{name: "a documented 5xx is still a finding", ex: exchangeWith(503, "200", "503"), wantCategory: CategoryServerError, wantInMsg: "503"},
		{name: "transport failed", ex: transportFailed, wantCategory: CategoryNoResponse, wantInMsg: "connection refused"},
		{name: "nil exchange", ex: nil},
		{name: "request that was not built", ex: &Exchange{Operation: operation(http.MethodPost), Err: errors.New("build body")}},
	}

	// Going through the interface also checks that ContractOracle implements
	// Oracle.
	var o Oracle = ContractOracle{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := o.Check(tt.ex)

			if tt.wantCategory == "" {
				if len(got) != 0 {
					t.Fatalf("Check() = %+v, want no finding", got)
				}
				return
			}

			if len(got) != 1 {
				t.Fatalf("Check() returned %d findings, want 1: %+v", len(got), got)
			}
			f := got[0]
			if f.Category != tt.wantCategory {
				t.Errorf("Category = %q, want %q", f.Category, tt.wantCategory)
			}
			if f.Oracle != o.Name() {
				t.Errorf("Oracle = %q, want %q", f.Oracle, o.Name())
			}
			if want := tt.ex.Operation.BuildID(); f.Operation != want {
				t.Errorf("Operation = %q, want %q", f.Operation, want)
			}
			if !strings.Contains(f.Message, tt.wantInMsg) {
				t.Errorf("Message = %q, want it to contain %q", f.Message, tt.wantInMsg)
			}
		})
	}
}

// The response body comes from the target and can be large or hold secrets, so
// the message of a finding carries the status and never the body.
func TestContractOracleMessageHasNoBody(t *testing.T) {
	ex := exchangeWith(500, "200")
	ex.Body = []byte(`{"token": "s3cret"}`)

	got := ContractOracle{}.Check(ex)
	if len(got) != 1 {
		t.Fatalf("Check() returned %d findings, want 1: %+v", len(got), got)
	}
	if strings.Contains(got[0].Message, "s3cret") {
		t.Errorf("Message = %q, must not contain the body", got[0].Message)
	}
}

// Both oracles can run on the same exchange: SimpleOracle does not categorise
// and keeps judging by the status alone, so the two can be compared.
func TestSimpleOracleHasNoCategory(t *testing.T) {
	got := SimpleOracle{}.Check(exchangeWith(404, "404"))
	if len(got) != 1 {
		t.Fatalf("Check() returned %d findings, want 1: %+v", len(got), got)
	}
	if got[0].Category != "" {
		t.Errorf("Category = %q, want empty", got[0].Category)
	}
}
