package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/davidhoang2406/mekong-api/internal/model"
	"github.com/davidhoang2406/mekong-api/internal/store"
)

var startTime = time.Now()

// snapshotClient bounds the upstream snapshot call so a hung service can't
// block the handler goroutine indefinitely.
var snapshotClient = &http.Client{Timeout: 5 * time.Second}

// symbolAndRange reads the symbol (required) plus the from/to date range
// (defaulting to the last 30 days). On a missing symbol it writes the error
// response and returns ok=false.
func symbolAndRange(c *gin.Context) (symbol, from, to string, ok bool) {
	symbol = c.Query("symbol")
	if symbol == "" {
		abortError(c, http.StatusBadRequest, "symbol is required", "MISSING_PARAM")
		return "", "", "", false
	}
	now := time.Now()
	from = c.DefaultQuery("from", now.AddDate(0, 0, -30).Format("2006-01-02"))
	to = c.DefaultQuery("to", now.Format("2006-01-02"))
	return symbol, from, to, true
}

func (a *App) Health(c *gin.Context) {
	duckStatus := "connected"
	if err := a.DuckDB.PingContext(context.Background()); err != nil {
		duckStatus = "error: " + err.Error()
	}
	pgStatus := "connected"
	if err := a.PG.Ping(context.Background()); err != nil {
		pgStatus = "error: " + err.Error()
	}
	c.JSON(http.StatusOK, gin.H{
		"status":         "ok",
		"duckdb":         duckStatus,
		"postgres":       pgStatus,
		"uptime_seconds": int(time.Since(startTime).Seconds()),
	})
}

func (a *App) GetSymbols(c *gin.Context) {
	assetClass := c.Query("asset_class")
	cacheKey := "symbols:" + assetClass
	if cached, ok := a.Cache.Get(cacheKey); ok {
		c.JSON(http.StatusOK, cached)
		return
	}
	symbols, err := store.QuerySymbolsPG(c.Request.Context(), a.PG, assetClass)
	if err != nil {
		abortError(c, http.StatusInternalServerError, err.Error(), "QUERY_ERROR")
		return
	}
	if len(symbols) == 0 {
		abortError(c, http.StatusNotFound, "no symbols found", "NOT_FOUND")
		return
	}
	resp := gin.H{"symbols": symbols}
	a.Cache.Set(cacheKey, resp)
	c.JSON(http.StatusOK, resp)
}

func (a *App) GetOHLCV(c *gin.Context) {
	symbol, from, to, ok := symbolAndRange(c)
	if !ok {
		return
	}

	bars, assetClass, exchange, err := store.QueryOHLCVPG(c.Request.Context(), a.PG, symbol, from, to)
	if err != nil {
		abortError(c, http.StatusInternalServerError, err.Error(), "QUERY_ERROR")
		return
	}
	if len(bars) == 0 {
		abortError(c, http.StatusNotFound, "no data found for symbol/date range", "NOT_FOUND")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"symbol": symbol, "asset_class": assetClass, "exchange": exchange, "bars": bars,
	})
}

func (a *App) GetIndicators(c *gin.Context) {
	symbol, from, to, ok := symbolAndRange(c)
	if !ok {
		return
	}

	indicators, err := store.QueryIndicatorsPG(c.Request.Context(), a.PG, symbol, from, to)
	if err != nil {
		abortError(c, http.StatusInternalServerError, err.Error(), "QUERY_ERROR")
		return
	}
	if len(indicators) == 0 {
		abortError(c, http.StatusNotFound, "no indicators found for symbol/date range", "NOT_FOUND")
		return
	}
	c.JSON(http.StatusOK, gin.H{"symbol": symbol, "indicators": indicators})
}

func (a *App) GetDigest(c *gin.Context) {
	now := time.Now()
	date := c.DefaultQuery("date", now.Format("2006-01-02"))
	category := c.DefaultQuery("category", "")
	limitStr := c.DefaultQuery("limit", "10")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		abortError(c, http.StatusBadRequest, "limit must be a positive integer", "INVALID_PARAM")
		return
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		abortError(c, http.StatusBadRequest, "date must be YYYY-MM-DD", "INVALID_PARAM")
		return
	}

	entries, err := store.QueryDigestPG(c.Request.Context(), a.PG, date, category, limit)
	if err != nil {
		abortError(c, http.StatusInternalServerError, err.Error(), "QUERY_ERROR")
		return
	}

	fallback := false
	if len(entries) == 0 {
		latestDate, err := store.LatestDigestDate(c.Request.Context(), a.PG)
		if err != nil || latestDate == "" {
			abortError(c, http.StatusNotFound, "no digest data available", "NOT_FOUND")
			return
		}
		entries, err = store.QueryDigestPG(c.Request.Context(), a.PG, latestDate, category, limit)
		if err != nil {
			abortError(c, http.StatusInternalServerError, err.Error(), "QUERY_ERROR")
			return
		}
		if len(entries) == 0 {
			abortError(c, http.StatusNotFound, "no digest data available", "NOT_FOUND")
			return
		}
		date = latestDate
		fallback = true
	}

	c.JSON(http.StatusOK, gin.H{"date": date, "digest": entries, "fallback": fallback})
}

