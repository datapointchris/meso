package config

import "os"

type Config struct {
	DatabaseURL string
	Port        string
	Env         string
	// OIDCIssuer is the identity provider whose JWKS signs the CLI's access
	// tokens. CLIClientIDPrefix is the per-product half of the client naming
	// (`meso-cli-macmini`), and is what stops another product's token being
	// accepted here.
	OIDCIssuer        string
	CLIClientIDPrefix string
	// PublicURL is the address clients reach this API at, which its RFC 9728
	// metadata names as the resource. A CLI logging in through that document
	// refuses it unless the two match exactly. Empty serves no metadata, since
	// only the deployment knows the address it is reached at.
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
