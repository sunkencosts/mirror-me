package config

import "strconv"

type Config struct {
	Port               string
	SleeperBaseURL     string
	RankingsCSVURL     string
	DatabaseURL        string
	MigrationsURL      string
	CurrentWeek        int
	CurrentSeason      string
	AppEnv             string
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	GoogleAuthURL      string
	GoogleTokenURL     string
	GoogleUserInfoURL  string
	FrontendURL        string
	JWTSecret          string
	AdminSecret        string
	LogFile            string
	// DevLogin* are the identity /dev/login mints by default. They mirror the seed's
	// SEED_PRIMARY_* env so dev-login "is" the same primary account seed-dev bookmarks and
	// gives lineups to. Default to the fixed dev_user when unset.
	DevLoginUserID   string
	DevLoginUsername string
	DevLoginEmail    string
}

func Load(getenv func(string) string) Config {
	port := getenv("PORT")
	if port == "" {
		port = "8080"
	}
	databaseURL := getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://mirrorleague:mirrorleague@localhost:5433/mirrorleague" //nolint:gosec
	}
	sleeperBaseURL := getenv("SLEEPER_BASE_URL")
	if sleeperBaseURL == "" {
		sleeperBaseURL = "https://api.sleeper.app/v1"
	}
	rankingsCSVURL := getenv("RANKINGS_CSV_URL")
	if rankingsCSVURL == "" {
		rankingsCSVURL = "https://raw.githubusercontent.com/dynastyprocess/data/master/files/db_fpecr_latest.csv"
	}
	migrationsURL := getenv("MIGRATIONS_URL")
	if migrationsURL == "" {
		migrationsURL = "file://migrations"
	}
	// CurrentWeek 0 means "infer from week_locks + the current date" (the normal case).
	// A non-zero CURRENT_WEEK is an explicit override — used by tests for determinism and
	// available as a manual safety valve.
	currentWeek := 0
	if s := getenv("CURRENT_WEEK"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			currentWeek = n
		}
	}
	currentSeason := getenv("CURRENT_SEASON")
	if currentSeason == "" {
		currentSeason = "2026"
	}
	googleAuthURL := getenv("GOOGLE_AUTH_URL")
	if googleAuthURL == "" {
		googleAuthURL = "https://accounts.google.com/o/oauth2/auth"
	}
	googleTokenURL := getenv("GOOGLE_TOKEN_URL")
	if googleTokenURL == "" {
		googleTokenURL = "https://oauth2.googleapis.com/token" //nolint:gosec
	}
	googleUserInfoURL := getenv("GOOGLE_USERINFO_URL")
	if googleUserInfoURL == "" {
		googleUserInfoURL = "https://www.googleapis.com/oauth2/v2/userinfo"
	}
	frontendURL := getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:5173"
	}
	devLoginUserID := getenv("SEED_PRIMARY_USER_ID")
	if devLoginUserID == "" {
		devLoginUserID = "00000000-0000-0000-0000-000000000001"
	}
	devLoginUsername := getenv("SEED_PRIMARY_USERNAME")
	if devLoginUsername == "" {
		devLoginUsername = "dev_user"
	}
	devLoginEmail := getenv("SEED_PRIMARY_EMAIL")
	if devLoginEmail == "" {
		devLoginEmail = "dev@localhost"
	}
	return Config{
		AppEnv:             getenv("APP_ENV"),
		Port:               port,
		SleeperBaseURL:     sleeperBaseURL,
		RankingsCSVURL:     rankingsCSVURL,
		DatabaseURL:        databaseURL,
		MigrationsURL:      migrationsURL,
		CurrentWeek:        currentWeek,
		CurrentSeason:      currentSeason,
		GoogleClientID:     getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  getenv("GOOGLE_REDIRECT_URL"),
		GoogleAuthURL:      googleAuthURL,
		GoogleTokenURL:     googleTokenURL,
		GoogleUserInfoURL:  googleUserInfoURL,
		FrontendURL:        frontendURL,
		JWTSecret:          getenv("JWT_SECRET"),
		AdminSecret:        getenv("ADMIN_SECRET"),
		LogFile:            getenv("LOG_FILE"),
		DevLoginUserID:     devLoginUserID,
		DevLoginUsername:   devLoginUsername,
		DevLoginEmail:      devLoginEmail,
	}
}
