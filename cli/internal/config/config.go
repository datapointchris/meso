// Package config resolves the meso CLI's OIDC and API settings from
// environment overrides layered over homelab defaults.
package config

import (
	"os"
	"strings"
)

const (
	defaultIssuer   = "https://auth.ichrisbirch.com"
	defaultAudience = "https://meso.ichrisbirch.com"
	defaultAPIBase  = "https://meso.ichrisbirch.com"
)

// LoopbackPorts is the fixed set registered as redirect_uris on the Authelia
// meso-cli clients (RFC 8252 §7.3 avoidance — Authelia matches exact URIs, so
// we register a small set and bind the first free one). Distinct from nomad's
// 8250-8252 so a meso login never contends with a nomad login for a port.
var LoopbackPorts = []int{8260, 8261, 8262}

// RedirectHost is the hostname used in the loopback redirect_uri, matching a
// registered redirect_uri on the Authelia clients (both `localhost` and
// `127.0.0.1` spellings are registered). Safari shows an https→http
// insecure-form warning on the form_post callback regardless of spelling
// (verified 2026-07-23 — `localhost` did not suppress it), so we use the IP
// literal. The warning is accepted; the 90-day refresh token (Authelia `cli`
// lifespan) makes an interactive login a ~quarterly event, so it is rare.
const RedirectHost = "127.0.0.1"

// Scopes requested at login. authelia.bearer.authz is Authelia's native bearer
// authorization scope — the resulting access token is opaque and authorized at
// the Traefik ForwardAuth edge by audience. offline_access yields a refresh
// token. No openid/profile: there is no id_token and identity is not read from
// the token (it is edge-authorized), so those scopes would be dead weight.
var Scopes = []string{"authelia.bearer.authz", "offline_access"}

type Config struct {
	Issuer   string
	ClientID string
	Audience string
	APIBase  string
}

// Load resolves settings. Precedence per CLI conventions: env var > default.
// A config file layer can slot in below env later without changing callers.
func Load() Config {
	return Config{
		Issuer:   getEnv("MESO_OIDC_ISSUER", defaultIssuer),
		ClientID: getEnv("MESO_CLIENT_ID", defaultClientID()),
		Audience: getEnv("MESO_OIDC_AUDIENCE", defaultAudience),
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
