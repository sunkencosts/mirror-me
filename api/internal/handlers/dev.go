package handlers

import (
	"net/http"

	"github.com/sunkencosts/mirrorleague/internal/jwtauth"
)

const devUserID = "00000000-0000-0000-0000-000000000001"

// HandleDevLogin mints a JWT for a dev user without going through Google OAuth.
// Only registered when APP_ENV != "production".
func HandleDevLogin(jwtSecret []byte, frontendURL string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			userID = devUserID
		}
		email := r.URL.Query().Get("email")
		if email == "" {
			email = "dev@localhost"
		}
		username := r.URL.Query().Get("username")
		if username == "" {
			username = "dev_user"
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
