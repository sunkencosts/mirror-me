package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sunkencosts/mirrorleague/internal/db"
	"github.com/sunkencosts/mirrorleague/internal/jwtauth"
	"github.com/sunkencosts/mirrorleague/internal/provider"
)

const testDatabaseURL = "postgres://mirrorleague:mirrorleague@localhost:5433/mirrorleague_test"

// testJWTSecret is used by signTestJWT and must match the JWT_SECRET env var in newTestServer.
const testJWTSecret = "test-jwt-secret-32bytes-long-pad!"
const testAdminSecret = "test-admin-secret"

const testUserID = "00000000-0000-0000-0000-000000000001"

// testPlayers is the fixed reference dataset seeded once before all tests run.
// Use these player IDs in any Sleeper mock that needs player resolution.
// IDs 111/222/333/444/555 are the original five (kept so existing tests still pass);
// the rest add positional depth — multiple QB/RB/WR/TE plus K and DEF — so the fixture
// world (fixtures_test.go) can build legal 9-slot lineups with meaningful benches.
var testPlayers = []provider.Player{
	// QB
	{PlayerID: "111", FirstName: "Josh", LastName: "Allen", FantasyPositions: []string{"QB"}, Active: true},
	{PlayerID: "112", FirstName: "Lamar", LastName: "Jackson", FantasyPositions: []string{"QB"}, Active: true},
	{PlayerID: "113", FirstName: "Jalen", LastName: "Hurts", FantasyPositions: []string{"QB"}, Active: true},
	// RB
	{PlayerID: "333", FirstName: "Christian", LastName: "McCaffrey", FantasyPositions: []string{"RB"}, Active: true},
	{PlayerID: "334", FirstName: "Bijan", LastName: "Robinson", FantasyPositions: []string{"RB"}, Active: true},
	{PlayerID: "335", FirstName: "Saquon", LastName: "Barkley", FantasyPositions: []string{"RB"}, Active: true},
	{PlayerID: "336", FirstName: "Jahmyr", LastName: "Gibbs", FantasyPositions: []string{"RB"}, Active: true},
	// WR
	{PlayerID: "222", FirstName: "Justin", LastName: "Jefferson", FantasyPositions: []string{"WR"}, Active: true},
	{PlayerID: "555", FirstName: "Tyreek", LastName: "Hill", FantasyPositions: []string{"WR"}, Active: true},
	{PlayerID: "223", FirstName: "CeeDee", LastName: "Lamb", FantasyPositions: []string{"WR"}, Active: true},
	{PlayerID: "224", FirstName: "Amon-Ra", LastName: "St. Brown", FantasyPositions: []string{"WR"}, Active: true},
	{PlayerID: "225", FirstName: "A.J.", LastName: "Brown", FantasyPositions: []string{"WR"}, Active: true},
	// TE
	{PlayerID: "444", FirstName: "Travis", LastName: "Kelce", FantasyPositions: []string{"TE"}, Active: true},
	{PlayerID: "445", FirstName: "Sam", LastName: "LaPorta", FantasyPositions: []string{"TE"}, Active: true},
	{PlayerID: "446", FirstName: "Mark", LastName: "Andrews", FantasyPositions: []string{"TE"}, Active: true},
	// K
	{PlayerID: "711", FirstName: "Harrison", LastName: "Butker", FantasyPositions: []string{"K"}, Active: true},
	{PlayerID: "712", FirstName: "Justin", LastName: "Tucker", FantasyPositions: []string{"K"}, Active: true},
	// DEF
	{PlayerID: "811", FirstName: "San Francisco", LastName: "49ers", FantasyPositions: []string{"DEF"}, Active: true},
	{PlayerID: "812", FirstName: "Dallas", LastName: "Cowboys", FantasyPositions: []string{"DEF"}, Active: true},
}

func TestMain(m *testing.M) {
	ctx := context.Background()

	migrateURL := "pgx5://mirrorleague:mirrorleague@localhost:5433/mirrorleague_test"
	mg, err := migrate.New("file://../../migrations", migrateURL)
	if err != nil {
		log.Fatalf("TestMain: create migrator: %v", err)
	}
	if err := mg.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatalf("TestMain: migrate: %v", err)
	}

	pool, err := pgxpool.New(ctx, testDatabaseURL)
	if err != nil {
		log.Fatalf("TestMain: connect test db: %v", err)
	}
	if _, err := pool.Exec(ctx, "TRUNCATE users, lineups, players, league_bookmarks, week_locks, week_results, week_matchups, leagues, visits RESTART IDENTITY CASCADE"); err != nil {
		log.Fatalf("TestMain: truncate: %v", err)
	}
	if err := db.NewStore(pool).UpsertPlayers(ctx, testPlayers); err != nil {
		log.Fatalf("TestMain: seed players: %v", err)
	}
	code := m.Run()
	pool.Close()
	os.Exit(code)
}

func newTestServer(t *testing.T, sleeperHandler http.Handler, extraEnv ...map[string]string) string {
	t.Helper()

	pool, err := pgxpool.New(context.Background(), testDatabaseURL)
	if err != nil {
		t.Fatalf("newTestServer: connect db: %v", err)
	}
	if _, err := pool.Exec(context.Background(), "TRUNCATE users, lineups, players, league_bookmarks, week_locks, week_results, week_matchups, leagues, visits"); err != nil {
		t.Fatalf("newTestServer: truncate: %v", err)
	}
	if err := db.NewStore(pool).UpsertPlayers(context.Background(), testPlayers); err != nil {
		t.Fatalf("newTestServer: seed players: %v", err)
	}
	pool.Close()

	fakeSleeper := httptest.NewServer(sleeperHandler)
	t.Cleanup(fakeSleeper.Close)

	// fakeGoogle handles token exchange and userinfo for OAuth tests.
	// The identity returned is derived from the auth code: each unique code produces
	// a unique sub ("sub-<code>") and email ("<code>@test.example"), so tests that
	// call doGoogleLogin with distinct codes are fully isolated.
	fakeGoogle := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/token":
			r.ParseForm() //nolint:errcheck — test server, form always valid
			code := r.FormValue("code")
			json.NewEncoder(w).Encode(map[string]any{
				"access_token": code,
				"token_type":   "Bearer",
				"expires_in":   3600,
			})
		case "/oauth2/v2/userinfo":
			code := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			json.NewEncoder(w).Encode(map[string]any{
				"id":    "sub-" + code,
				"email": code + "@test.example",
			})
		}
	}))
	t.Cleanup(fakeGoogle.Close)

	port := freePort(t)
	getenv := func(key string) string {
		for _, env := range extraEnv {
			if v, ok := env[key]; ok {
				return v
			}
		}
		switch key {
		case "PORT":
			return port
		case "SLEEPER_BASE_URL":
			return fakeSleeper.URL
		case "DATABASE_URL":
			return testDatabaseURL
		case "MIGRATIONS_URL":
			return "file://../../migrations"
		case "GOOGLE_CLIENT_ID":
			return "test-client-id"
		case "GOOGLE_CLIENT_SECRET":
			return "test-client-secret"
		case "GOOGLE_REDIRECT_URL":
			return "http://localhost:" + port + "/auth/google/callback"
		case "GOOGLE_AUTH_URL":
			return fakeGoogle.URL + "/oauth2/v2/auth"
		case "GOOGLE_TOKEN_URL":
			return fakeGoogle.URL + "/token"
		case "GOOGLE_USERINFO_URL":
			return fakeGoogle.URL + "/oauth2/v2/userinfo"
		case "JWT_SECRET":
			return testJWTSecret
		case "ADMIN_SECRET":
			return testAdminSecret
		case "FRONTEND_URL":
			return "http://localhost:9999"
		}
		return ""
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go run(ctx, getenv, io.Discard, io.Discard)

	baseURL := "http://localhost:" + port
	if err := waitForReady(ctx, 5*time.Second, baseURL+"/healthz"); err != nil {
		t.Fatalf("server never became ready: %v", err)
	}
	return baseURL
}

// signTestJWT creates a valid HS256 JWT signed with testJWTSecret.
func signTestJWT(userID, email, username string) string {
	token, err := jwtauth.Sign([]byte(testJWTSecret), userID, email, username, username)
	if err != nil {
		panic(fmt.Sprintf("signTestJWT: %v", err))
	}
	return token
}

// authedJSONRequest builds a POST/PATCH request with Content-Type and Authorization headers set.
func authedJSONRequest(method, url, token, body string) *http.Request {
	req, _ := http.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

// authedGet issues a GET with a Bearer token (for requireAuth-protected reads like compare).
func authedGet(token, url string) (*http.Response, error) {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	return http.DefaultClient.Do(req)
}

// doGoogleLogin drives the full OAuth callback flow against a test server and
// returns the JWT from the auth_token cookie set on the callback response.
// code determines the fake identity: sub="sub-<code>", email="<code>@test.example".
// Pass a distinct code per test to ensure each test uses an isolated identity.
func doGoogleLogin(t *testing.T, baseURL, code string) string {
	t.Helper()
	noFollow := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Initiate login — get state cookie and state value from redirect URL.
	resp1, err := noFollow.Get(baseURL + "/auth/google")
	if err != nil {
		t.Fatalf("doGoogleLogin: initiate: %v", err)
	}
	resp1.Body.Close()
	if resp1.StatusCode != http.StatusFound {
		t.Fatalf("doGoogleLogin: expected 302 from login, got %d", resp1.StatusCode)
	}
	loc, err := url.Parse(resp1.Header.Get("Location"))
	if err != nil {
		t.Fatalf("doGoogleLogin: parse location: %v", err)
	}
	state := loc.Query().Get("state")
	if state == "" {
		t.Fatal("doGoogleLogin: no state in redirect URL")
	}
	var stateCookie *http.Cookie
	for _, c := range resp1.Cookies() {
		if c.Name == "oauth_state" {
			stateCookie = c
		}
	}
	if stateCookie == nil {
		t.Fatal("doGoogleLogin: no oauth_state cookie")
	}

	// Hit the callback with the state and the caller-supplied code.
	callbackURL := baseURL + "/auth/google/callback?code=" + url.QueryEscape(code) + "&state=" + url.QueryEscape(state)
	req2, _ := http.NewRequest(http.MethodGet, callbackURL, nil)
	req2.AddCookie(stateCookie)
	resp2, err := noFollow.Do(req2)
	if err != nil {
		t.Fatalf("doGoogleLogin: callback: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusFound {
		t.Fatalf("doGoogleLogin: callback expected 302, got %d", resp2.StatusCode)
	}
	for _, c := range resp2.Cookies() {
		if c.Name == "auth_token" {
			return c.Value
		}
	}
	t.Fatalf("doGoogleLogin: no auth_token cookie in callback response")
	return ""
}

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	defer l.Close()
	return strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
}

func waitForReady(ctx context.Context, timeout time.Duration, endpoint string) error {
	client := http.Client{}
	startTime := time.Now()
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if time.Since(startTime) >= timeout {
				return fmt.Errorf("timeout reached while waiting for endpoint")
			}
			time.Sleep(250 * time.Millisecond)
		}
	}
}

