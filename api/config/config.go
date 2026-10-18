package config

import "os"

type Config struct {
	DatabaseURL string
	Port        string
	Env         string
	// OIDCIssuer is the identity provider. It signs the CLI's access tokens,
	// and it logs the SPA's browsers in.
	OIDCIssuer string
	// CLIClientIDPrefix is the per-product half of the CLI client naming
	// (`meso-cli-laptop`), and is what stops another product's token being
	// accepted here.
	CLIClientIDPrefix string
	// WebClientID and WebClientSecret are the confidential client this API logs
	// the SPA's browsers in as. Production refuses to start without the secret.
	WebClientID     string
	WebClientSecret string
	// PublicURL is the address clients reach this API at. Its RFC 9728 metadata
	// names it as the resource, and a CLI logging in through that document
	// refuses it unless the two match exactly. The browser login's callback is
	// under it too. Empty serves no metadata, since only the deployment knows
	// the address it is reached at, and production refuses to start without it.
	PublicURL string
	// ServiceToken is the shared secret for internal service-to-service calls
	// on the docker network that bypass the edge. Empty disables that path.
	ServiceToken string
}

func Load() *Config {
	return &Config{
		DatabaseURL:       getEnv("DATABASE_URL", "postgres://meso:meso@localhost:5459/meso?sslmode=disable"),
		Port:              getEnv("PORT", "8088"),
		Env:               getEnv("ENV", "development"),
		OIDCIssuer:        getEnv("OIDC_ISSUER", "https://auth.ichrisbirch.com"),
		CLIClientIDPrefix: getEnv("CLI_CLIENT_ID_PREFIX", "meso-cli-"),
		WebClientID:       getEnv("WEB_CLIENT_ID", "meso-web"),
		WebClientSecret:   getEnv("WEB_CLIENT_SECRET", ""),
		PublicURL:         getEnv("PUBLIC_URL", ""),
		ServiceToken:      getEnv("MESO_SERVICE_TOKEN", ""),
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
