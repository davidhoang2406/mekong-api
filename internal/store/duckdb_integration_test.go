//go:build integration

package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/davidhoang2406/mekong-api/internal/config"
)

// TestMain creates local Parquet fixtures once for all integration tests.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "mekong-test-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	ParquetBase = dir + "/"

	if err := writeFixtures(dir); err != nil {
		panic(fmt.Sprintf("write fixtures: %v", err))
	}

	os.Exit(m.Run())
}

// writeFixtures uses DuckDB to create Parquet test data with Hive partitioning.
func writeFixtures(base string) error {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		return err
	}
	defer db.Close()

	// ── OHLCV ────────────────────────────────────────────────────────────────
	ohlcvDir := filepath.Join(base, "market-analysis", "ohlcv.bar",
		"asset_class=stock", "year=2026", "month=05", "day=20")
	if err := os.MkdirAll(ohlcvDir, 0755); err != nil {
		return err
	}
	_, err = db.Exec(fmt.Sprintf(`
		COPY (
			SELECT
				'2026-05-20 00:00:00'::TIMESTAMPTZ AS time,
				'VCB'   AS symbol,
				'HOSE'  AS exchange,
				'stock' AS asset_class,
				85000.0 AS open,
				86200.0 AS high,
				84500.0 AS low,
				85800.0 AS close,
				2345678 AS volume
		) TO '%s/data.parquet' (FORMAT parquet)
	`, ohlcvDir))
	if err != nil {
		return fmt.Errorf("ohlcv fixture: %w", err)
	}

	// ── Indicators ───────────────────────────────────────────────────────────
	indDir := filepath.Join(base, "market-analysis", "technical.indicators",
		"asset_class=stock", "year=2026", "month=05", "day=20")
	if err := os.MkdirAll(indDir, 0755); err != nil {
		return err
	}
	_, err = db.Exec(fmt.Sprintf(`
		COPY (
			SELECT
				'2026-05-20 00:00:00'::TIMESTAMPTZ AS time,
				'VCB'   AS symbol,
				'stock' AS asset_class,
				'HOSE'  AS exchange,
				85800.0 AS close,
				85100.0 AS sma20,
				84200.0 AS sma50,
				82500.0 AS sma200,
				58.3    AS rsi14,
				120.5   AS macd,
				95.2    AS macd_signal,
				25.3    AS macd_hist,
				87200.0 AS bb_upper,
				85100.0 AS bb_mid,
				83000.0 AS bb_lower
		) TO '%s/data.parquet' (FORMAT parquet)
	`, indDir))
	if err != nil {
		return fmt.Errorf("indicators fixture: %w", err)
	}

	// ── Digest ───────────────────────────────────────────────────────────────
	digestDir := filepath.Join(base, "market-analysis", "digest",
		"year=2026", "month=05", "day=20")
	if err := os.MkdirAll(digestDir, 0755); err != nil {
		return err
	}
	_, err = db.Exec(fmt.Sprintf(`
		COPY (
			SELECT
				'gainer' AS category,
				1        AS rank,
				'FPT'    AS symbol,
				'HOSE'   AS exchange,
				'stock'  AS asset_class,
				120000.0 AS open,
				128000.0 AS close,
				5678901  AS volume,
				6.67     AS pct_change
		) TO '%s/data.parquet' (FORMAT parquet)
	`, digestDir))
	if err != nil {
		return fmt.Errorf("digest fixture: %w", err)
	}

	// ── Screener ─────────────────────────────────────────────────────────────
	screenerDir := filepath.Join(base, "market-analysis", "screener",
		"year=2026", "week=21")
	if err := os.MkdirAll(screenerDir, 0755); err != nil {
		return err
	}
	_, err = db.Exec(fmt.Sprintf(`
		COPY (
			SELECT
				'VCB' AS symbol,
				14.2  AS pe_ratio,
				2.1   AS pb_ratio,
				22.5  AS roe,
				6200.0 AS eps,
				0.8   AS de_ratio,
				1.4   AS current_ratio
		) TO '%s/data.parquet' (FORMAT parquet)
	`, screenerDir))
	if err != nil {
		return fmt.Errorf("screener fixture: %w", err)
	}

	return nil
}

func openTestDuckDB(t *testing.T) *sql.DB {
	t.Helper()
	cfg := config.Config{
		MinioEndpoint:  "unused-in-integration",
		MinioAccessKey: "x",
		MinioSecretKey: "x",
	}
	// Don't load httpfs/S3 in integration tests — reading local files directly.
	db, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open duckdb: %v", err)
	}
	_ = cfg
	t.Cleanup(func() { db.Close() })
	return db
}

const testBucket = "market-analysis"

