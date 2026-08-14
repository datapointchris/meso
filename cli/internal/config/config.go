// Package config resolves the meso CLI's OIDC and API settings from
// environment overrides layered over homelab defaults.
package config

import (
	"os"
	"strings"
)

const (
	defaultIssuer  = "https://auth.ichrisbirch.com"
	defaultAPIBase = "https://meso.ichrisbirch.com"
)

// Scopes requested at login. offline_access yields the refresh token; openid
// and profile make the access token a standard OIDC one. The bearer.authz
// scope is deliberately absent — Authelia permits only authorization_code,
// refresh_token and client_credentials alongside it, which rules out the
// device grant this CLI logs in with.
var Scopes = []string{"openid", "profile", "offline_access"}

type Config struct {
	Issuer   string
	ClientID string
	APIBase  string
}

// Load resolves settings. Precedence per CLI conventions: env var > default.
// A config file layer can slot in below env later without changing callers.
func Load() Config {
	return Config{
		Issuer:   getEnv("MESO_OIDC_ISSUER", defaultIssuer),
		ClientID: getEnv("MESO_CLIENT_ID", defaultClientID()),
		APIBase:  getEnv("MESO_API_BASE", defaultAPIBase),
	}
}

// defaultClientID derives the per-(machine × app) Authelia client_id from the
// short hostname: `macmini.trusted` → `meso-cli-macmini`, matching the clients
// registered in the Authelia template. Machines whose hostname differs from
// their logical name override with MESO_CLIENT_ID (pyinfra can template this).
func defaultClientID() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		return "meso-cli"
	}
	short := strings.ToLower(strings.SplitN(host, ".", 2)[0])
	return "meso-cli-" + short
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
