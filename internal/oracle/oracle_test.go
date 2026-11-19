package oracle

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func request(method string) *http.Request {
	return httptest.NewRequestWithContext(context.Background(), method, "/animals", nil)
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
	// Oracle with the value signature Check(Exchange).
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

			got := o.Check(Exchange{Request: request(tt.method), Response: resp})

			if len(got) != tt.want {
				t.Fatalf("Check() returned %d findings, want %d: %+v", len(got), tt.want, got)
			}
			for _, f := range got {
				if f.Oracle != o.Name() {
					t.Errorf("finding Oracle = %q, want %q", f.Oracle, o.Name())
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
		ex   Exchange
		want int
	}{
		{
			name: "GET transport error is a finding",
			ex:   Exchange{Request: request(http.MethodGet), Err: boom},
			want: 1,
		},
		{
			name: "request never built is not the API's fault",
			ex:   Exchange{Err: boom},
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
