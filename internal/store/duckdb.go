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

// queryAll runs query with args and scans every row into a []T via scan.
// It centralises the rows/defer/Next/Err boilerplate shared by all queries.
func queryAll[T any](db *sql.DB, query string, scan func(*sql.Rows, *T) error, args ...any) ([]T, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []T
	for rows.Next() {
		var item T
		if err := scan(rows, &item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func QuerySymbols(db *sql.DB, bucket, assetClass string) ([]model.SymbolInfo, error) {
	path := parquetPath(bucket, "ohlcv.bar")

	where := ""
	var args []any
	if assetClass != "" {
		where = "WHERE asset_class = $1"
		args = append(args, assetClass)
	}

	query := fmt.Sprintf(`
		SELECT symbol, asset_class, exchange,
		       MIN(time)::TIMESTAMP::DATE::VARCHAR AS first_date,
		       MAX(time)::TIMESTAMP::DATE::VARCHAR AS last_date
		FROM read_parquet('%s', hive_partitioning=true)
		%s
		GROUP BY symbol, asset_class, exchange
		ORDER BY symbol
	`, path, where)

	out, err := queryAll(db, query, func(r *sql.Rows, s *model.SymbolInfo) error {
		return r.Scan(&s.Symbol, &s.AssetClass, &s.Exchange, &s.FirstDate, &s.LastDate)
	}, args...)
	if err != nil {
		return nil, fmt.Errorf("query symbols: %w", err)
	}
	return out, nil
}

func QueryOHLCV(db *sql.DB, bucket, symbol, from, to string) ([]model.OHLCVBar, error) {
	path := parquetPath(bucket, "ohlcv.bar")
	query := fmt.Sprintf(`
		SELECT time::VARCHAR,
		       open::DOUBLE, high::DOUBLE, low::DOUBLE, close::DOUBLE,
		       volume::BIGINT
		FROM read_parquet('%s', hive_partitioning=true)
		WHERE symbol = $1
		  AND (year || '-' || month || '-' || day) BETWEEN $2 AND $3
		ORDER BY time
	`, path)

	out, err := queryAll(db, query, func(r *sql.Rows, b *model.OHLCVBar) error {
		return r.Scan(&b.Time, &b.Open, &b.High, &b.Low, &b.Close, &b.Volume)
	}, symbol, from, to)
	if err != nil {
		return nil, fmt.Errorf("query ohlcv: %w", err)
	}
	return out, nil
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
	query := fmt.Sprintf(`
		SELECT time::VARCHAR,
		       close::DOUBLE,
		       sma20::DOUBLE, sma50::DOUBLE, sma200::DOUBLE, rsi14::DOUBLE,
		       macd::DOUBLE, macd_signal::DOUBLE, macd_hist::DOUBLE,
		       bb_upper::DOUBLE, bb_mid::DOUBLE, bb_lower::DOUBLE
		FROM read_parquet('%s', hive_partitioning=true)
		WHERE symbol = $1
		  AND (year || '-' || month || '-' || day) BETWEEN $2 AND $3
		ORDER BY time
	`, path)

	out, err := queryAll(db, query, func(r *sql.Rows, i *model.IndicatorRow) error {
		return r.Scan(
			&i.Time, &i.Close,
			&i.SMA20, &i.SMA50, &i.SMA200, &i.RSI14,
			&i.MACD, &i.MACDSignal, &i.MACDHist,
			&i.BBUpper, &i.BBMid, &i.BBLower,
		)
	}, symbol, from, to)
	if err != nil {
		return nil, fmt.Errorf("query indicators: %w", err)
	}
	return out, nil
}

func QueryDigest(db *sql.DB, bucket, year, month, day, category string, limit int) ([]model.DigestEntry, error) {
	path := parquetPath(bucket, "digest")

	where := "WHERE year = $1 AND month = $2 AND day = $3 AND rank <= $4"
	args := []any{year, month, day, limit}
	if category != "" {
		where += " AND category = $5"
		args = append(args, category)
	}

	query := fmt.Sprintf(`
		SELECT category, rank::INTEGER, symbol, exchange, asset_class,
		       open::DOUBLE, close::DOUBLE, volume::BIGINT, pct_change::DOUBLE
		FROM read_parquet('%s', hive_partitioning=true)
		%s
		ORDER BY category, rank
	`, path, where)

	out, err := queryAll(db, query, func(r *sql.Rows, e *model.DigestEntry) error {
		return r.Scan(
			&e.Category, &e.Rank, &e.Symbol, &e.Exchange, &e.AssetClass,
			&e.Open, &e.Close, &e.Volume, &e.PctChange,
		)
	}, args...)
	if err != nil {
		return nil, fmt.Errorf("query digest: %w", err)
	}
	return out, nil
}

func QueryScreener(db *sql.DB, bucket, year, week string) ([]model.ScreenerResult, error) {
	path := parquetPath(bucket, "screener")
	query := fmt.Sprintf(`
		SELECT symbol,
		       pe_ratio::DOUBLE, pb_ratio::DOUBLE, roe::DOUBLE, eps::DOUBLE,
		       de_ratio::DOUBLE, current_ratio::DOUBLE
		FROM read_parquet('%s', hive_partitioning=true)
		WHERE year = $1 AND week = $2
		ORDER BY pe_ratio
	`, path)

	out, err := queryAll(db, query, func(r *sql.Rows, s *model.ScreenerResult) error {
		return r.Scan(
			&s.Symbol, &s.PERatio, &s.PBRatio, &s.ROE, &s.EPS, &s.DERatio, &s.CurrentRatio,
		)
	}, year, week)
	if err != nil {
		return nil, fmt.Errorf("query screener: %w", err)
	}
	return out, nil
}
