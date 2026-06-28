package handlers

import (
	"net/http"

	"github.com/sunkencosts/mirrorleague/internal/jwtauth"
)

// HandleDevLogin mints a JWT for a dev user without going through Google OAuth.
// Only registered when APP_ENV != "production". The default identity is the seed's primary
// account (config DevLogin*, mirroring SEED_PRIMARY_*), so dev-login "is" the same account
// seed-dev bookmarks and gives lineups to. Query params still override per-request.
func HandleDevLogin(jwtSecret []byte, frontendURL, defaultUserID, defaultEmail, defaultUsername string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			userID = defaultUserID
		}
		email := r.URL.Query().Get("email")
		if email == "" {
			email = defaultEmail
		}
		username := r.URL.Query().Get("username")
		if username == "" {
			username = defaultUsername
		}

		signed, err := jwtauth.Sign(jwtSecret, userID, email, username)
		if err != nil {
			http.Error(w, "failed to sign token", http.StatusInternalServerError)
			return
		}
		setAuthCookie(w, signed, false)
		http.Redirect(w, r, frontendURL, http.StatusFound)
	})
}
