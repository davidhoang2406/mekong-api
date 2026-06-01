package store

import (
	"database/sql"
	"fmt"

	"github.com/davidhoang2406/mekong-api/internal/config"
	"github.com/davidhoang2406/mekong-api/internal/model"
	_ "github.com/marcboeker/go-duckdb"
)

// ParquetBase is the URL prefix for all read_parquet paths.
// Default is "s3://" for production. Integration tests override to a local dir.
var ParquetBase = "s3://"

func InitDuckDB(cfg config.Config) (*sql.DB, error) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		return nil, fmt.Errorf("open duckdb: %w", err)
	}

	// DuckDB is single-writer; one connection shared across all handlers.
	db.SetMaxOpenConns(1)

	setup := fmt.Sprintf(`
		SET home_directory = '/tmp';
		INSTALL httpfs;
		LOAD httpfs;
		SET s3_region = 'us-east-1';
		SET s3_endpoint = '%s';
		SET s3_access_key_id = '%s';
		SET s3_secret_access_key = '%s';
		SET s3_use_ssl = false;
		SET s3_url_style = 'path';
	`, cfg.MinioEndpoint, cfg.MinioAccessKey, cfg.MinioSecretKey)

	if _, err := db.Exec(setup); err != nil {
		db.Close()
		return nil, fmt.Errorf("duckdb httpfs setup: %w", err)
	}

	return db, nil
}

func parquetPath(bucket, prefix string) string {
	return fmt.Sprintf("%s%s/%s/**/*.parquet", ParquetBase, bucket, prefix)
}

func QuerySymbols(db *sql.DB, bucket, assetClass string) ([]model.SymbolInfo, error) {
	path := parquetPath(bucket, "ohlcv.bar")
	query := fmt.Sprintf(`
		SELECT symbol, asset_class, exchange,
		       MIN(time)::TIMESTAMP::DATE::VARCHAR AS first_date,
		       MAX(time)::TIMESTAMP::DATE::VARCHAR AS last_date
		FROM read_parquet('%s', hive_partitioning=true)
		%s
		GROUP BY symbol, asset_class, exchange
		ORDER BY symbol
	`, path, assetClassFilter(assetClass, "WHERE"))

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query symbols: %w", err)
	}
	defer rows.Close()

	var symbols []model.SymbolInfo
	for rows.Next() {
		var s model.SymbolInfo
		if err := rows.Scan(&s.Symbol, &s.AssetClass, &s.Exchange, &s.FirstDate, &s.LastDate); err != nil {
			return nil, err
		}
		symbols = append(symbols, s)
	}
	return symbols, rows.Err()
}

func QueryOHLCV(db *sql.DB, bucket, symbol, from, to string) ([]model.OHLCVBar, error) {
	path := parquetPath(bucket, "ohlcv.bar")
	rows, err := db.Query(fmt.Sprintf(`
		SELECT time::VARCHAR,
		       open::DOUBLE, high::DOUBLE, low::DOUBLE, close::DOUBLE,
		       volume::BIGINT
		FROM read_parquet('%s', hive_partitioning=true)
		WHERE symbol = $1
		  AND (year || '-' || month || '-' || day) BETWEEN $2 AND $3
		ORDER BY time
	`, path), symbol, from, to)
	if err != nil {
		return nil, fmt.Errorf("query ohlcv: %w", err)
	}
	defer rows.Close()

	var bars []model.OHLCVBar
	for rows.Next() {
		var b model.OHLCVBar
		if err := rows.Scan(&b.Time, &b.Open, &b.High, &b.Low, &b.Close, &b.Volume); err != nil {
			return nil, err
		}
		bars = append(bars, b)
	}
	return bars, rows.Err()
}

func QueryOHLCVMeta(db *sql.DB, bucket, symbol string) (assetClass, exchange string, err error) {
	path := parquetPath(bucket, "ohlcv.bar")
	row := db.QueryRow(fmt.Sprintf(`
		SELECT asset_class, exchange
		FROM read_parquet('%s', hive_partitioning=true)
		WHERE symbol = $1
		LIMIT 1
	`, path), symbol)
	err = row.Scan(&assetClass, &exchange)
	return
}

