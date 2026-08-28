package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/sunkencosts/mirrorleague/internal/provider"
)

type visitStore interface {
	InsertVisit(ctx context.Context, v provider.Visit) error
}

type collectRequest struct {
	Path      string `json:"path"`
	Referrer  string `json:"referrer"`
	VisitorID string `json:"visitor_id"`
}

// HandleCollect records a first-party page-view event fired by the SPA on each route
// change. It is intentionally public (anonymous visitors are the whole point) but stamps
// user_id from the OptionalAuth claims when the request carries a valid auth JWT, so the
// same visitor_id row history reveals the anon → sign-up conversion. No cookie is set:
// visitor_id rides in the request body, reusing the id the frontend already keeps in
// localStorage.
func HandleCollect(store visitStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, err := decode[collectRequest](w, r)
		if err != nil {
			writeDecodeError(w, err)
			return
		}
		if req.VisitorID == "" || req.Path == "" {
			http.Error(w, "missing visitor_id or path", http.StatusBadRequest)
			return
		}

		visit := provider.Visit{
			VisitorID: req.VisitorID,
			Path:      req.Path,
			Referrer:  req.Referrer,
			Country:   r.Header.Get("CF-IPCountry"),
			IsBot:     isBot(r.UserAgent()),
		}
		if claims, ok := ClaimsFromContext(r.Context()); ok {
			visit.UserID = claims.Subject
		}

		if err := store.InsertVisit(r.Context(), visit); err != nil {
			http.Error(w, "failed to record visit", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// botUASubstrings are lowercased markers of non-human clients. A JS-fired beacon already
// filters most crawlers (they don't run the SPA), so this is a cheap best-effort backstop,
// not ground truth — is_bot lets read-time queries exclude or measure them.
var botUASubstrings = []string{
	"bot", "crawl", "spider", "slurp", "bingpreview", "facebookexternalhit",
	"embedly", "headlesschrome", "python-requests", "curl/", "wget",
}

func isBot(userAgent string) bool {
	userAgent = strings.ToLower(userAgent)
	if userAgent == "" {
		return true // a missing UA is almost always automation, not a real browser
	}
	for _, marker := range botUASubstrings {
		if strings.Contains(userAgent, marker) {
			return true
		}
	}
	return false
}
