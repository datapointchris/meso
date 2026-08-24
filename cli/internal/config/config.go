// Package config resolves the meso CLI's OIDC and API settings from
// environment overrides layered over homelab defaults.
package config

import (
	"os"

	"github.com/datapointchris/goclilogin"
)

const (
	defaultIssuer  = "https://auth.ichrisbirch.com"
	defaultAPIBase = "https://meso.ichrisbirch.com"

	// keyringService namespaces meso's entries in the OS keychain. It is
	// a deployed identifier rather than a path-derived one, so it keeps its
	// spelling independently of the module or binary name.
	keyringService = "meso-cli"
)

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
	return goclilogin.ClientID("meso")
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// Login is the goclilogin view of this config: which provider to authenticate
// against, as which client, and where its state lives. StateDir holds the
// refresh lock and the fallback token file, and naming the tool's own directory
// keeps both beside its other state rather than under the keyring service name,
// which is where goclilogin would put them by default.
func (c Config) Login() goclilogin.Config {
	return goclilogin.Config{
		Issuer:         c.Issuer,
		ClientID:       c.ClientID,
		KeyringService: keyringService,
		StateDir:       goclilogin.StateDir("meso"),
	}
}