func QueryIndicators(db *sql.DB, bucket, symbol, from, to string) ([]model.IndicatorRow, error) {
	path := parquetPath(bucket, "technical.indicators")
	rows, err := db.Query(fmt.Sprintf(`
		SELECT time::VARCHAR,
		       close::DOUBLE,
		       sma20::DOUBLE, sma50::DOUBLE, sma200::DOUBLE, rsi14::DOUBLE,
		       macd::DOUBLE, macd_signal::DOUBLE, macd_hist::DOUBLE,
		       bb_upper::DOUBLE, bb_mid::DOUBLE, bb_lower::DOUBLE
		FROM read_parquet('%s', hive_partitioning=true)
		WHERE symbol = $1
		  AND (year || '-' || month || '-' || day) BETWEEN $2 AND $3
		ORDER BY time
	`, path), symbol, from, to)
	if err != nil {
		return nil, fmt.Errorf("query indicators: %w", err)
	}
	defer rows.Close()

	var rows_ []model.IndicatorRow
	for rows.Next() {
		var r model.IndicatorRow
		if err := rows.Scan(
			&r.Time, &r.Close,
			&r.SMA20, &r.SMA50, &r.SMA200, &r.RSI14,
			&r.MACD, &r.MACDSignal, &r.MACDHist,
			&r.BBUpper, &r.BBMid, &r.BBLower,
		); err != nil {
			return nil, err
		}
		rows_ = append(rows_, r)
	}
	return rows_, rows.Err()
}

func QueryDigest(db *sql.DB, bucket, year, month, day, category string, limit int) ([]model.DigestEntry, error) {
	path := parquetPath(bucket, "digest")
	catFilter := ""
	if category != "" {
		catFilter = fmt.Sprintf("AND category = '%s'", category)
	}
	rows, err := db.Query(fmt.Sprintf(`
		SELECT category, rank::INTEGER, symbol, exchange, asset_class,
		       open::DOUBLE, close::DOUBLE, volume::BIGINT, pct_change::DOUBLE
		FROM read_parquet('%s', hive_partitioning=true)
		WHERE year = $1 AND month = $2 AND day = $3
		  %s
		  AND rank <= $4
		ORDER BY category, rank
	`, path, catFilter), year, month, day, limit)
	if err != nil {
		return nil, fmt.Errorf("query digest: %w", err)
	}
	defer rows.Close()

	var entries []model.DigestEntry
	for rows.Next() {
		var e model.DigestEntry
		if err := rows.Scan(
			&e.Category, &e.Rank, &e.Symbol, &e.Exchange, &e.AssetClass,
			&e.Open, &e.Close, &e.Volume, &e.PctChange,
		); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func QueryScreener(db *sql.DB, bucket, year, week string) ([]model.ScreenerResult, error) {
	path := parquetPath(bucket, "screener")
	rows, err := db.Query(fmt.Sprintf(`
		SELECT symbol,
		       pe_ratio::DOUBLE, pb_ratio::DOUBLE, roe::DOUBLE, eps::DOUBLE,
		       de_ratio::DOUBLE, current_ratio::DOUBLE
		FROM read_parquet('%s', hive_partitioning=true)
		WHERE year = $1 AND week = $2
		ORDER BY pe_ratio
	`, path), year, week)
	if err != nil {
		return nil, fmt.Errorf("query screener: %w", err)
	}
	defer rows.Close()

	var results []model.ScreenerResult
	for rows.Next() {
		var r model.ScreenerResult
		if err := rows.Scan(
			&r.Symbol, &r.PERatio, &r.PBRatio, &r.ROE, &r.EPS, &r.DERatio, &r.CurrentRatio,
		); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func assetClassFilter(assetClass, clause string) string {
	if assetClass == "" {
		return ""
	}
	return fmt.Sprintf("%s asset_class = '%s'", clause, assetClass)
}