func TestQueryOHLCV_Integration(t *testing.T) {
	db := openTestDuckDB(t)
	bars, err := QueryOHLCV(db, testBucket, "VCB", "2026-05-20", "2026-05-20")
	if err != nil {
		t.Fatalf("QueryOHLCV: %v", err)
	}
	if len(bars) != 1 {
		t.Fatalf("expected 1 bar, got %d", len(bars))
	}
	b := bars[0]
	if b.Open != 85000.0 {
		t.Errorf("Open = %v, want 85000", b.Open)
	}
	if b.Close != 85800.0 {
		t.Errorf("Close = %v, want 85800", b.Close)
	}
	if b.Volume != 2345678 {
		t.Errorf("Volume = %v, want 2345678", b.Volume)
	}
}

func TestQueryOHLCV_NoResults(t *testing.T) {
	db := openTestDuckDB(t)
	bars, err := QueryOHLCV(db, testBucket, "NONEXISTENT", "2026-05-20", "2026-05-20")
	if err != nil {
		t.Fatalf("QueryOHLCV: %v", err)
	}
	if len(bars) != 0 {
		t.Errorf("expected 0 bars, got %d", len(bars))
	}
}

func TestQuerySymbols_Integration(t *testing.T) {
	db := openTestDuckDB(t)
	symbols, err := QuerySymbols(db, testBucket, "")
	if err != nil {
		t.Fatalf("QuerySymbols: %v", err)
	}
	if len(symbols) == 0 {
		t.Fatal("expected symbols, got none")
	}
	s := symbols[0]
	if s.Symbol != "VCB" {
		t.Errorf("Symbol = %q", s.Symbol)
	}
	if s.AssetClass != "stock" {
		t.Errorf("AssetClass = %q", s.AssetClass)
	}
}

func TestQuerySymbols_FilterByAssetClass(t *testing.T) {
	db := openTestDuckDB(t)
	stocks, err := QuerySymbols(db, testBucket, "stock")
	if err != nil {
		t.Fatalf("QuerySymbols stock: %v", err)
	}
	cryptos, err := QuerySymbols(db, testBucket, "crypto")
	if err != nil {
		t.Fatalf("QuerySymbols crypto: %v", err)
	}
	if len(stocks) == 0 {
		t.Error("expected stocks")
	}
	if len(cryptos) != 0 {
		t.Error("expected no cryptos in fixture")
	}
}

func TestQueryIndicators_Integration(t *testing.T) {
	db := openTestDuckDB(t)
	rows, err := QueryIndicators(db, testBucket, "VCB", "2026-05-20", "2026-05-20")
	if err != nil {
		t.Fatalf("QueryIndicators: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	r := rows[0]
	if r.RSI14 == nil || *r.RSI14 != 58.3 {
		t.Errorf("RSI14 = %v", r.RSI14)
	}
	if r.SMA20 == nil || *r.SMA20 != 85100.0 {
		t.Errorf("SMA20 = %v", r.SMA20)
	}
}

func TestQueryDigest_Integration(t *testing.T) {
	db := openTestDuckDB(t)
	entries, err := QueryDigest(db, testBucket, "2026", "05", "20", "", 10)
	if err != nil {
		t.Fatalf("QueryDigest: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Symbol != "FPT" {
		t.Errorf("Symbol = %q", e.Symbol)
	}
	if e.Category != "gainer" {
		t.Errorf("Category = %q", e.Category)
	}
	if e.PctChange != 6.67 {
		t.Errorf("PctChange = %v", e.PctChange)
	}
}

func TestQueryDigest_CategoryFilter(t *testing.T) {
	db := openTestDuckDB(t)
	gainers, err := QueryDigest(db, testBucket, "2026", "05", "20", "gainer", 10)
	if err != nil {
		t.Fatalf("QueryDigest gainer: %v", err)
	}
	losers, err := QueryDigest(db, testBucket, "2026", "05", "20", "loser", 10)
	if err != nil {
		t.Fatalf("QueryDigest loser: %v", err)
	}
	if len(gainers) != 1 {
		t.Errorf("expected 1 gainer, got %d", len(gainers))
	}
	if len(losers) != 0 {
		t.Errorf("expected 0 losers, got %d", len(losers))
	}
}

func TestQueryScreener_Integration(t *testing.T) {
	db := openTestDuckDB(t)
	results, err := QueryScreener(db, testBucket, "2026", "21")
	if err != nil {
		t.Fatalf("QueryScreener: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Symbol != "VCB" {
		t.Errorf("Symbol = %q", r.Symbol)
	}
	if r.PERatio == nil || *r.PERatio != 14.2 {
		t.Errorf("PERatio = %v", r.PERatio)
	}
}

func TestQueryScreener_NoData(t *testing.T) {
	db := openTestDuckDB(t)
	results, err := QueryScreener(db, testBucket, "2025", "01")
	if err != nil {
		t.Fatalf("QueryScreener: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}
