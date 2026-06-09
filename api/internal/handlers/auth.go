package handlers

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	mrand "math/rand/v2"
	"net/http"

	"github.com/sunkencosts/mirrorleague/internal/jwtauth"
	"github.com/sunkencosts/mirrorleague/internal/provider"
)

const oauthStateCookie = "oauth_state"
const oauthProviderGoogle = "google"
const maxUsernameAttempts = 5
const authCookieMaxAge = 30 * 24 * 60 * 60

type googleClient interface {
	AuthCodeURL(state string) string
	IsSecure() bool
	FetchUser(ctx context.Context, code string) (id, email string, err error)
}

type authStore interface {
	CreateOrGetOAuthUser(ctx context.Context, oauthProvider, providerID, email, username string) (provider.AuthUser, error)
	MergeAnonymousData(ctx context.Context, anonymousID, userID string) error
}

var adjectives = []string{
	"amber", "bold", "calm", "dark", "fast", "keen", "mild", "neat",
	"pure", "rare", "safe", "tall", "vast", "warm", "wise", "zeal",
}

var nouns = []string{
	"bear", "bird", "buck", "bull", "crow", "dove", "duck", "elk",
	"fish", "fawn", "fox", "goat", "hawk", "hare", "jay", "kite",
	"lark", "lion", "lynx", "mole", "moth", "mule", "newt", "owl",
	"puma", "rook", "seal", "swan", "wolf", "wren", "yak", "zebu",
}

func generateUsername() string {
	return adjectives[mrand.IntN(len(adjectives))] + "_" + nouns[mrand.IntN(len(nouns))] + fmt.Sprintf("%02d", mrand.IntN(100)) //nolint:gosec
}

func sameSiteMode(secure bool) http.SameSite {
	if secure {
		return http.SameSiteNoneMode
	}
	return http.SameSiteLaxMode
}

func setAuthCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec
		Name:     "auth_token",
		Value:    token,
		Path:     "/",
		MaxAge:   authCookieMaxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSiteMode(secure),
	})
}

func HandleGoogleLogin(client googleClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stateBytes := make([]byte, 16)
		if _, err := rand.Read(stateBytes); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		state := fmt.Sprintf("%x", stateBytes)
		http.SetCookie(w, &http.Cookie{ //nolint:gosec
			Name:     oauthStateCookie,
			Value:    state,
			MaxAge:   300,
			HttpOnly: true,
			Secure:   client.IsSecure(),
			SameSite: sameSiteMode(client.IsSecure()),
		})
		http.Redirect(w, r, client.AuthCodeURL(state), http.StatusFound)
	})
}

func HandleGoogleCallback(client googleClient, store authStore, jwtSecret []byte, frontendURL string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		cookie, err := r.Cookie(oauthStateCookie)
		if err != nil || cookie.Value != q.Get("state") {
			http.Error(w, "invalid state", http.StatusBadRequest)
			return
		}

		providerID, email, err := client.FetchUser(r.Context(), q.Get("code"))
		if err != nil {
			http.Error(w, "failed to get user info", http.StatusInternalServerError)
			return
		}

		var user provider.AuthUser
		for range maxUsernameAttempts {
			user, err = store.CreateOrGetOAuthUser(r.Context(), oauthProviderGoogle, providerID, email, generateUsername())
			if err == nil || !errors.Is(err, provider.ErrUsernameConflict) {
				break
			}
		}
		if err != nil {
			http.Error(w, "failed to create user", http.StatusInternalServerError)
			return
		}

		signed, err := jwtauth.Sign(jwtSecret, user.ID, user.Email, user.Username)
		if err != nil {
			http.Error(w, "failed to sign token", http.StatusInternalServerError)
			return
		}

		setAuthCookie(w, signed, client.IsSecure())
		http.Redirect(w, r, frontendURL, http.StatusFound)
	})
}

func HandleAuthMe() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_ = encode(w, r, http.StatusOK, provider.AuthUser{
			ID:       claims.Subject,
			Email:    claims.Email,
			Username: claims.Username,
		})
	})
}

type mergeRequest struct {
	AnonymousID string `json:"anonymous_id"`
}

func HandleMerge(store authStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		req, err := decode[mergeRequest](r)
		if err != nil || req.AnonymousID == "" {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if err := store.MergeAnonymousData(r.Context(), req.AnonymousID, claims.Subject); err != nil {
			http.Error(w, "failed to merge", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func HandleLogout(secure bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{ //nolint:gosec
			Name:     "auth_token",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   secure,
			SameSite: sameSiteMode(secure),
		})
		w.WriteHeader(http.StatusNoContent)
	})
}
