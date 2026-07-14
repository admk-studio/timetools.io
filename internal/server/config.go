package server

import (
	"os"
	"strconv"
)

// Config is everything tunable, filled from TT_* environment variables.
// Defaults are what the public timetools.io instance runs with.
type Config struct {
	Addr       string // listen address
	BaseURL    string // hostname shown in examples and hints
	RateRPM    int    // sustained requests per minute per client
	RateBurst  int    // extra requests allowed in a burst
	TrustProxy bool   // read the client IP from X-Forwarded-For
	RepoURL    string // "source" link, shown in help and page footers
	LinkText   string // optional footer link on HTML pages...
	LinkURL    string // ...for whoever runs the instance
	Version    string
}

// FromEnv reads configuration from the environment. Every value has a
// sane default; an empty environment gives a working server.
func FromEnv() Config {
	return Config{
		Addr:       envStr("TT_ADDR", ":8080"),
		BaseURL:    envStr("TT_BASE_URL", "timetools.io"),
		RateRPM:    envInt("TT_RATE_RPM", 120),
		RateBurst:  envInt("TT_RATE_BURST", 30),
		TrustProxy: envBool("TT_TRUST_PROXY", false),
		RepoURL:    envStr("TT_REPO_URL", "github.com/admk-studio/timetools.io"),
		LinkText:   envStr("TT_LINK_TEXT", ""),
		LinkURL:    envStr("TT_LINK_URL", ""),
	}
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}
