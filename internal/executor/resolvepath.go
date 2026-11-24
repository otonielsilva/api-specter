package executor

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ErrMissingParam means the template has a {name} that values does not have.
var ErrMissingParam = errors.New("path parameter has no value")

// ErrBadTemplate means the path template is malformed: empty, a brace that is
// not closed or opened, or an empty name.
var ErrBadTemplate = errors.New("malformed path template")

// resolvePath fills each {name} of an OpenAPI path template with values[name]
// and returns the path, for example "/animals/{id}/feedings" with id "1"
// becomes "/animals/1/feedings". The keys of values are the parameter names
// without braces, and keys the template does not use are ignored.
//
// Each value is one path segment: it is escaped with url.PathEscape, so it
// cannot add a separator, a query or a fragment. A value is only text and is
// never read as a placeholder: the template is read once, from left to right,
// and what is written is not read again.
//
// resolvePath returns an empty string and an error that wraps one of:
//   - ErrBadTemplate: the template is empty or has an unbalanced brace or an
//     empty name;
//   - ErrMissingParam: values has no entry for a name in the template;
//   - ErrUnconvertible: the value is empty, "." or ".." (the last two make
//     url.JoinPath leave the resource, so the call would reach another
//     endpoint). The rule looks only at the value, not at the text around it.
//
// The errors name the parameter or the template but never the value, which
// comes from a response of the target and may be large.
func resolvePath(template string, values map[string]string) (string, error) {
	if template == "" {
		return "", fmt.Errorf("%w: empty template", ErrBadTemplate)
	}

	var b strings.Builder
	rest := template

	for {
		before, after, foundBefore := strings.Cut(rest, "{")
		// A "}" in the text between placeholders has no "{" to close. The last
		// piece of the template comes through here too, so this covers it.
		if strings.Contains(before, "}") {
			return "", fmt.Errorf("%w: unexpected } in %q", ErrBadTemplate, template)
		}
		b.WriteString(before)
		if !foundBefore {
			break
		}

		pVar, tail, foundAfter := strings.Cut(after, "}")
		if !foundAfter {
			return "", fmt.Errorf("%w: { is not closed in %q", ErrBadTemplate, template)
		}
		// An empty name, or a "{" inside it as in "{{id}}".
		if pVar == "" || strings.Contains(pVar, "{") {
			return "", fmt.Errorf("%w: invalid name %q in %q", ErrBadTemplate, pVar, template)
		}

		resolvedValue, ok := values[pVar]
		if !ok {
			return "", fmt.Errorf("%w: %q", ErrMissingParam, pVar)
		}
		if resolvedValue == "" || resolvedValue == "." || resolvedValue == ".." {
			return "", fmt.Errorf("%w: value of %q is empty or a dot segment", ErrUnconvertible, pVar)
		}

		b.WriteString(url.PathEscape(resolvedValue))
		rest = tail
	}

	return b.String(), nil
}
