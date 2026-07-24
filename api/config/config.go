package config

import "os"

type Config struct {
	DatabaseURL string
	Port        string
	Env         string
}

func Load() *Config {
	return &Config{
		DatabaseURL: getEnv("DATABASE_URL", "postgres://meso:meso@localhost:5459/meso?sslmode=disable"),
		Port:        getEnv("PORT", "8088"),
		Env:         getEnv("ENV", "development"),
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
