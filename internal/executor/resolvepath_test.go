package executor

import (
	"errors"
	"testing"
)

// resolvePath fills the {name} placeholders of an OpenAPI path template with
// values that paramString already converted. It is the last step before the
// text goes into a URL, so it escapes each value and refuses the ones that
// would leave the segment they were meant for.
func TestResolvePath(t *testing.T) {
	tests := []struct {
		name     string
		template string
		values   map[string]string
		want     string
		wantErr  error // nil means success; otherwise errors.Is must match
	}{
		// Plain substitution.
		{name: "no parameters", template: "/animals", values: map[string]string{}, want: "/animals"},
		{name: "one parameter at the end", template: "/animals/{id}", values: map[string]string{"id": "1"}, want: "/animals/1"},
		{name: "one parameter in the middle", template: "/animals/{id}/feedings", values: map[string]string{"id": "1"}, want: "/animals/1/feedings"},
		{name: "two parameters", template: "/zoos/{zoo}/animals/{id}", values: map[string]string{"zoo": "7", "id": "1"}, want: "/zoos/7/animals/1"},
		{name: "same parameter twice", template: "/a/{id}/b/{id}", values: map[string]string{"id": "1"}, want: "/a/1/b/1"},
		{name: "parameter inside a segment", template: "/files/{name}.json", values: map[string]string{"name": "report"}, want: "/files/report.json"},
		{name: "two parameters in one segment", template: "/range/{from}-{to}", values: map[string]string{"from": "1", "to": "9"}, want: "/range/1-9"},
		{name: "extra values are ignored", template: "/animals/{id}", values: map[string]string{"id": "1", "other": "2"}, want: "/animals/1"},

		// Each value is one path segment: it is escaped, so it cannot add
		// separators, query or fragment.
		{name: "space", template: "/animals/{id}", values: map[string]string{"id": "a b"}, want: "/animals/a%20b"},
		{name: "slash stays inside the segment", template: "/animals/{id}", values: map[string]string{"id": "a/b"}, want: "/animals/a%2Fb"},
		{name: "question mark and hash", template: "/animals/{id}", values: map[string]string{"id": "a?b#c"}, want: "/animals/a%3Fb%23c"},
		{name: "non ASCII", template: "/animals/{id}", values: map[string]string{"id": "ü"}, want: "/animals/%C3%BC"},
		{name: "percent sign", template: "/animals/{id}", values: map[string]string{"id": "a%b"}, want: "/animals/a%25b"},
		{name: "already escaped dots are text, not dots", template: "/animals/{id}", values: map[string]string{"id": "%2e%2e"}, want: "/animals/%252e%252e"},
		{name: "unreserved characters are left alone", template: "/animals/{id}", values: map[string]string{"id": "a-b_c.d~e"}, want: "/animals/a-b_c.d~e"},

		// A value is text. It is never read as a placeholder, whatever order
		// the placeholders come in.
		{name: "value that looks like a placeholder", template: "/a/{x}/{y}", values: map[string]string{"x": "{y}", "y": "2"}, want: "/a/%7By%7D/2"},

		// Segments that leave the resource. url.JoinPath, which builds the URL
		// later, turns /animals/../feeds into /feeds and /animals/./feeds into
		// /animals/feeds, so the call would reach another endpoint.
		{name: "dot", template: "/animals/{id}/feedings", values: map[string]string{"id": "."}, wantErr: ErrUnconvertible},
		{name: "double dot", template: "/animals/{id}/feedings", values: map[string]string{"id": ".."}, wantErr: ErrUnconvertible},
		{name: "empty value", template: "/animals/{id}/feedings", values: map[string]string{"id": ""}, wantErr: ErrUnconvertible},
		{name: "second parameter is a double dot", template: "/zoos/{zoo}/animals/{id}", values: map[string]string{"zoo": "7", "id": ".."}, wantErr: ErrUnconvertible},
		// Only a value that is exactly "." or ".." is refused: these are
		// ordinary names.
		{name: "three dots are a name", template: "/animals/{id}", values: map[string]string{"id": "..."}, want: "/animals/..."},
		{name: "leading dot is a name", template: "/animals/{id}", values: map[string]string{"id": ".hidden"}, want: "/animals/.hidden"},
		// The rule looks at the value, not at what surrounds it: simpler, and
		// it never lets a dot segment through.
		{name: "double dot is refused even next to text", template: "/files/{name}.json", values: map[string]string{"name": ".."}, wantErr: ErrUnconvertible},

		// A placeholder without a value.
		{name: "missing value", template: "/animals/{id}", values: map[string]string{}, wantErr: ErrMissingParam},
		{name: "nil map", template: "/animals/{id}", values: nil, wantErr: ErrMissingParam},
		{name: "second parameter missing", template: "/zoos/{zoo}/animals/{id}", values: map[string]string{"zoo": "7"}, wantErr: ErrMissingParam},
		{name: "name is case sensitive", template: "/animals/{id}", values: map[string]string{"ID": "1"}, wantErr: ErrMissingParam},

		// A template that is not valid.
		{name: "brace not closed", template: "/animals/{id", values: map[string]string{"id": "1"}, wantErr: ErrBadTemplate},
		{name: "closing brace alone", template: "/animals/id}", values: map[string]string{"id": "1"}, wantErr: ErrBadTemplate},
		{name: "empty name", template: "/animals/{}", values: map[string]string{"": "1"}, wantErr: ErrBadTemplate},
		{name: "opening brace inside a name", template: "/animals/{{id}}", values: map[string]string{"id": "1"}, wantErr: ErrBadTemplate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolvePath(tt.template, tt.values)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("resolvePath(%q, %v) error = %v, want %v", tt.template, tt.values, err, tt.wantErr)
				}
				if got != "" {
					t.Errorf("resolvePath(%q, %v) = %q on error, want empty", tt.template, tt.values, got)
				}
				return
			}

			if err != nil {
				t.Fatalf("resolvePath(%q, %v) unexpected error: %v", tt.template, tt.values, err)
			}
			if got != tt.want {
				t.Errorf("resolvePath(%q, %v) = %q, want %q", tt.template, tt.values, got, tt.want)
			}
		})
	}
}
