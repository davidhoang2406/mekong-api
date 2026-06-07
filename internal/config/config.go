package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port                string
	GinMode             string
	MinioEndpoint       string
	MinioAccessKey      string
	MinioSecretKey      string
	MinioAnalysisBucket string
	PostgresURL         string
	PostgresMaxConns    int
	CacheTTLSeconds     int
	SnapshotCacheURL    string
	WSInternalURL       string
	LogLevel            string
	JWTSecret           string
	JWTExpiryHours      int
	GoogleClientID      string
	GoogleClientSecret  string
	GitHubClientID      string
	GitHubClientSecret  string
	OAuthRedirectBase   string
}

func Load() (Config, error) {
	maxConns, err := strconv.Atoi(getEnv("POSTGRES_MAX_CONNS", "10"))
	if err != nil {
		return Config{}, fmt.Errorf("POSTGRES_MAX_CONNS must be an integer: %w", err)
	}
	cacheTTL, err := strconv.Atoi(getEnv("CACHE_TTL_SECONDS", "300"))
	if err != nil {
		return Config{}, fmt.Errorf("CACHE_TTL_SECONDS must be an integer: %w", err)
	}

	jwtExpiry, err := strconv.Atoi(getEnv("JWT_EXPIRY_HOURS", "720"))
	if err != nil {
		return Config{}, fmt.Errorf("JWT_EXPIRY_HOURS must be an integer: %w", err)
	}

	return Config{
		Port:                getEnv("PORT", "8090"),
		GinMode:             getEnv("GIN_MODE", "release"),
		MinioEndpoint:       getEnv("MINIO_ENDPOINT", "minio:9000"),
		MinioAccessKey:      getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinioSecretKey:      getEnv("MINIO_SECRET_KEY", "minioadmin"),
		MinioAnalysisBucket: getEnv("MINIO_ANALYSIS_BUCKET", "market-analysis"),
		PostgresURL:         getEnv("POSTGRES_URL", "postgres://mekong:mekong@postgres:5432/mekong_api?sslmode=disable"),
		PostgresMaxConns:    maxConns,
		CacheTTLSeconds:     cacheTTL,
		SnapshotCacheURL:    getEnv("SNAPSHOT_CACHE_URL", ""),
		WSInternalURL:       getEnv("WS_INTERNAL_URL", ""),
		LogLevel:            getEnv("LOG_LEVEL", "info"),
		JWTSecret:           getEnv("JWT_SECRET", "change-me-in-production"),
		JWTExpiryHours:      jwtExpiry,
		GoogleClientID:      getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret:  getEnv("GOOGLE_CLIENT_SECRET", ""),
		GitHubClientID:      getEnv("GITHUB_CLIENT_ID", ""),
		GitHubClientSecret:  getEnv("GITHUB_CLIENT_SECRET", ""),
		OAuthRedirectBase:   getEnv("OAUTH_REDIRECT_BASE", "http://localhost:8080"),
	}, nil
}

func (c Config) Validate() error {
	if c.MinioEndpoint == "" {
		return fmt.Errorf("MINIO_ENDPOINT is required")
	}
	if c.PostgresURL == "" {
		return fmt.Errorf("POSTGRES_URL is required")
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