func (a *App) GetDigestLive(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	category := c.DefaultQuery("category", "")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		abortError(c, http.StatusBadRequest, "limit must be a positive integer", "INVALID_PARAM")
		return
	}

	if a.Cfg.WSInternalURL == "" {
		abortError(c, http.StatusServiceUnavailable, "snapshot service not configured", "NOT_CONFIGURED")
		return
	}

	endpoint := a.Cfg.WSInternalURL + "/internal/snapshot/all"
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, endpoint, nil)
	if err != nil {
		abortError(c, http.StatusInternalServerError, "failed to build snapshot request", "INTERNAL_ERROR")
		return
	}
	resp, err := snapshotClient.Do(req)
	if err != nil {
		abortError(c, http.StatusBadGateway, "snapshot service unreachable", "BAD_GATEWAY")
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var ticks []model.PriceSnapshot
	if err := json.Unmarshal(body, &ticks); err != nil {
		abortError(c, http.StatusInternalServerError, "invalid snapshot response", "PARSE_ERROR")
		return
	}

	entries := computeLiveDigest(ticks, category, limit)
	c.JSON(http.StatusOK, gin.H{
		"live":   true,
		"as_of":  time.Now().UTC().Format(time.RFC3339),
		"digest": entries,
	})
}

func computeLiveDigest(ticks []model.PriceSnapshot, category string, topN int) []model.DigestEntry {
	valid := make([]model.PriceSnapshot, 0, len(ticks))
	for _, t := range ticks {
		if t.Price > 0 {
			valid = append(valid, t)
		}
	}

	cats := []string{"gainer", "loser", "volume"}
	if category != "" {
		cats = []string{category}
	}

	var out []model.DigestEntry
	for _, cat := range cats {
		out = append(out, rankLiveCategory(valid, cat, topN)...)
	}
	return out
}

func rankLiveCategory(ticks []model.PriceSnapshot, category string, topN int) []model.DigestEntry {
	pool := make([]model.PriceSnapshot, len(ticks))
	copy(pool, ticks)

	switch category {
	case "gainer":
		filtered := make([]model.PriceSnapshot, 0, len(pool))
		for _, t := range pool {
			if t.PctChange > 0 {
				filtered = append(filtered, t)
			}
		}
		sort.Slice(filtered, func(i, j int) bool { return filtered[i].PctChange > filtered[j].PctChange })
		pool = filtered
	case "loser":
		filtered := make([]model.PriceSnapshot, 0, len(pool))
		for _, t := range pool {
			if t.PctChange < 0 {
				filtered = append(filtered, t)
			}
		}
		sort.Slice(filtered, func(i, j int) bool { return filtered[i].PctChange < filtered[j].PctChange })
		pool = filtered
	case "volume":
		sort.Slice(pool, func(i, j int) bool { return pool[i].Volume > pool[j].Volume })
	}

	if len(pool) > topN {
		pool = pool[:topN]
	}

	out := make([]model.DigestEntry, len(pool))
	for i, t := range pool {
		openPrice := t.Price
		if t.PctChange != -100 {
			openPrice = t.Price / (1 + t.PctChange/100)
		}
		out[i] = model.DigestEntry{
			Category:   category,
			Rank:       i + 1,
			Symbol:     t.Symbol,
			Exchange:   t.Exchange,
			AssetClass: t.AssetClass,
			Open:       openPrice,
			Close:      t.Price,
			Volume:     t.Volume,
			PctChange:  t.PctChange,
		}
	}
	return out
}

func (a *App) GetScreener(c *gin.Context) {
	now := time.Now()
	_, isoWeek := now.ISOWeek()
	year := c.DefaultQuery("year", now.Format("2006"))
	week := c.DefaultQuery("week", fmt.Sprintf("%02d", isoWeek))

	results, err := store.QueryScreenerPG(c.Request.Context(), a.PG, year, week)
	if err != nil {
		abortError(c, http.StatusInternalServerError, err.Error(), "QUERY_ERROR")
		return
	}
	if len(results) == 0 {
		abortError(c, http.StatusNotFound, "no screener data for week", "NOT_FOUND")
		return
	}
	c.JSON(http.StatusOK, gin.H{"year": year, "week": week, "results": results})
}

func (a *App) GetSnapshot(c *gin.Context) {
	symbol := c.Query("symbol")
	if symbol == "" {
		abortError(c, http.StatusBadRequest, "symbol is required", "MISSING_PARAM")
		return
	}
	if a.Cfg.WSInternalURL == "" {
		abortError(c, http.StatusServiceUnavailable, "snapshot service not configured (Phase 3)", "NOT_CONFIGURED")
		return
	}

	endpoint := fmt.Sprintf("%s/internal/snapshot?symbol=%s", a.Cfg.WSInternalURL, url.QueryEscape(symbol))
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, endpoint, nil)
	if err != nil {
		abortError(c, http.StatusInternalServerError, "failed to build snapshot request", "INTERNAL_ERROR")
		return
	}
	resp, err := snapshotClient.Do(req)
	if err != nil {
		abortError(c, http.StatusBadGateway, "snapshot service unreachable", "BAD_GATEWAY")
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var snap model.PriceSnapshot
	if err := json.Unmarshal(body, &snap); err != nil {
		abortError(c, http.StatusInternalServerError, "invalid snapshot response", "PARSE_ERROR")
		return
	}
	c.JSON(http.StatusOK, snap)
}

func abortError(c *gin.Context, status int, msg, code string) {
	c.AbortWithStatusJSON(status, gin.H{"error": msg, "code": code, "status": status})
}
