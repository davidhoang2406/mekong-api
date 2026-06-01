package config

import (
	"os"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	unsetAll(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != "8090" {
		t.Errorf("Port = %q, want 8090", cfg.Port)
	}
	if cfg.GinMode != "release" {
		t.Errorf("GinMode = %q, want release", cfg.GinMode)
	}
	if cfg.MinioEndpoint != "minio:9000" {
		t.Errorf("MinioEndpoint = %q", cfg.MinioEndpoint)
	}
	if cfg.PostgresMaxConns != 10 {
		t.Errorf("PostgresMaxConns = %d, want 10", cfg.PostgresMaxConns)
	}
	if cfg.CacheTTLSeconds != 300 {
		t.Errorf("CacheTTLSeconds = %d, want 300", cfg.CacheTTLSeconds)
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	unsetAll(t)
	t.Setenv("PORT", "9999")
	t.Setenv("GIN_MODE", "debug")
	t.Setenv("MINIO_ENDPOINT", "localhost:9000")
	t.Setenv("POSTGRES_MAX_CONNS", "25")
	t.Setenv("CACHE_TTL_SECONDS", "60")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != "9999" {
		t.Errorf("Port = %q", cfg.Port)
	}
	if cfg.GinMode != "debug" {
		t.Errorf("GinMode = %q", cfg.GinMode)
	}
	if cfg.MinioEndpoint != "localhost:9000" {
		t.Errorf("MinioEndpoint = %q", cfg.MinioEndpoint)
	}
	if cfg.PostgresMaxConns != 25 {
		t.Errorf("PostgresMaxConns = %d", cfg.PostgresMaxConns)
	}
	if cfg.CacheTTLSeconds != 60 {
		t.Errorf("CacheTTLSeconds = %d", cfg.CacheTTLSeconds)
	}
}

func TestLoad_InvalidMaxConns(t *testing.T) {
	t.Setenv("POSTGRES_MAX_CONNS", "not-a-number")
	t.Cleanup(func() { os.Unsetenv("POSTGRES_MAX_CONNS") })
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid POSTGRES_MAX_CONNS")
	}
}

func TestLoad_InvalidCacheTTL(t *testing.T) {
	t.Setenv("CACHE_TTL_SECONDS", "abc")
	t.Cleanup(func() { os.Unsetenv("CACHE_TTL_SECONDS") })
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid CACHE_TTL_SECONDS")
	}
}

func TestValidate_OK(t *testing.T) {
	cfg := Config{MinioEndpoint: "minio:9000", PostgresURL: "postgres://x"}
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_MissingMinio(t *testing.T) {
	cfg := Config{PostgresURL: "postgres://x"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for missing MinioEndpoint")
	}
}

func TestValidate_MissingPostgres(t *testing.T) {
	cfg := Config{MinioEndpoint: "minio:9000"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for missing PostgresURL")
	}
}

// unsetAll clears env vars that Load() reads, so defaults apply cleanly.
func unsetAll(t *testing.T) {
	t.Helper()
	vars := []string{
		"PORT", "GIN_MODE", "MINIO_ENDPOINT", "MINIO_ACCESS_KEY",
		"MINIO_SECRET_KEY", "MINIO_ANALYSIS_BUCKET", "POSTGRES_URL",
		"POSTGRES_MAX_CONNS", "CACHE_TTL_SECONDS", "SNAPSHOT_CACHE_URL",
		"WS_INTERNAL_URL", "LOG_LEVEL",
	}
	for _, v := range vars {
		os.Unsetenv(v)
		t.Cleanup(func() { os.Unsetenv(v) })
	}
}