func TestGoogleCallback_NewUser(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())
	token := doGoogleLogin(t, baseURL, "new-user")

	claims, err := jwtauth.Validate([]byte(testJWTSecret), token)
	if err != nil {
		t.Fatalf("validating JWT: %v", err)
	}
	if claims.Email != "new-user@test.example" {
		t.Errorf("expected email %q in JWT, got %q", "new-user@test.example", claims.Email)
	}
	if claims.Subject == "" {
		t.Error("expected non-empty sub claim in JWT")
	}

	pool, err := pgxpool.New(context.Background(), testDatabaseURL)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	defer pool.Close()
	var count int
	if err := pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM users WHERE oauth_id = 'sub-new-user'").Scan(&count); err != nil {
		t.Fatalf("counting users: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 user row after new login, got %d", count)
	}
}

func TestGoogleCallback_ExistingUser(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())

	token1 := doGoogleLogin(t, baseURL, "existing-user")
	token2 := doGoogleLogin(t, baseURL, "existing-user")

	claims1, err := jwtauth.Validate([]byte(testJWTSecret), token1)
	if err != nil {
		t.Fatalf("validating JWT 1: %v", err)
	}
	claims2, err := jwtauth.Validate([]byte(testJWTSecret), token2)
	if err != nil {
		t.Fatalf("validating JWT 2: %v", err)
	}
	if claims1.Subject != claims2.Subject {
		t.Errorf("expected same sub on both logins, got %q and %q", claims1.Subject, claims2.Subject)
	}

	pool, err := pgxpool.New(context.Background(), testDatabaseURL)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	defer pool.Close()
	var count int
	if err := pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM users WHERE oauth_id = 'sub-existing-user'").Scan(&count); err != nil {
		t.Fatalf("counting users: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 user row after two logins with same Google identity, got %d", count)
	}
}

func TestAuthMe_Valid(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())
	token := signTestJWT("00000000-0000-0000-0000-000000000099", "me@example.com", "cool_bear")

	req, _ := http.NewRequest(http.MethodGet, baseURL+"/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var user provider.AuthUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if user.ID != "00000000-0000-0000-0000-000000000099" {
		t.Errorf("expected id %q, got %q", "00000000-0000-0000-0000-000000000099", user.ID)
	}
	if user.Email != "me@example.com" {
		t.Errorf("expected email %q, got %q", "me@example.com", user.Email)
	}
	if user.Username != "cool_bear" {
		t.Errorf("expected username %q, got %q", "cool_bear", user.Username)
	}
}

// authTokenFromResponse returns the value of the auth_token cookie set on a response, or "".
func authTokenFromResponse(resp *http.Response) string {
	for _, c := range resp.Cookies() {
		if c.Name == "auth_token" {
			return c.Value
		}
	}
	return ""
}

func TestUpdateProfile_Success(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())
	token := doGoogleLogin(t, baseURL, "profile-success")

	body := `{"username":"new_handle","display_name":"New Name"}`
	resp, err := http.DefaultClient.Do(authedJSONRequest(http.MethodPatch, baseURL+"/auth/profile", token, body))
	if err != nil {
		t.Fatalf("profile request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var user provider.AuthUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		t.Fatalf("decode profile response: %v", err)
	}
	if user.Username != "new_handle" {
		t.Errorf("expected username %q, got %q", "new_handle", user.Username)
	}
	if user.DisplayName != "New Name" {
		t.Errorf("expected display_name %q, got %q", "New Name", user.DisplayName)
	}

	// The re-issued cookie carries the new identity; /auth/me must reflect it.
	newToken := authTokenFromResponse(resp)
	if newToken == "" {
		t.Fatal("expected a re-issued auth_token cookie after profile update")
	}
	meResp, err := authedGet(newToken, baseURL+"/auth/me")
	if err != nil {
		t.Fatalf("/auth/me request failed: %v", err)
	}
	defer meResp.Body.Close()
	var me provider.AuthUser
	if err := json.NewDecoder(meResp.Body).Decode(&me); err != nil {
		t.Fatalf("decode /auth/me: %v", err)
	}
	if me.Username != "new_handle" || me.DisplayName != "New Name" {
		t.Errorf("/auth/me = {%q, %q}, want {new_handle, New Name}", me.Username, me.DisplayName)
	}

	// Persisted in the database.
	pool, err := pgxpool.New(context.Background(), testDatabaseURL)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	defer pool.Close()
	var dbUsername, dbDisplayName string
	if err := pool.QueryRow(context.Background(), "SELECT username, display_name FROM users WHERE id = $1", user.ID).Scan(&dbUsername, &dbDisplayName); err != nil {
		t.Fatalf("querying updated user: %v", err)
	}
	if dbUsername != "new_handle" || dbDisplayName != "New Name" {
		t.Errorf("db row = {%q, %q}, want {new_handle, New Name}", dbUsername, dbDisplayName)
	}
}

func TestUpdateProfile_NormalizesUsername(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())
	token := doGoogleLogin(t, baseURL, "profile-normalize")

	body := `{"username":"Bold_Hawk","display_name":"Bold Hawk"}`
	resp, err := http.DefaultClient.Do(authedJSONRequest(http.MethodPatch, baseURL+"/auth/profile", token, body))
	if err != nil {
		t.Fatalf("profile request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var user provider.AuthUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		t.Fatalf("decode profile response: %v", err)
	}
	if user.Username != "bold_hawk" {
		t.Errorf("expected normalized username %q, got %q", "bold_hawk", user.Username)
	}
}

func TestUpdateProfile_DisplayNameNotUnique(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())
	tokenA := doGoogleLogin(t, baseURL, "profile-dupdisplay-a")
	tokenB := doGoogleLogin(t, baseURL, "profile-dupdisplay-b")

	for _, tc := range []struct{ token, body string }{
		{tokenA, `{"username":"handle_aaa","display_name":"Shared Name"}`},
		{tokenB, `{"username":"handle_bbb","display_name":"Shared Name"}`},
	} {
		resp, err := http.DefaultClient.Do(authedJSONRequest(http.MethodPatch, baseURL+"/auth/profile", tc.token, tc.body))
		if err != nil {
			t.Fatalf("profile request failed: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 for shared display name %q, got %d", tc.body, resp.StatusCode)
		}
	}
}

func TestUpdateProfile_UsernameConflict(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())
	tokenA := doGoogleLogin(t, baseURL, "profile-conflict-a")
	tokenB := doGoogleLogin(t, baseURL, "profile-conflict-b")

	respA, err := http.DefaultClient.Do(authedJSONRequest(http.MethodPatch, baseURL+"/auth/profile", tokenA, `{"username":"taken_handle","display_name":"A"}`))
	if err != nil {
		t.Fatalf("user A profile request failed: %v", err)
	}
	respA.Body.Close()
	if respA.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for user A, got %d", respA.StatusCode)
	}

	// A case variant normalizes to the same handle and must be rejected as a conflict.
	respB, err := http.DefaultClient.Do(authedJSONRequest(http.MethodPatch, baseURL+"/auth/profile", tokenB, `{"username":"Taken_Handle","display_name":"B"}`))
	if err != nil {
		t.Fatalf("user B profile request failed: %v", err)
	}
	respB.Body.Close()
	if respB.StatusCode != http.StatusConflict {
		t.Errorf("expected 409 for case-insensitive username conflict, got %d", respB.StatusCode)
	}
}

func TestUpdateProfile_InvalidInputs(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())
	token := doGoogleLogin(t, baseURL, "profile-invalid")

	longUsername := strings.Repeat("a", 21)
	longDisplayName := strings.Repeat("a", 31)
	cases := []struct {
		name string
		body string
	}{
		{"username too short", `{"username":"ab","display_name":"Valid"}`},
		{"username too long", fmt.Sprintf(`{"username":%q,"display_name":"Valid"}`, longUsername)},
		{"username illegal chars", `{"username":"bad name!","display_name":"Valid"}`},
		{"display name empty", `{"username":"good_handle","display_name":"   "}`},
		{"display name too long", fmt.Sprintf(`{"username":"good_handle","display_name":%q}`, longDisplayName)},
		//  (BEL) decodes to a valid rune, so this exercises the control-char rejection.
		{"display name control char", "{\"username\":\"good_handle\",\"display_name\":\"bad\\u0007name\"}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.DefaultClient.Do(authedJSONRequest(http.MethodPatch, baseURL+"/auth/profile", token, tc.body))
			if err != nil {
				t.Fatalf("profile request failed: %v", err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("expected 400 for %s, got %d", tc.name, resp.StatusCode)
			}
		})
	}
}

