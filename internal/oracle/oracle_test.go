package oracle

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/otonielsilva/api-specter/internal/contract"
)

func request(method string) *http.Request {
	return httptest.NewRequestWithContext(context.Background(), method, "/animals", nil)
}

// operation returns the contract operation that request(method) is a call to.
func operation(method string) *contract.Operation {
	return &contract.Operation{Method: contract.Method(method), Path: "/animals"}
}

func TestSimpleOracleCheck(t *testing.T) {
	tests := []struct {
		name   string
		method string
		code   int
		want   int // number of findings
	}{
		{name: "GET 200 is fine", method: http.MethodGet, code: http.StatusOK, want: 0},
		{name: "GET 500 is a finding", method: http.MethodGet, code: http.StatusInternalServerError, want: 1},
		{name: "POST 500 is a finding", method: http.MethodPost, code: http.StatusInternalServerError, want: 1},
	}

	// Going through the interface also checks that SimpleOracle implements
	// Oracle with the signature Check(*Exchange).
	var o Oracle = SimpleOracle{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Status is the full text ("200 OK") and StatusCode the number,
			// as the http client fills them. The body is http.NoBody, which
			// needs no Close.
			resp := &http.Response{ //nolint:bodyclose // http.NoBody needs no Close
				Status:     http.StatusText(tt.code),
				StatusCode: tt.code,
				Body:       http.NoBody,
			}

			op := operation(tt.method)

			got := o.Check(&Exchange{Operation: op, Request: request(tt.method), Response: resp})

			if len(got) != tt.want {
				t.Fatalf("Check() returned %d findings, want %d: %+v", len(got), tt.want, got)
			}
			for _, f := range got {
				if f.Oracle != o.Name() {
					t.Errorf("finding Oracle = %q, want %q", f.Oracle, o.Name())
				}
				if f.Operation != op.BuildID() {
					t.Errorf("finding Operation = %q, want %q", f.Operation, op.BuildID())
				}
			}
		})
	}
}

// A failed exchange has no response, and when the request could not even be
// built it has no request either. Check must not panic in either case.
func TestSimpleOracleCheckWithoutResponse(t *testing.T) {
	var o Oracle = SimpleOracle{}
	boom := errors.New("boom")

	tests := []struct {
		name string
		ex   *Exchange
		want int
	}{
		{
			name: "GET transport error is a finding",
			ex:   &Exchange{Operation: operation(http.MethodGet), Request: request(http.MethodGet), Err: boom},
			want: 1,
		},
		{
			name: "request never built is not the API's fault",
			ex:   &Exchange{Err: boom},
			want: 0,
		},
		{
			name: "nil exchange",
			ex:   nil,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := o.Check(tt.ex)

			if len(got) != tt.want {
				t.Fatalf("Check() returned %d findings, want %d: %+v", len(got), tt.want, got)
			}
		})
	}
}

// responseWith returns a 200 response whose body is read from body.
func responseWith(body io.Reader) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(body)} //nolint:bodyclose // NopCloser needs no Close
}

func TestNewExchangeReadsTheBody(t *testing.T) {
	boom := errors.New("boom")
	transport := errors.New("transport")

	tests := []struct {
		name          string
		body          io.Reader
		err           error // transport error given to NewExchange
		wantBody      string
		wantTruncated bool
		wantErr       error // nil means Exchange.Err must be nil
	}{
		{name: "small body", body: strings.NewReader(`{"id":1}`), wantBody: `{"id":1}`},
		{name: "empty body", body: http.NoBody, wantBody: ""},
		{
			name:     "body of exactly the limit is not truncated",
			body:     strings.NewReader(strings.Repeat("a", maxBodySize)),
			wantBody: strings.Repeat("a", maxBodySize),
		},
		{
			name:          "body over the limit is cut and marked",
			body:          strings.NewReader(strings.Repeat("a", maxBodySize+10)),
			wantBody:      strings.Repeat("a", maxBodySize),
			wantTruncated: true,
		},
		{
			name:     "read that fails keeps what was read and the error",
			body:     io.MultiReader(strings.NewReader("par"), iotest.ErrReader(boom)),
			wantBody: "par",
			wantErr:  boom,
		},
		{
			name:    "transport error has priority over a read error",
			body:    iotest.ErrReader(boom),
			err:     transport,
			wantErr: transport,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := responseWith(tt.body)
			defer resp.Body.Close()

			got := NewExchange(operation(http.MethodGet), request(http.MethodGet), resp, tt.err)

			if string(got.Body) != tt.wantBody {
				t.Errorf("Body has %d bytes, want %d", len(got.Body), len(tt.wantBody))
			}
			if got.Truncated != tt.wantTruncated {
				t.Errorf("Truncated = %v, want %v", got.Truncated, tt.wantTruncated)
			}
			if !errors.Is(got.Err, tt.wantErr) {
				t.Errorf("Err = %v, want %v", got.Err, tt.wantErr)
			}
		})
	}
}

// Without a response there is nothing to read, and the transport error is
// kept as it came.
func TestNewExchangeWithoutResponse(t *testing.T) {
	boom := errors.New("boom")
	op := operation(http.MethodGet)
	req := request(http.MethodGet)

	got := NewExchange(op, req, nil, boom)

	if got.Body != nil {
		t.Errorf("Body = %q, want nil", got.Body)
	}
	if got.Truncated {
		t.Error("Truncated = true, want false")
	}
	if !errors.Is(got.Err, boom) {
		t.Errorf("Err = %v, want %v", got.Err, boom)
	}
	if got.Operation != op || got.Request != req {
		t.Errorf("Operation and Request must be the ones given: got %v and %v", got.Operation, got.Request)
	}
}

// NewExchange only reads the response. It must not replace the body, so the
// caller still closes the one it got.
func TestNewExchangeKeepsTheGivenFields(t *testing.T) {
	op := operation(http.MethodPost)
	req := request(http.MethodPost)
	resp := responseWith(strings.NewReader("x"))
	defer resp.Body.Close()

	got := NewExchange(op, req, resp, nil)

	if got.Operation != op || got.Request != req || got.Response != resp {
		t.Errorf("NewExchange changed Operation, Request or Response: %+v", got)
	}
}
