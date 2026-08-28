package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
)

// maxRequestBodyBytes bounds every JSON request body decode() reads. Without a cap, an
// unauthenticated caller (e.g. POST /collect) could send an arbitrarily large body and
// force the whole thing into memory before any validation runs — a cheap way to strain a
// 1GB-RAM host (GH #13). 64KiB comfortably covers every payload this API decodes today
// (lineup starters, profile fields, bookmarks).
const maxRequestBodyBytes = 64 << 10 // 64KiB

func decode[T any](w http.ResponseWriter, r *http.Request) (T, error) {
	var v T
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		return v, err
	}
	return v, nil
}

// writeDecodeError maps a decode() failure to the appropriate HTTP status: 413 when the
// body exceeded maxRequestBodyBytes, 400 for any other malformed-JSON error.
func writeDecodeError(w http.ResponseWriter, err error) {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}
	http.Error(w, "invalid request body", http.StatusBadRequest)
}

func encode[T any](w http.ResponseWriter, r *http.Request, status int, v T) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	return nil
}

func parsePositiveInt(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	return n, err == nil && n >= 1
}

func parseWeek(s string) (int, bool) {
	return parsePositiveInt(s)
}
