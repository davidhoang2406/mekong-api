package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/davidhoang2406/mekong-api/internal/model"
	"github.com/davidhoang2406/mekong-api/internal/store"
)

var startTime = time.Now()

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
	symbols, err := store.QuerySymbols(a.DuckDB, a.Cfg.MinioAnalysisBucket, assetClass)
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
	symbol := c.Query("symbol")
	if symbol == "" {
		abortError(c, http.StatusBadRequest, "symbol is required", "MISSING_PARAM")
		return
	}
	now := time.Now()
	from := c.DefaultQuery("from", now.AddDate(0, 0, -30).Format("2006-01-02"))
	to := c.DefaultQuery("to", now.Format("2006-01-02"))

	bars, err := store.QueryOHLCV(a.DuckDB, a.Cfg.MinioAnalysisBucket, symbol, from, to)
	if err != nil {
		abortError(c, http.StatusInternalServerError, err.Error(), "QUERY_ERROR")
		return
	}
	if len(bars) == 0 {
		abortError(c, http.StatusNotFound, "no data found for symbol/date range", "NOT_FOUND")
		return
	}
	assetClass, exchange, _ := store.QueryOHLCVMeta(a.DuckDB, a.Cfg.MinioAnalysisBucket, symbol)
	c.JSON(http.StatusOK, gin.H{
		"symbol": symbol, "asset_class": assetClass, "exchange": exchange, "bars": bars,
	})
}

func (a *App) GetIndicators(c *gin.Context) {
	symbol := c.Query("symbol")
	if symbol == "" {
		abortError(c, http.StatusBadRequest, "symbol is required", "MISSING_PARAM")
		return
	}
	now := time.Now()
	from := c.DefaultQuery("from", now.AddDate(0, 0, -30).Format("2006-01-02"))
	to := c.DefaultQuery("to", now.Format("2006-01-02"))

	indicators, err := store.QueryIndicators(a.DuckDB, a.Cfg.MinioAnalysisBucket, symbol, from, to)
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
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		abortError(c, http.StatusBadRequest, "date must be YYYY-MM-DD", "INVALID_PARAM")
		return
	}

	entries, err := store.QueryDigest(
		a.DuckDB, a.Cfg.MinioAnalysisBucket,
		t.Format("2006"), t.Format("01"), t.Format("02"),
		category, limit,
	)
	if err != nil {
		abortError(c, http.StatusInternalServerError, err.Error(), "QUERY_ERROR")
		return
	}
	if len(entries) == 0 {
		abortError(c, http.StatusNotFound, "no digest data for date", "NOT_FOUND")
		return
	}
	c.JSON(http.StatusOK, gin.H{"date": date, "digest": entries})
}

func (a *App) GetScreener(c *gin.Context) {
	now := time.Now()
	_, isoWeek := now.ISOWeek()
	year := c.DefaultQuery("year", now.Format("2006"))
	week := c.DefaultQuery("week", fmt.Sprintf("%02d", isoWeek))

	results, err := store.QueryScreener(a.DuckDB, a.Cfg.MinioAnalysisBucket, year, week)
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

	url := fmt.Sprintf("%s/internal/snapshot?symbol=%s", a.Cfg.WSInternalURL, symbol)
	resp, err := http.Get(url) //nolint:gosec
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
