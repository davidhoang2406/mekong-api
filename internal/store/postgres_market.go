package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/davidhoang2406/mekong-api/internal/model"
)

func QuerySymbolsPG(ctx context.Context, pool *pgxpool.Pool, assetClass string) ([]model.SymbolInfo, error) {
	rows, err := pool.Query(ctx, `
		SELECT symbol, asset_class, exchange,
		       to_char(first_date, 'YYYY-MM-DD'),
		       to_char(last_date,  'YYYY-MM-DD')
		FROM symbols
		WHERE ($1 = '' OR asset_class = $1)
		ORDER BY symbol
	`, assetClass)
	if err != nil {
		return nil, fmt.Errorf("query symbols pg: %w", err)
	}
	defer rows.Close()

	var out []model.SymbolInfo
	for rows.Next() {
		var s model.SymbolInfo
		if err := rows.Scan(&s.Symbol, &s.AssetClass, &s.Exchange, &s.FirstDate, &s.LastDate); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// QueryOHLCVPG returns bars plus asset_class and exchange for the symbol.
func QueryOHLCVPG(ctx context.Context, pool *pgxpool.Pool, symbol, from, to string) (
	bars []model.OHLCVBar, assetClass, exchange string, err error,
) {
	rows, err := pool.Query(ctx, `
		SELECT to_char(time, 'YYYY-MM-DD'), open, high, low, close, volume,
		       asset_class, exchange
		FROM ohlcv_bars
		WHERE symbol = $1 AND time BETWEEN $2::date AND $3::date
		ORDER BY time ASC
	`, symbol, from, to)
	if err != nil {
		return nil, "", "", fmt.Errorf("query ohlcv pg: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var b model.OHLCVBar
		if err := rows.Scan(&b.Time, &b.Open, &b.High, &b.Low, &b.Close, &b.Volume,
			&assetClass, &exchange); err != nil {
			return nil, "", "", err
		}
		bars = append(bars, b)
	}
	return bars, assetClass, exchange, rows.Err()
}

func QueryIndicatorsPG(ctx context.Context, pool *pgxpool.Pool, symbol, from, to string) ([]model.IndicatorRow, error) {
	rows, err := pool.Query(ctx, `
		SELECT to_char(time, 'YYYY-MM-DD'), close,
		       sma20, sma50, sma200, rsi14,
		       macd, macd_signal, macd_hist,
		       bb_upper, bb_mid, bb_lower
		FROM technical_indicators
		WHERE symbol = $1 AND time BETWEEN $2::date AND $3::date
		ORDER BY time ASC
	`, symbol, from, to)
	if err != nil {
		return nil, fmt.Errorf("query indicators pg: %w", err)
	}
	defer rows.Close()

	var out []model.IndicatorRow
	for rows.Next() {
		var i model.IndicatorRow
		if err := rows.Scan(
			&i.Time, &i.Close,
			&i.SMA20, &i.SMA50, &i.SMA200, &i.RSI14,
			&i.MACD, &i.MACDSignal, &i.MACDHist,
			&i.BBUpper, &i.BBMid, &i.BBLower,
		); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func QueryDigestPG(ctx context.Context, pool *pgxpool.Pool, date, category string, limit int) ([]model.DigestEntry, error) {
	rows, err := pool.Query(ctx, `
		SELECT category, rank, symbol, exchange, asset_class,
		       open, close, volume, pct_change
		FROM digest_entries
		WHERE date = $1::date AND ($2 = '' OR category = $2)
		ORDER BY category, rank
		LIMIT $3
	`, date, category, limit)
	if err != nil {
		return nil, fmt.Errorf("query digest pg: %w", err)
	}
	defer rows.Close()

	var out []model.DigestEntry
	for rows.Next() {
		var e model.DigestEntry
		if err := rows.Scan(
			&e.Category, &e.Rank, &e.Symbol, &e.Exchange, &e.AssetClass,
			&e.Open, &e.Close, &e.Volume, &e.PctChange,
		); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func QueryScreenerPG(ctx context.Context, pool *pgxpool.Pool, year, week string) ([]model.ScreenerResult, error) {
	rows, err := pool.Query(ctx, `
		SELECT symbol, pe_ratio, pb_ratio, roe, eps, de_ratio, current_ratio
		FROM screener_results
		WHERE year = $1 AND week = $2
		ORDER BY pe_ratio NULLS LAST
	`, year, week)
	if err != nil {
		return nil, fmt.Errorf("query screener pg: %w", err)
	}
	defer rows.Close()

	var out []model.ScreenerResult
	for rows.Next() {
		var s model.ScreenerResult
		if err := rows.Scan(
			&s.Symbol, &s.PERatio, &s.PBRatio, &s.ROE, &s.EPS, &s.DERatio, &s.CurrentRatio,
		); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
