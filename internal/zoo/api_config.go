package zoo

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

// maxBodyBytes caps request bodies so a client cannot exhaust memory.
const maxBodyBytes = 1 << 20

var errTrailingData = errors.New("request body must contain a single JSON object")

// decodeJSON reads exactly one JSON value from the request body into dst.
// Unknown fields, trailing data and bodies over maxBodyBytes are errors.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("decode body: %w", err)
	}

	// A second Decode must hit a clean EOF; anything else is trailing data.
	err := dec.Decode(&struct{}{})
	var tooLarge *http.MaxBytesError
	switch {
	case errors.Is(err, io.EOF):
		return nil
	case errors.As(err, &tooLarge):
		return fmt.Errorf("decode body: %w", err)
	default:
		return errTrailingData
	}
}

// rejectBody logs a failed decodeJSON and writes the matching error
// response: 413 for oversized bodies, 400 for everything else.
func (api *ZooApiConf) rejectBody(w http.ResponseWriter, r *http.Request, op string, err error, attrs ...any) {
	api.logger.WarnContext(r.Context(), op+" rejected",
		append([]any{"reason", "invalid body", "error", err}, attrs...)...)

	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &tooLarge):
		api.writeErrorResponse(w, http.StatusRequestEntityTooLarge, "Error: request body too large.")
	case errors.Is(err, errTrailingData):
		api.writeErrorResponse(w, http.StatusBadRequest, "Error: request body must contain only a single JSON object.")
	default:
		api.writeErrorResponse(w, http.StatusBadRequest, "Error to parse JSON")
	}
}

func (api *ZooApiConf) writeResponse(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Printf("failed to encode JSON response: %v", err)
		}
	}
}

func (api *ZooApiConf) writeErrorResponse(w http.ResponseWriter, code int, message string) {
	api.writeResponse(w, code, ErrorResponse{Error: message})
}
