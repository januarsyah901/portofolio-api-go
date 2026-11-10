// Package httpx holds HTTP helpers shared by every module: JSON I/O, error responses
// and middleware. Error bodies always have the shape {"message": "..."} to match the
// Express API, with messages in Indonesian.
package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
)

// Shared messages, identical to the Express version.
const (
	MsgServerError   = "Terjadi kesalahan pada server."
	MsgInternalError = "Terjadi kesalahan internal pada server."
	MsgNotFound      = "Endpoint tidak ditemukan."
	MsgBodyTooLarge  = "Body request terlalu besar."
	MsgInvalidJSON   = "Body request harus berupa JSON yang valid."
)

// MaxJSONBody is the largest JSON request body accepted (1 MB).
const MaxJSONBody = 1 << 20

// Message is the standard error/info body.
type Message struct {
	Message string `json:"message"`
}

// JSON writes v as JSON with the given status.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write json response", "err", err)
	}
}

// Error writes {"message": msg} with the given status.
func Error(w http.ResponseWriter, status int, msg string) {
	JSON(w, status, Message{Message: msg})
}

// ServerError logs err with a label and writes a 500 with the standard message.
func ServerError(w http.ResponseWriter, r *http.Request, label string, err error) {
	slog.ErrorContext(r.Context(), label, "err", err, "request_id", RequestID(r.Context()))
	Error(w, http.StatusInternalServerError, MsgServerError)
}

// DecodeObject reads a JSON body into a map of raw fields so handlers can check each
// field's JSON type, reproducing the lenient validation of the Express version.
// A missing or non-object body decodes to an empty map (Express treated it as {}).
// It returns false after writing a 400/413 response when the body is unreadable.
func DecodeObject(w http.ResponseWriter, r *http.Request) (Fields, bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxJSONBody))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			Error(w, http.StatusRequestEntityTooLarge, MsgBodyTooLarge)
			return nil, false
		}
		Error(w, http.StatusBadRequest, MsgInvalidJSON)
		return nil, false
	}
	fields := Fields{}
	if len(body) == 0 {
		return fields, true
	}
	var anyVal any
	if err := json.Unmarshal(body, &anyVal); err != nil {
		Error(w, http.StatusBadRequest, MsgInvalidJSON)
		return nil, false
	}
	if _, isObj := anyVal.(map[string]any); !isObj {
		return fields, true
	}
	if err := json.Unmarshal(body, &fields); err != nil {
		Error(w, http.StatusBadRequest, MsgInvalidJSON)
		return nil, false
	}
	return fields, true
}

// PathInt parses the named path wildcard as an int. ok is false when it is not an integer.
func PathInt(r *http.Request, name string) (int, bool) {
	n, err := strconv.Atoi(r.PathValue(name))
	return n, err == nil
}