func TestUpdateProfile_Unauthorized(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())

	req, _ := http.NewRequest(http.MethodPatch, baseURL+"/auth/profile", strings.NewReader(`{"username":"new_handle","display_name":"New Name"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuthMe_NoToken(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())

	resp, err := http.Get(baseURL + "/auth/me")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestLogout_ClearsCookie(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())

	req, _ := http.NewRequest(http.MethodDelete, baseURL+"/auth/logout", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	for _, c := range resp.Cookies() {
		if c.Name == "auth_token" && c.MaxAge < 0 {
			return
		}
	}
	t.Error("expected auth_token cookie with MaxAge < 0 in logout response")
}

// TestCors_VaryOriginSetOnCorsResponse asserts that any response passing through
// corsMiddleware with an Origin header present includes "Vary: Origin", so caches
// sitting in front of the API don't serve one origin's CORS headers to another.
func TestCors_VaryOriginSetOnCorsResponse(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())

	req, _ := http.NewRequest(http.MethodGet, baseURL+"/healthz", nil)
	req.Header.Set("Origin", "http://localhost:9999")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Vary"); got != "Origin" {
		t.Errorf("expected Vary: Origin, got %q", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:9999" {
		t.Errorf("expected Access-Control-Allow-Origin: http://localhost:9999, got %q", got)
	}
}

// TestCors_PlainOptionsFallsThroughToRouting asserts a bare OPTIONS request with no
// Origin header (and thus clearly not a CORS preflight) is not short-circuited into
// a 204 — it should fall through to normal mux routing, which returns 404 for a path
// with no registered handler.
func TestCors_PlainOptionsFallsThroughToRouting(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())

	req, _ := http.NewRequest(http.MethodOptions, baseURL+"/no-such-route", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		t.Errorf("plain OPTIONS with no Origin header should not be short-circuited to 204, got %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 from mux for unmatched route, got %d", resp.StatusCode)
	}
}

// TestCors_OptionsWithOriginNoRequestMethodFallsThrough asserts that an OPTIONS
// request with an Origin header but no Access-Control-Request-Method header is not
// treated as a preflight (browsers always send both together for a real preflight).
func TestCors_OptionsWithOriginNoRequestMethodFallsThrough(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())

	req, _ := http.NewRequest(http.MethodOptions, baseURL+"/no-such-route", nil)
	req.Header.Set("Origin", "http://localhost:9999")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		t.Errorf("OPTIONS with Origin but no Access-Control-Request-Method should not be short-circuited to 204, got %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 from mux for unmatched route, got %d", resp.StatusCode)
	}
}

// TestCors_PreflightWithOriginAndRequestMethod asserts a genuine preflight request —
// Origin + Access-Control-Request-Method both present — still gets the existing 204
// CORS response with the right headers.
func TestCors_PreflightWithOriginAndRequestMethod(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())

	req, _ := http.NewRequest(http.MethodOptions, baseURL+"/healthz", nil)
	req.Header.Set("Origin", "http://localhost:9999")
	req.Header.Set("Access-Control-Request-Method", "GET")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 for CORS preflight, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Vary"); got != "Origin" {
		t.Errorf("expected Vary: Origin, got %q", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:9999" {
		t.Errorf("expected Access-Control-Allow-Origin: http://localhost:9999, got %q", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Methods"); got == "" {
		t.Error("expected Access-Control-Allow-Methods to be set on preflight response")
	}
}

func TestDevLogin_IssuesUsableToken(t *testing.T) {
	baseURL := newTestServer(t, noopHandler(), map[string]string{"APP_ENV": "development"})

	noFollow := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := noFollow.Get(baseURL + "/dev/login")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected 302, got %d", resp.StatusCode)
	}
	var token string
	for _, c := range resp.Cookies() {
		if c.Name == "auth_token" {
			token = c.Value
		}
	}
	if token == "" {
		t.Fatal("expected auth_token cookie in dev login response")
	}

	// Token must be accepted by a protected route and carry the default dev identity.
	req, _ := http.NewRequest(http.MethodGet, baseURL+"/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	meResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("/auth/me request failed: %v", err)
	}
	defer meResp.Body.Close()
	if meResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from /auth/me with dev token, got %d", meResp.StatusCode)
	}
	var user provider.AuthUser
	if err := json.NewDecoder(meResp.Body).Decode(&user); err != nil {
		t.Fatalf("decode /auth/me: %v", err)
	}
	if user.ID != "00000000-0000-0000-0000-000000000001" {
		t.Errorf("expected dev user_id, got %q", user.ID)
	}
	if user.Email != "dev@localhost" {
		t.Errorf("expected dev email, got %q", user.Email)
	}
	if user.Username != "dev_user" {
		t.Errorf("expected dev username, got %q", user.Username)
	}
}

func TestDevLogin_NotAvailableInProduction(t *testing.T) {
	baseURL := newTestServer(t, noopHandler(), map[string]string{"APP_ENV": "production"})

	resp, err := http.Get(baseURL + "/dev/login")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for dev login in production, got %d", resp.StatusCode)
	}
}

func TestMerge_ReassociatesBookmarks(t *testing.T) {
	const anonID = "00000000-0000-0000-0000-000000000087"
	const realUserID = "00000000-0000-0000-0000-000000000088"

	baseURL := newTestServer(t, noopHandler())
	saveTestUserLeague(t, baseURL, anonID, "league-merge-1", "sleeper", "Anon Bookmark")

	token := signTestJWT(realUserID, "merge@example.com", "merge_user")
	body := fmt.Sprintf(`{"anonymous_id":%q}`, anonID)
	resp, err := http.DefaultClient.Do(authedJSONRequest(http.MethodPost, baseURL+"/auth/merge", token, body))
	if err != nil {
		t.Fatalf("merge request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	listResp, err := http.Get(baseURL + "/league-bookmarks?user_id=" + realUserID)
	if err != nil {
		t.Fatalf("list request failed: %v", err)
	}
	defer listResp.Body.Close()
	var leagues []provider.UserLeague
	if err := json.NewDecoder(listResp.Body).Decode(&leagues); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(leagues) != 1 || leagues[0].LeagueID != "league-merge-1" {
		t.Errorf("expected bookmark re-keyed to real user, got %+v", leagues)
	}

	anonResp, err := http.Get(baseURL + "/league-bookmarks?user_id=" + anonID)
	if err != nil {
		t.Fatalf("anon list request failed: %v", err)
	}
	defer anonResp.Body.Close()
	var anonLeagues []provider.UserLeague
	json.NewDecoder(anonResp.Body).Decode(&anonLeagues)
	if len(anonLeagues) != 0 {
		t.Errorf("expected 0 bookmarks under anon after merge, got %d", len(anonLeagues))
	}
}

func TestMerge_ReassociatesLineups(t *testing.T) {
	const anonID = "00000000-0000-0000-0000-000000000010"
	const realUserID = "00000000-0000-0000-0000-000000000011"

	baseURL := newTestServer(t, lineupSleeperHandler())

	anonToken := signTestJWT(anonID, "anon@example.com", "anon_user")
	createTestLineup(t, baseURL, anonToken)

	realToken := signTestJWT(realUserID, "real@example.com", "real_user")
	body := fmt.Sprintf(`{"anonymous_id":%q}`, anonID)
	resp, err := http.DefaultClient.Do(authedJSONRequest(http.MethodPost, baseURL+"/auth/merge", realToken, body))
	if err != nil {
		t.Fatalf("merge request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	listURL := baseURL + "/lineups?user_id=" + realUserID + "&league_id=test-league&week_number=1"
	listResp, err := http.Get(listURL)
	if err != nil {
		t.Fatalf("list request: %v", err)
	}
	defer listResp.Body.Close()
	var lineups []provider.Lineup
	if err := json.NewDecoder(listResp.Body).Decode(&lineups); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(lineups) != 1 {
		t.Errorf("expected 1 lineup under real user after merge, got %d", len(lineups))
	}

	anonListURL := baseURL + "/lineups?user_id=" + anonID + "&league_id=test-league&week_number=1"
	anonResp, err := http.Get(anonListURL)
	if err != nil {
		t.Fatalf("anon list request: %v", err)
	}
	defer anonResp.Body.Close()
	var anonLineups []provider.Lineup
	json.NewDecoder(anonResp.Body).Decode(&anonLineups)
	if len(anonLineups) != 0 {
		t.Errorf("expected 0 lineups under anon after merge, got %d", len(anonLineups))
	}
}

func TestMerge_Unauthenticated(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())

	body := `{"anonymous_id":"some-anon-id"}`
	resp, err := http.Post(baseURL+"/auth/merge", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestCreateLineup_Unauthenticated(t *testing.T) {
	baseURL := newTestServer(t, lineupSleeperHandler())

	body := `{"source":"sleeper","league_id":"test-league","roster_id":1,"week_number":1,"starters":["111","222"]}`
	resp, err := http.Post(baseURL+"/lineups", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestCreateLineup_Authenticated(t *testing.T) {
	const userID = "00000000-0000-0000-0000-000000000099"
	baseURL := newTestServer(t, lineupSleeperHandler())
	token := signTestJWT(userID, "auth@example.com", "auth_user")

	body := `{"source":"sleeper","league_id":"test-league","roster_id":1,"week_number":1,"starters":["111","222"]}`
	resp, err := http.DefaultClient.Do(authedJSONRequest(http.MethodPost, baseURL+"/lineups", token, body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var lineup provider.Lineup
	if err := json.NewDecoder(resp.Body).Decode(&lineup); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if lineup.UserID != userID {
		t.Errorf("expected user_id %q from JWT, got %q", userID, lineup.UserID)
	}
}

func TestGetRosters(t *testing.T) {
	baseURL := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/league/abc/rosters":
			json.NewEncoder(w).Encode([]map[string]any{
				{"roster_id": 1, "owner_id": "u1", "players": []string{"111"}, "starters": []string{"111"}},
			})
		case "/league/abc/users":
			json.NewEncoder(w).Encode([]map[string]any{
				{"user_id": "u1", "metadata": map[string]string{"team_name": "Test Team"}},
			})
		}
	}))

	resp, err := http.Get(baseURL + "/league/abc/rosters")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var rosters []provider.Roster
	if err := json.NewDecoder(resp.Body).Decode(&rosters); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(rosters) != 1 {
		t.Fatalf("expected 1 roster, got %d", len(rosters))
	}
	if rosters[0].TeamName != "Test Team" {
		t.Errorf("expected team name %q, got %q", "Test Team", rosters[0].TeamName)
	}
	if len(rosters[0].Players) != 1 || rosters[0].Players[0].PlayerID != "111" {
		t.Errorf("unexpected players: %+v", rosters[0].Players)
	}
}

func TestGetLeague(t *testing.T) {
	baseURL := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/league/abc" {
			json.NewEncoder(w).Encode(map[string]any{
				"league_id": "abc",
				"name":      "Test League",
				"season":    "2025",
			})
		}
	}))

	resp, err := http.Get(baseURL + "/league/abc")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var league provider.League
	if err := json.NewDecoder(resp.Body).Decode(&league); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if league.Name != "Test League" || league.Season != "2025" {
		t.Errorf("unexpected league: %+v", league)
	}
}

func TestGetLeagueNotFound(t *testing.T) {
	baseURL := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/league/bad-id" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, "null")
			return
		}
		http.NotFound(w, r)
	}))

	resp, err := http.Get(baseURL + "/league/bad-id")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestHealthz(t *testing.T) {
	baseURL := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	resp, err := http.Get(baseURL + "/healthz")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestSyncPlayers(t *testing.T) {
	// Fake rankings CSV: 23 columns, two players with known rarities.
	// Column indices: 1=page_type, 5=pos, 22=merge_name.
	fakeRankings := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "c0,page_type,c2,c3,c4,pos,c6,c7,c8,c9,c10,c11,c12,c13,c14,c15,c16,c17,c18,c19,c20,c21,merge_name\n")
		fmt.Fprint(w, "x,dynasty-qb,x,x,x,QB,x,x,x,x,x,x,x,x,x,x,x,x,x,x,x,x,Josh Allen\n")
		fmt.Fprint(w, "x,dynasty-rb,x,x,x,RB,x,x,x,x,x,x,x,x,x,x,x,x,x,x,x,x,Christian McCaffrey\n")
	}))
	t.Cleanup(fakeRankings.Close)

	baseURL := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/players/nfl" {
			json.NewEncoder(w).Encode(map[string]provider.Player{
				"111": {PlayerID: "111", FirstName: "Josh", LastName: "Allen", FantasyPositions: []string{"QB"}},
				"333": {PlayerID: "333", FirstName: "Christian", LastName: "McCaffrey", FantasyPositions: []string{"RB"}},
			})
		}
	}), map[string]string{"RANKINGS_CSV_URL": fakeRankings.URL})

	req, _ := http.NewRequest(http.MethodPost, baseURL+"/admin/sync-players", nil)
	req.Header.Set("X-Admin-Secret", testAdminSecret)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]int
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result["upserted"] != 2 {
		t.Errorf("expected 2 upserted, got %d", result["upserted"])
	}
}

func TestSyncPlayers_Unauthorized(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())

	t.Run("no secret", func(t *testing.T) {
		resp, err := http.Post(baseURL+"/admin/sync-players", "", nil)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", resp.StatusCode)
		}
	})

	t.Run("wrong secret", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, baseURL+"/admin/sync-players", nil)
		req.Header.Set("X-Admin-Secret", "wrong-secret")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", resp.StatusCode)
		}
	})
}

func lineupSleeperHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/league/test-league":
			json.NewEncoder(w).Encode(map[string]any{"league_id": "test-league", "name": "Test League", "season": "2025"})
		case "/league/test-league/matchups/1":
			json.NewEncoder(w).Encode([]map[string]any{
				{"roster_id": 1, "matchup_id": 1, "players": []string{"111", "222", "333"}, "starters": []string{"111"}, "points": 0.0},
			})
		case "/league/test-league/rosters":
			json.NewEncoder(w).Encode([]map[string]any{
				{"roster_id": 1, "owner_id": "owner1", "players": []string{"111", "222", "333"}, "starters": []string{"111"}},
			})
		case "/league/test-league/users":
			json.NewEncoder(w).Encode([]map[string]any{
				{"user_id": "owner1", "metadata": map[string]string{"team_name": "Test Team"}},
			})
		}
	})
}

// createTestLineup posts a lineup authenticated as token and returns the created lineup.
// user_id is taken from the JWT sub, not the request body.
func createTestLineup(t *testing.T, baseURL, token string) provider.Lineup {
	t.Helper()
	body := `{"source":"sleeper","league_id":"test-league","roster_id":1,"week_number":1,"starters":["111","222"]}`
	resp, err := http.DefaultClient.Do(authedJSONRequest(http.MethodPost, baseURL+"/lineups", token, body))
	if err != nil {
		t.Fatalf("createTestLineup: request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("createTestLineup: expected 201, got %d", resp.StatusCode)
	}
	var lineup provider.Lineup
	if err := json.NewDecoder(resp.Body).Decode(&lineup); err != nil {
		t.Fatalf("createTestLineup: failed to decode: %v", err)
	}
	return lineup
}

func TestCreateLineup(t *testing.T) {
	baseURL := newTestServer(t, lineupSleeperHandler())
	token := signTestJWT(testUserID, "test@example.com", "test_user")

	body := `{"source":"sleeper","league_id":"test-league","roster_id":1,"week_number":1,"starters":["111","222"]}`
	resp, err := http.DefaultClient.Do(authedJSONRequest(http.MethodPost, baseURL+"/lineups", token, body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var lineup provider.Lineup
	if err := json.NewDecoder(resp.Body).Decode(&lineup); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if lineup.ID == "" {
		t.Error("expected a non-empty lineup ID")
	}
	if loc := resp.Header.Get("Location"); loc != "/lineups/"+lineup.ID {
		t.Errorf("expected Location header %q, got %q", "/lineups/"+lineup.ID, loc)
	}
	if lineup.UserID != testUserID {
		t.Errorf("expected user_id %q, got %q", testUserID, lineup.UserID)
	}
	if lineup.LeagueID != "test-league" {
		t.Errorf("expected league_id %q, got %q", "test-league", lineup.LeagueID)
	}
	if lineup.RosterID != 1 {
		t.Errorf("expected roster_id 1, got %d", lineup.RosterID)
	}
	if lineup.WeekNumber != 1 {
		t.Errorf("expected week_number 1, got %d", lineup.WeekNumber)
	}
	if len(lineup.Starters) != 2 || lineup.Starters[0] != "111" || lineup.Starters[1] != "222" {
		t.Errorf("unexpected starters: %v", lineup.Starters)
	}
}

func TestCreateLineup_InvalidPlayer(t *testing.T) {
	baseURL := newTestServer(t, lineupSleeperHandler())
	token := signTestJWT(testUserID, "test@example.com", "test_user")

	body := `{"source":"sleeper","league_id":"test-league","roster_id":1,"week_number":1,"starters":["999"]}`
	resp, err := http.DefaultClient.Do(authedJSONRequest(http.MethodPost, baseURL+"/lineups", token, body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

// lineupSleeperHandlerMatchupsUnavailable mirrors lineupSleeperHandler but makes the
// matchups fetch fail with a 503 (simulating a Sleeper outage) instead of returning
// real matchup data. Used to verify GH #11: a failed matchup fetch must reject lineup
// submission rather than silently skipping validation (the old D17 behavior).
func lineupSleeperHandlerMatchupsUnavailable() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/league/test-league":
			json.NewEncoder(w).Encode(map[string]any{"league_id": "test-league", "name": "Test League", "season": "2025"})
		case "/league/test-league/matchups/1":
			// Realistic Sleeper degradation: a non-200 with a valid, empty JSON body —
			// not a broken/truncated one — so a decode-error check alone wouldn't catch
			// this; only an explicit status-code check does.
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode([]map[string]any{})
		case "/league/test-league/rosters":
			json.NewEncoder(w).Encode([]map[string]any{
				{"roster_id": 1, "owner_id": "owner1", "players": []string{"111", "222", "333"}, "starters": []string{"111"}},
			})
		case "/league/test-league/users":
			json.NewEncoder(w).Encode([]map[string]any{
				{"user_id": "owner1", "metadata": map[string]string{"team_name": "Test Team"}},
			})
		}
	})
}

// lineupSleeperHandlerNoMatchupsPublished mirrors lineupSleeperHandler but returns a
// legitimate, successful empty matchups response (e.g. matchups not published yet for
// the week). Used to verify GH #11: this is the one case that should still skip
// per-matchup validation (D17), unlike a fetch failure.
func lineupSleeperHandlerNoMatchupsPublished() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/league/test-league":
			json.NewEncoder(w).Encode(map[string]any{"league_id": "test-league", "name": "Test League", "season": "2025"})
		case "/league/test-league/matchups/1":
			json.NewEncoder(w).Encode([]map[string]any{})
		case "/league/test-league/rosters":
			json.NewEncoder(w).Encode([]map[string]any{
				{"roster_id": 1, "owner_id": "owner1", "players": []string{"111", "222", "333"}, "starters": []string{"111"}},
			})
		case "/league/test-league/users":
			json.NewEncoder(w).Encode([]map[string]any{
				{"user_id": "owner1", "metadata": map[string]string{"team_name": "Test Team"}},
			})
		}
	})
}

func TestCreateLineup_MatchupFetchFailsRejectsSubmission(t *testing.T) {
	baseURL := newTestServer(t, lineupSleeperHandlerMatchupsUnavailable())
	token := signTestJWT(testUserID, "test@example.com", "test_user")

	// "999" isn't on the roster at all — if validation were silently skipped (the
	// old D17 bug during an outage), this illegal lineup would be accepted.
	body := `{"source":"sleeper","league_id":"test-league","roster_id":1,"week_number":1,"starters":["999"]}`
	resp, err := http.DefaultClient.Do(authedJSONRequest(http.MethodPost, baseURL+"/lineups", token, body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		t.Fatal("expected lineup submission to be rejected when the matchup fetch fails, got 201 Created")
	}
}

func TestCreateLineup_NoMatchupsPublishedStillAccepted(t *testing.T) {
	baseURL := newTestServer(t, lineupSleeperHandlerNoMatchupsPublished())
	token := signTestJWT(testUserID, "test@example.com", "test_user")

	body := `{"source":"sleeper","league_id":"test-league","roster_id":1,"week_number":1,"starters":["111","222"]}`
	resp, err := http.DefaultClient.Do(authedJSONRequest(http.MethodPost, baseURL+"/lineups", token, body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected a legitimately empty (successful) matchups response to still allow lineup creation (D17), got %d", resp.StatusCode)
	}
}

func TestGetLineup(t *testing.T) {
	baseURL := newTestServer(t, lineupSleeperHandler())
	token := signTestJWT(testUserID, "test@example.com", "test_user")
	created := createTestLineup(t, baseURL, token)

	resp, err := http.Get(baseURL + "/lineups/" + created.ID)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var lineup provider.Lineup
	if err := json.NewDecoder(resp.Body).Decode(&lineup); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if lineup.ID != created.ID {
		t.Errorf("expected id %q, got %q", created.ID, lineup.ID)
	}
}

func TestGetLineup_NotFound(t *testing.T) {
	baseURL := newTestServer(t, lineupSleeperHandler())

	resp, err := http.Get(baseURL + "/lineups/00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestUpdateLineup(t *testing.T) {
	baseURL := newTestServer(t, lineupSleeperHandler())
	token := signTestJWT(testUserID, "test@example.com", "test_user")
	created := createTestLineup(t, baseURL, token)

	body := `{"starters":["111","333"]}`
	resp, err := http.DefaultClient.Do(authedJSONRequest(http.MethodPatch, baseURL+"/lineups/"+created.ID, token, body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var lineup provider.Lineup
	if err := json.NewDecoder(resp.Body).Decode(&lineup); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(lineup.Starters) != 2 || lineup.Starters[0] != "111" || lineup.Starters[1] != "333" {
		t.Errorf("unexpected starters: %v", lineup.Starters)
	}
}

func TestUpdateLineup_WrongUser(t *testing.T) {
	baseURL := newTestServer(t, lineupSleeperHandler())
	token1 := signTestJWT(testUserID, "user1@example.com", "user_one")
	token2 := signTestJWT("00000000-0000-0000-0000-000000000002", "user2@example.com", "user_two")
	created := createTestLineup(t, baseURL, token1)

	body := `{"starters":["111"]}`
	resp, err := http.DefaultClient.Do(authedJSONRequest(http.MethodPatch, baseURL+"/lineups/"+created.ID, token2, body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func TestUpdateLineup_NotFound(t *testing.T) {
	baseURL := newTestServer(t, lineupSleeperHandler())
	token := signTestJWT(testUserID, "test@example.com", "test_user")

	body := `{"starters":["111"]}`
	resp, err := http.DefaultClient.Do(authedJSONRequest(http.MethodPatch, baseURL+"/lineups/00000000-0000-0000-0000-000000000000", token, body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestListLineups_FilterByRoster(t *testing.T) {
	baseURL := newTestServer(t, lineupSleeperHandler())
	token := signTestJWT(testUserID, "test@example.com", "test_user")
	created := createTestLineup(t, baseURL, token)

	url := baseURL + "/lineups?user_id=" + created.UserID + "&league_id=test-league&week_number=1&roster_id=1"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var lineups []provider.Lineup
	if err := json.NewDecoder(resp.Body).Decode(&lineups); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(lineups) != 1 {
		t.Fatalf("expected 1 lineup, got %d", len(lineups))
	}
	if lineups[0].ID != created.ID {
		t.Errorf("expected id %q, got %q", created.ID, lineups[0].ID)
	}
}

func TestListLineups_All(t *testing.T) {
	baseURL := newTestServer(t, lineupSleeperHandler())
	token := signTestJWT(testUserID, "test@example.com", "test_user")
	created := createTestLineup(t, baseURL, token)

	url := baseURL + "/lineups?user_id=" + created.UserID + "&league_id=test-league&week_number=1"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var lineups []provider.Lineup
	if err := json.NewDecoder(resp.Body).Decode(&lineups); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(lineups) != 1 {
		t.Fatalf("expected 1 lineup, got %d", len(lineups))
	}
	if lineups[0].ID != created.ID {
		t.Errorf("expected id %q, got %q", created.ID, lineups[0].ID)
	}
}

func TestListLineups_Empty(t *testing.T) {
	baseURL := newTestServer(t, lineupSleeperHandler())

	url := baseURL + "/lineups?user_id=00000000-0000-0000-0000-000000000001&league_id=test-league&week_number=99"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var lineups []provider.Lineup
	if err := json.NewDecoder(resp.Body).Decode(&lineups); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(lineups) != 0 {
		t.Errorf("expected empty array, got %d lineups", len(lineups))
	}
}

func TestGetWeekMatchups(t *testing.T) {
	baseURL := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/league/abc":
			json.NewEncoder(w).Encode(map[string]any{"league_id": "abc", "name": "Test League", "season": "2025"})
		case "/league/abc/matchups/8":
			json.NewEncoder(w).Encode([]map[string]any{
				{"roster_id": 1, "matchup_id": 1, "players": []string{"111", "222"}, "starters": []string{"111"}, "points": 95.5, "custom_points": nil, "players_points": map[string]float64{"111": 22.4, "222": 8.1}},
				{"roster_id": 2, "matchup_id": 1, "players": []string{"333"}, "starters": []string{"333"}, "points": 88.0, "custom_points": nil},
			})
		case "/league/abc/rosters":
			json.NewEncoder(w).Encode([]map[string]any{
				{"roster_id": 1, "owner_id": "u1", "players": []string{"111", "222"}, "starters": []string{"111"}},
				{"roster_id": 2, "owner_id": "u2", "players": []string{"333"}, "starters": []string{"333"}},
			})
		case "/league/abc/users":
			json.NewEncoder(w).Encode([]map[string]any{
				{"user_id": "u1", "metadata": map[string]string{"team_name": "Team One"}},
				{"user_id": "u2", "metadata": map[string]string{"team_name": "Team Two"}},
			})
		}
	}))

	resp, err := http.Get(baseURL + "/league/abc/week/8")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var envelope provider.WeekMatchupsResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	matchups := envelope.Matchups
	if len(matchups) != 2 {
		t.Fatalf("expected 2 matchups, got %d", len(matchups))
	}
	if matchups[0].TeamName != "Team One" {
		t.Errorf("expected team name %q, got %q", "Team One", matchups[0].TeamName)
	}
	if len(matchups[0].Players) != 2 {
		t.Errorf("expected 2 players on roster 1, got %d", len(matchups[0].Players))
	}
	if matchups[0].Players[0].PlayerID != "111" {
		t.Errorf("expected player 111, got %s", matchups[0].Players[0].PlayerID)
	}
	if matchups[0].Points != 95.5 {
		t.Errorf("expected points 95.5, got %f", matchups[0].Points)
	}
	if matchups[0].CustomPoints != nil {
		t.Errorf("expected custom_points nil, got %v", matchups[0].CustomPoints)
	}
	if matchups[0].PlayerPoints["111"] != 22.4 {
		t.Errorf("expected player 111 points 22.4, got %f", matchups[0].PlayerPoints["111"])
	}
	if matchups[0].PlayerPoints["222"] != 8.1 {
		t.Errorf("expected player 222 points 8.1, got %f", matchups[0].PlayerPoints["222"])
	}
	if matchups[1].PlayerPoints != nil {
		t.Errorf("expected nil PlayerPoints for roster 2, got %v", matchups[1].PlayerPoints)
	}
}

func TestGetWeekMatchups_InvalidWeek(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())

	resp, err := http.Get(baseURL + "/league/abc/week/notanumber")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestGetWeekMatchups_ZeroWeek(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())

	resp, err := http.Get(baseURL + "/league/abc/week/0")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func compareSleeperHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/league/abc":
			json.NewEncoder(w).Encode(map[string]any{"league_id": "abc", "name": "Test League", "season": "2025"})
		case "/league/abc/matchups/8":
			json.NewEncoder(w).Encode([]map[string]any{
				{
					"roster_id": 1, "matchup_id": 5,
					"players":        []string{"111", "222", "333"},
					"starters":       []string{"111", "222"},
					"points":         30.5,
					"players_points": map[string]float64{"111": 22.4, "222": 8.1, "333": 15.0},
				},
			})
		case "/league/abc/rosters":
			json.NewEncoder(w).Encode([]map[string]any{
				{"roster_id": 1, "owner_id": "owner1", "players": []string{"111", "222", "333"}, "starters": []string{"111", "222"}},
			})
		case "/league/abc/users":
			json.NewEncoder(w).Encode([]map[string]any{
				{"user_id": "owner1", "metadata": map[string]string{"team_name": "Test Team"}},
			})
		}
	})
}

// createLineupForCompare posts a lineup authenticated as token for the compare-score test fixture.
func createLineupForCompare(t *testing.T, baseURL, token string) provider.Lineup {
	t.Helper()
	body := `{"source":"sleeper","league_id":"abc","roster_id":1,"week_number":8,"starters":["111","333"]}`
	resp, err := http.DefaultClient.Do(authedJSONRequest(http.MethodPost, baseURL+"/lineups", token, body))
	if err != nil {
		t.Fatalf("createLineupForCompare: request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("createLineupForCompare: expected 201, got %d", resp.StatusCode)
	}
	var lineup provider.Lineup
	if err := json.NewDecoder(resp.Body).Decode(&lineup); err != nil {
		t.Fatalf("createLineupForCompare: failed to decode: %v", err)
	}
	return lineup
}

func TestCompareLineup(t *testing.T) {
	const compareUserID = "00000000-0000-0000-0000-000000000002"
	baseURL := newTestServer(t, compareSleeperHandler())
	token := signTestJWT(compareUserID, "compare@example.com", "compare_user")
	createLineupForCompare(t, baseURL, token)

	resp, err := authedGet(token, baseURL+"/league/abc/week/8/roster/1/compare")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result provider.CompareResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result.Official.TotalPoints != 30.5 {
		t.Errorf("expected official total_points 30.5, got %f", result.Official.TotalPoints)
	}
	if result.User.TotalPoints != 37.4 {
		t.Errorf("expected user total_points 37.4 (22.4+15.0), got %f", result.User.TotalPoints)
	}
	if result.Winner != "user" {
		t.Errorf("expected winner %q, got %q", "user", result.Winner)
	}
	if result.User.LineupID == "" {
		t.Errorf("expected user lineup_id to be set")
	}
	if result.Official.LineupID != "" {
		t.Errorf("expected official lineup_id to be empty, got %q", result.Official.LineupID)
	}
	if len(result.User.Starters) != 2 {
		t.Errorf("expected 2 user starters, got %d", len(result.User.Starters))
	}
	if len(result.Official.Starters) != 2 {
		t.Errorf("expected 2 official starters, got %d", len(result.Official.Starters))
	}
}

func TestCompareLineup_NoLineup(t *testing.T) {
	baseURL := newTestServer(t, compareSleeperHandler())
	// Authenticated, but this user submitted no lineup for the week → 404.
	token := signTestJWT("00000000-0000-0000-0000-000000000044", "nolineup@example.com", "no_lineup")

	resp, err := authedGet(token, baseURL+"/league/abc/week/8/roster/1/compare")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestCompareLineup_InvalidWeek(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())
	token := signTestJWT("00000000-0000-0000-0000-000000000045", "iw@example.com", "invalid_week")

	resp, err := authedGet(token, baseURL+"/league/abc/week/notanumber/roster/1/compare")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCompare_RequiresAuth(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())

	// No Authorization header → requireAuth rejects before the handler runs.
	resp, err := http.Get(baseURL + "/league/abc/week/8/roster/1/compare")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func noopHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
}

// seedWeekLock upserts a week_locks row so a test can control whether a given
// (season, week) is locked. Pass a past locksAt to assert writes are rejected,
// or a future locksAt to assert they are allowed.
func seedWeekLock(t *testing.T, season string, week int, locksAt time.Time) {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDatabaseURL)
	if err != nil {
		t.Fatalf("seedWeekLock: connect db: %v", err)
	}
	defer pool.Close()
	_, err = pool.Exec(context.Background(),
		`INSERT INTO week_locks (season, week, locks_at) VALUES ($1, $2, $3)
		 ON CONFLICT (season, week) DO UPDATE SET locks_at = EXCLUDED.locks_at`,
		season, week, locksAt)
	if err != nil {
		t.Fatalf("seedWeekLock: insert: %v", err)
	}
}

// weekLockMatchupHandler serves league "abc" for week 8 (league object, matchups,
// rosters, users) so the week-matchups envelope + lock state can be asserted.
func weekLockMatchupHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/league/abc":
			json.NewEncoder(w).Encode(map[string]any{"league_id": "abc", "name": "Test League", "season": "2025"})
		case "/league/abc/matchups/8":
			json.NewEncoder(w).Encode([]map[string]any{
				{"roster_id": 1, "matchup_id": 1, "players": []string{"111", "222"}, "starters": []string{"111"}, "points": 95.5},
			})
		case "/league/abc/rosters":
			json.NewEncoder(w).Encode([]map[string]any{
				{"roster_id": 1, "owner_id": "u1", "players": []string{"111", "222"}, "starters": []string{"111"}},
			})
		case "/league/abc/users":
			json.NewEncoder(w).Encode([]map[string]any{
				{"user_id": "u1", "metadata": map[string]string{"team_name": "Team One"}},
			})
		}
	})
}

func TestCreateLineup_RejectedAfterLock(t *testing.T) {
	baseURL := newTestServer(t, lineupSleeperHandler())
	token := signTestJWT(testUserID, "test@example.com", "test_user")
	seedWeekLock(t, "2025", 1, time.Now().Add(-time.Hour))

	body := `{"source":"sleeper","league_id":"test-league","roster_id":1,"week_number":1,"starters":["111","222"]}`
	resp, err := http.DefaultClient.Do(authedJSONRequest(http.MethodPost, baseURL+"/lineups", token, body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 after lock, got %d", resp.StatusCode)
	}

	// Nothing should have been persisted.
	listResp, err := http.Get(baseURL + "/lineups?user_id=" + testUserID + "&league_id=test-league&week_number=1")
	if err != nil {
		t.Fatalf("list request failed: %v", err)
	}
	defer listResp.Body.Close()
	var lineups []provider.Lineup
	if err := json.NewDecoder(listResp.Body).Decode(&lineups); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(lineups) != 0 {
		t.Errorf("expected no lineup persisted after locked create, got %d", len(lineups))
	}
}

func TestCreateLineup_AllowedBeforeLock(t *testing.T) {
	baseURL := newTestServer(t, lineupSleeperHandler())
	token := signTestJWT(testUserID, "test@example.com", "test_user")
	seedWeekLock(t, "2025", 1, time.Now().Add(time.Hour))

	body := `{"source":"sleeper","league_id":"test-league","roster_id":1,"week_number":1,"starters":["111","222"]}`
	resp, err := http.DefaultClient.Do(authedJSONRequest(http.MethodPost, baseURL+"/lineups", token, body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 before lock, got %d", resp.StatusCode)
	}
}

func TestCreateLineup_StoresDerivedSeason(t *testing.T) {
	baseURL := newTestServer(t, lineupSleeperHandler())
	token := signTestJWT(testUserID, "test@example.com", "test_user")

	lineup := createTestLineup(t, baseURL, token)
	if lineup.Season != "2025" {
		t.Errorf("expected season %q derived from league, got %q", "2025", lineup.Season)
	}
}

func TestUpdateLineup_RejectedAfterLock(t *testing.T) {
	baseURL := newTestServer(t, lineupSleeperHandler())
	token := signTestJWT(testUserID, "test@example.com", "test_user")
	seedWeekLock(t, "2025", 1, time.Now().Add(time.Hour))
	created := createTestLineup(t, baseURL, token)

	// Week locks (kickoff passes).
	seedWeekLock(t, "2025", 1, time.Now().Add(-time.Hour))

	body := `{"starters":["111","333"]}`
	resp, err := http.DefaultClient.Do(authedJSONRequest(http.MethodPatch, baseURL+"/lineups/"+created.ID, token, body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 after lock, got %d", resp.StatusCode)
	}

	// Starters must be unchanged.
	getResp, err := http.Get(baseURL + "/lineups/" + created.ID)
	if err != nil {
		t.Fatalf("get request failed: %v", err)
	}
	defer getResp.Body.Close()
	var lineup provider.Lineup
	if err := json.NewDecoder(getResp.Body).Decode(&lineup); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(lineup.Starters) != 2 || lineup.Starters[0] != "111" || lineup.Starters[1] != "222" {
		t.Errorf("expected starters unchanged after rejected update, got %v", lineup.Starters)
	}
}

func TestUpdateLineup_AllowedBeforeLock(t *testing.T) {
	baseURL := newTestServer(t, lineupSleeperHandler())
	token := signTestJWT(testUserID, "test@example.com", "test_user")
	seedWeekLock(t, "2025", 1, time.Now().Add(time.Hour))
	created := createTestLineup(t, baseURL, token)

	body := `{"starters":["111","333"]}`
	resp, err := http.DefaultClient.Do(authedJSONRequest(http.MethodPatch, baseURL+"/lineups/"+created.ID, token, body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 before lock, got %d", resp.StatusCode)
	}
}

// TestListLineups_NonOwnerHidesPreLockLineups verifies the GH #9 fix: an unauthenticated
// caller who knows a registered user's real user_id (e.g. harvested from GET /leaderboard)
// cannot read that user's lineup for a week that has not yet locked.
func TestListLineups_NonOwnerHidesPreLockLineups(t *testing.T) {
	baseURL := newTestServer(t, lineupSleeperHandler())
	victimToken := doGoogleLogin(t, baseURL, "lineup-victim-prelock")
	victimClaims, err := jwtauth.Validate([]byte(testJWTSecret), victimToken)
	if err != nil {
		t.Fatalf("failed to validate victim token: %v", err)
	}
	victimID := victimClaims.Subject

	seedWeekLock(t, "2025", 1, time.Now().Add(time.Hour)) // not yet locked
	createTestLineup(t, baseURL, victimToken)

	resp, err := http.Get(baseURL + "/lineups?user_id=" + victimID + "&league_id=test-league&week_number=1")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var lineups []provider.Lineup
	if err := json.NewDecoder(resp.Body).Decode(&lineups); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(lineups) != 0 {
		t.Errorf("expected pre-lock lineup hidden from non-owner, got %d", len(lineups))
	}
}

// TestListLineups_NonOwnerSeesPostLockLineups verifies visibility is restored once the
// week locks — pre-lock secrecy is the whole point, post-lock is what the results
// browser is for.
func TestListLineups_NonOwnerSeesPostLockLineups(t *testing.T) {
	baseURL := newTestServer(t, lineupSleeperHandler())
	victimToken := doGoogleLogin(t, baseURL, "lineup-victim-postlock")
	victimClaims, err := jwtauth.Validate([]byte(testJWTSecret), victimToken)
	if err != nil {
		t.Fatalf("failed to validate victim token: %v", err)
	}
	victimID := victimClaims.Subject

	seedWeekLock(t, "2025", 1, time.Now().Add(time.Hour))
	createTestLineup(t, baseURL, victimToken)
	seedWeekLock(t, "2025", 1, time.Now().Add(-time.Hour)) // kickoff passes

	resp, err := http.Get(baseURL + "/lineups?user_id=" + victimID + "&league_id=test-league&week_number=1")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	var lineups []provider.Lineup
	if err := json.NewDecoder(resp.Body).Decode(&lineups); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(lineups) != 1 {
		t.Errorf("expected post-lock lineup visible to non-owner, got %d", len(lineups))
	}
}

// TestListLineups_OwnerSeesOwnPreLockLineups verifies the fix doesn't regress the normal
// case: an authenticated user reading their own lineups always sees everything, locked or
// not.
func TestListLineups_OwnerSeesOwnPreLockLineups(t *testing.T) {
	baseURL := newTestServer(t, lineupSleeperHandler())
	token := doGoogleLogin(t, baseURL, "lineup-owner-prelock")
	claims, err := jwtauth.Validate([]byte(testJWTSecret), token)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}
	userID := claims.Subject

	seedWeekLock(t, "2025", 1, time.Now().Add(time.Hour)) // not yet locked
	createTestLineup(t, baseURL, token)

	resp, err := authedGet(token, baseURL+"/lineups?user_id="+userID+"&league_id=test-league&week_number=1")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	var lineups []provider.Lineup
	if err := json.NewDecoder(resp.Body).Decode(&lineups); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(lineups) != 1 {
		t.Errorf("expected owner to see their own pre-lock lineup, got %d", len(lineups))
	}
}

// TestListLineups_AnonymousUserStillFullyVisible is a regression check: an anonymous
// localStorage id (never signed in — no row in users) is not a harvestable identifier via
// GET /leaderboard, so the pre-existing anon trust model (client-supplied id is trusted,
// same as GH #8) must be unaffected by this fix.
func TestListLineups_AnonymousUserStillFullyVisible(t *testing.T) {
	baseURL := newTestServer(t, lineupSleeperHandler())
	anonToken := signTestJWT(testUserID, "anon@example.com", "anon_user")

	seedWeekLock(t, "2025", 1, time.Now().Add(time.Hour)) // not yet locked
	createTestLineup(t, baseURL, anonToken)

	resp, err := http.Get(baseURL + "/lineups?user_id=" + testUserID + "&league_id=test-league&week_number=1")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	var lineups []provider.Lineup
	if err := json.NewDecoder(resp.Body).Decode(&lineups); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(lineups) != 1 {
		t.Errorf("expected anon lineup still visible pre-lock, got %d", len(lineups))
	}
}

func TestWeekMatchups_EnvelopeLockedTrue(t *testing.T) {
	baseURL := newTestServer(t, weekLockMatchupHandler())
	seedWeekLock(t, "2025", 8, time.Now().Add(-time.Hour))

	resp, err := http.Get(baseURL + "/league/abc/week/8")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var envelope provider.WeekMatchupsResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if !envelope.Locked {
		t.Error("expected locked=true after kickoff")
	}
	if envelope.LocksAt == nil {
		t.Error("expected locks_at to be set")
	}
	if len(envelope.Matchups) != 1 {
		t.Errorf("expected 1 matchup in envelope, got %d", len(envelope.Matchups))
	}
}

func TestWeekMatchups_EnvelopeLockedFalse(t *testing.T) {
	baseURL := newTestServer(t, weekLockMatchupHandler())
	seedWeekLock(t, "2025", 8, time.Now().Add(time.Hour))

	resp, err := http.Get(baseURL + "/league/abc/week/8")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var envelope provider.WeekMatchupsResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if envelope.Locked {
		t.Error("expected locked=false before kickoff")
	}
	if envelope.LocksAt == nil {
		t.Error("expected locks_at to be set even when not yet locked")
	}
}

func TestWeekMatchups_EnvelopeNoRow(t *testing.T) {
	baseURL := newTestServer(t, weekLockMatchupHandler())
	// No week_locks row seeded — fail open.

	resp, err := http.Get(baseURL + "/league/abc/week/8")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var envelope provider.WeekMatchupsResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if envelope.Locked {
		t.Error("expected locked=false when no lock row seeded")
	}
	if envelope.LocksAt != nil {
		t.Errorf("expected locks_at nil when no lock row, got %v", envelope.LocksAt)
	}
}

func TestLineupRead_EchoesLocked(t *testing.T) {
	baseURL := newTestServer(t, lineupSleeperHandler())
	token := signTestJWT(testUserID, "test@example.com", "test_user")
	seedWeekLock(t, "2025", 1, time.Now().Add(time.Hour))
	created := createTestLineup(t, baseURL, token)

	// Week locks.
	seedWeekLock(t, "2025", 1, time.Now().Add(-time.Hour))

	resp, err := http.Get(baseURL + "/lineups/" + created.ID)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	var lineup provider.Lineup
	if err := json.NewDecoder(resp.Body).Decode(&lineup); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if !lineup.Locked {
		t.Error("expected lineup read to echo locked=true after kickoff")
	}
}

func saveTestUserLeague(t *testing.T, baseURL, userID, leagueID, source, label string) provider.UserLeague {
	t.Helper()
	body := fmt.Sprintf(`{"user_id":%q,"league_id":%q,"source":%q,"label":%q}`, userID, leagueID, source, label)
	resp, err := http.Post(baseURL+"/league-bookmarks", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("saveTestUserLeague: request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("saveTestUserLeague: expected 200, got %d", resp.StatusCode)
	}
	var ul provider.UserLeague
	if err := json.NewDecoder(resp.Body).Decode(&ul); err != nil {
		t.Fatalf("saveTestUserLeague: failed to decode: %v", err)
	}
	return ul
}

func TestSaveUserLeague(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())

	ul := saveTestUserLeague(t, baseURL, "user-a", "league-1", "sleeper", "My League")

	if ul.UserID != "user-a" {
		t.Errorf("expected user_id %q, got %q", "user-a", ul.UserID)
	}
	if ul.LeagueID != "league-1" {
		t.Errorf("expected league_id %q, got %q", "league-1", ul.LeagueID)
	}
	if ul.Label != "My League" {
		t.Errorf("expected label %q, got %q", "My League", ul.Label)
	}
}

func TestListUserLeagues(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())
	saveTestUserLeague(t, baseURL, "user-a", "league-1", "sleeper", "My League")

	resp, err := http.Get(baseURL + "/league-bookmarks?user_id=user-a")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var leagues []provider.UserLeague
	if err := json.NewDecoder(resp.Body).Decode(&leagues); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(leagues) != 1 {
		t.Fatalf("expected 1 league, got %d", len(leagues))
	}
	if leagues[0].LeagueID != "league-1" || leagues[0].Label != "My League" {
		t.Errorf("unexpected league: %+v", leagues[0])
	}
}

func TestListUserLeagues_Empty(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())

	resp, err := http.Get(baseURL + "/league-bookmarks?user_id=user-nobody")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var leagues []provider.UserLeague
	if err := json.NewDecoder(resp.Body).Decode(&leagues); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(leagues) != 0 {
		t.Errorf("expected empty array, got %d leagues", len(leagues))
	}
}

func TestListUserLeagues_Isolation(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())
	saveTestUserLeague(t, baseURL, "user-a", "league-1", "sleeper", "User A League")

	resp, err := http.Get(baseURL + "/league-bookmarks?user_id=user-b")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var leagues []provider.UserLeague
	if err := json.NewDecoder(resp.Body).Decode(&leagues); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(leagues) != 0 {
		t.Errorf("expected user-b to see no leagues, got %d", len(leagues))
	}
}

func TestSaveUserLeague_Upsert(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())
	saveTestUserLeague(t, baseURL, "user-a", "league-1", "sleeper", "First Label")
	ul := saveTestUserLeague(t, baseURL, "user-a", "league-1", "sleeper", "Updated Label")

	if ul.Label != "Updated Label" {
		t.Errorf("expected updated label, got %q", ul.Label)
	}

	resp, err := http.Get(baseURL + "/league-bookmarks?user_id=user-a")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var leagues []provider.UserLeague
	if err := json.NewDecoder(resp.Body).Decode(&leagues); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(leagues) != 1 {
		t.Errorf("expected 1 entry after upsert, got %d", len(leagues))
	}
	if leagues[0].Label != "Updated Label" {
		t.Errorf("expected %q, got %q", "Updated Label", leagues[0].Label)
	}
}

func TestUpdateUserLeague(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())
	saveTestUserLeague(t, baseURL, "user-a", "league-1", "sleeper", "Old Label")

	body := `{"user_id":"user-a","label":"New Label"}`
	req, _ := http.NewRequest(http.MethodPatch, baseURL+"/league-bookmarks/league-1?source=sleeper", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var ul provider.UserLeague
	if err := json.NewDecoder(resp.Body).Decode(&ul); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if ul.Label != "New Label" {
		t.Errorf("expected %q, got %q", "New Label", ul.Label)
	}
}

func TestUpdateUserLeague_NotFound(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())

	body := `{"user_id":"user-a","label":"Whatever"}`
	req, _ := http.NewRequest(http.MethodPatch, baseURL+"/league-bookmarks/nonexistent?source=sleeper", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestDeleteUserLeague(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())
	saveTestUserLeague(t, baseURL, "user-a", "league-1", "sleeper", "To Delete")

	req, _ := http.NewRequest(http.MethodDelete, baseURL+"/league-bookmarks/league-1?user_id=user-a&source=sleeper", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	listResp, err := http.Get(baseURL + "/league-bookmarks?user_id=user-a")
	if err != nil {
		t.Fatalf("list request failed: %v", err)
	}
	defer listResp.Body.Close()

	var leagues []provider.UserLeague
	if err := json.NewDecoder(listResp.Body).Decode(&leagues); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(leagues) != 0 {
		t.Errorf("expected empty list after delete, got %d", len(leagues))
	}
}

func TestDeleteUserLeague_NotFound(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())

	req, _ := http.NewRequest(http.MethodDelete, baseURL+"/league-bookmarks/nonexistent?user_id=user-a&source=sleeper", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// TestSaveUserLeague_AuthedUsesClaimsUserID verifies that an authenticated request cannot
// choose which user a bookmark is saved under — the bookmark must be re-keyed to the JWT
// subject even if the body claims a different user_id. See GH #8.
func TestSaveUserLeague_AuthedUsesClaimsUserID(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())
	token := signTestJWT("real-user", "real@test.example", "realuser")

	body := `{"user_id":"victim-id","league_id":"league-1","source":"sleeper","label":"My League"}`
	req := authedJSONRequest(http.MethodPost, baseURL+"/league-bookmarks", token, body)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var ul provider.UserLeague
	if err := json.NewDecoder(resp.Body).Decode(&ul); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if ul.UserID != "real-user" {
		t.Errorf("expected bookmark saved under claims subject %q, got %q", "real-user", ul.UserID)
	}
}

// TestListUserLeagues_AuthedIgnoresQueryUserID verifies an authenticated attacker cannot
// harvest another signed-in user's bookmarks by passing their user_id in the query string.
func TestListUserLeagues_AuthedIgnoresQueryUserID(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())
	saveTestUserLeague(t, baseURL, "victim-id", "league-1", "sleeper", "Victim League")

	token := signTestJWT("attacker-id", "attacker@test.example", "attacker")
	resp, err := authedGet(token, baseURL+"/league-bookmarks?user_id=victim-id")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var leagues []provider.UserLeague
	if err := json.NewDecoder(resp.Body).Decode(&leagues); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(leagues) != 0 {
		t.Errorf("expected attacker to see 0 of victim's bookmarks, got %d", len(leagues))
	}
}

// TestUpdateUserLeague_AuthedCannotEditAnotherUsersBookmark verifies an authenticated
// attacker cannot relabel another signed-in user's bookmark by passing their user_id.
func TestUpdateUserLeague_AuthedCannotEditAnotherUsersBookmark(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())
	saveTestUserLeague(t, baseURL, "victim-id", "league-1", "sleeper", "Victim Label")

	token := signTestJWT("attacker-id", "attacker@test.example", "attacker")
	body := `{"user_id":"victim-id","label":"Hijacked"}`
	req := authedJSONRequest(http.MethodPatch, baseURL+"/league-bookmarks/league-1?source=sleeper", token, body)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 (attacker doesn't own this bookmark), got %d", resp.StatusCode)
	}

	listResp, err := http.Get(baseURL + "/league-bookmarks?user_id=victim-id")
	if err != nil {
		t.Fatalf("list request failed: %v", err)
	}
	defer listResp.Body.Close()
	var leagues []provider.UserLeague
	if err := json.NewDecoder(listResp.Body).Decode(&leagues); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(leagues) != 1 || leagues[0].Label != "Victim Label" {
		t.Errorf("expected victim's bookmark untouched, got %+v", leagues)
	}
}

// TestDeleteUserLeague_AuthedCannotDeleteAnotherUsersBookmark verifies an authenticated
// attacker cannot delete another signed-in user's bookmark by passing their user_id.
func TestDeleteUserLeague_AuthedCannotDeleteAnotherUsersBookmark(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())
	saveTestUserLeague(t, baseURL, "victim-id", "league-1", "sleeper", "Victim Bookmark")

	token := signTestJWT("attacker-id", "attacker@test.example", "attacker")
	req, _ := http.NewRequest(http.MethodDelete, baseURL+"/league-bookmarks/league-1?user_id=victim-id&source=sleeper", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 (attacker doesn't own this bookmark), got %d", resp.StatusCode)
	}

	listResp, err := http.Get(baseURL + "/league-bookmarks?user_id=victim-id")
	if err != nil {
		t.Fatalf("list request failed: %v", err)
	}
	defer listResp.Body.Close()
	var leagues []provider.UserLeague
	if err := json.NewDecoder(listResp.Body).Decode(&leagues); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(leagues) != 1 {
		t.Errorf("expected victim's bookmark to survive attacker's delete attempt, got %d", len(leagues))
	}
}

func TestGetPlayers(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())

	resp, err := http.Get(baseURL + "/players")
	if err != nil {
		t.Fatalf("GET /players: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var players []provider.SlimPlayer
	if err := json.NewDecoder(resp.Body).Decode(&players); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(players) != len(testPlayers) {
		t.Errorf("expected %d players, got %d", len(testPlayers), len(players))
	}

	for _, p := range players {
		if p.PlayerID == "" {
			t.Error("player has empty player_id")
		}
		if len(p.FantasyPositions) == 0 {
			t.Errorf("player %s has no fantasy positions", p.PlayerID)
		}
		if p.ImageURL == "" {
			t.Errorf("player %s has empty image_url", p.PlayerID)
		}
	}
}

func TestUnmatchedRoute_Returns404JSON(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())

	resp, err := http.Get(baseURL + "/some/unmatched/path")
	if err != nil {
		t.Fatalf("GET /some/unmatched/path: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}

	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json content type, got %q", ct)
	}
}

func TestResponseWriter_DefaultsTo200(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := newResponseWriter(rec)
	rw.Write([]byte("ok"))
	if rw.status != http.StatusOK {
		t.Errorf("want 200, got %d", rw.status)
	}
}

func TestResponseWriter_CapturesStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := newResponseWriter(rec)
	rw.WriteHeader(http.StatusNotFound)
	if rw.status != http.StatusNotFound {
		t.Errorf("want 404, got %d", rw.status)
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("underlying writer not called: want 404, got %d", rec.Code)
	}
}

// TestRateLimit_PublicRoute_ExceedingBurst_Returns429 hammers an unauthenticated,
// Sleeper-proxying route from a single client (GH #13) — without a per-IP limiter this
// traffic passes straight through to Sleeper on every request. Requests within the burst
// should succeed; once the bucket is exhausted, further requests should be rejected with
// 429 rather than forwarded.
func TestRateLimit_PublicRoute_ExceedingBurst_Returns429(t *testing.T) {
	baseURL := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/league/abc/rosters":
			json.NewEncoder(w).Encode([]map[string]any{})
		case "/league/abc/users":
			json.NewEncoder(w).Encode([]map[string]any{})
		}
	}))

	var sawTooManyRequests bool
	var okCount int
	// rateLimitBurst+extra requests fired back-to-back from the same source IP; the token
	// bucket cannot refill meaningfully in this span, so at least one must be rejected.
	for i := 0; i < rateLimitBurst+10; i++ {
		resp, err := http.Get(baseURL + "/league/abc/rosters")
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		resp.Body.Close()
		switch resp.StatusCode {
		case http.StatusOK:
			okCount++
		case http.StatusTooManyRequests:
			sawTooManyRequests = true
		default:
			t.Fatalf("request %d: unexpected status %d", i, resp.StatusCode)
		}
	}

	if !sawTooManyRequests {
		t.Fatal("expected at least one 429 once the per-IP burst was exceeded")
	}
	if okCount == 0 {
		t.Fatal("expected at least one request within the burst to succeed")
	}
	if okCount > rateLimitBurst {
		t.Errorf("expected at most %d requests to succeed within the burst, got %d", rateLimitBurst, okCount)
	}
}

// TestRateLimit_UnaffectedRoute_NotLimited confirms the limiter is scoped to the
// public/proxying routes it was wired onto, not applied globally — /healthz should never
// 429 no matter how many times it's hit.
func TestRateLimit_UnaffectedRoute_NotLimited(t *testing.T) {
	baseURL := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	for i := 0; i < rateLimitBurst+10; i++ {
		resp, err := http.Get(baseURL + "/healthz")
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, resp.StatusCode)
		}
	}
}

// TestCollect_BodyTooLarge_Returns413 posts a body well past the 64KiB cap decode()
// enforces (GH #13) to the one unauthenticated write endpoint in the API. Without a body
// cap this would be read into memory in full before any validation runs.
func TestCollect_BodyTooLarge_Returns413(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())

	oversizedPath := `"` + strings.Repeat("x", 100*1024) + `"`
	body := `{"visitor_id":"v1","path":` + oversizedPath + `}`

	resp, err := http.Post(baseURL+"/collect", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", resp.StatusCode)
	}
}

// TestCollect_NormalBody_Succeeds is the happy-path counterpart to the 413 test above —
// confirms wiring http.MaxBytesReader into decode() didn't break ordinary small requests.
func TestCollect_NormalBody_Succeeds(t *testing.T) {
	baseURL := newTestServer(t, noopHandler())

	resp, err := http.Post(baseURL+"/collect", "application/json", strings.NewReader(`{"visitor_id":"v1","path":"/home"}`))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}
