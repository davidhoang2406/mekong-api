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
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

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

// Health godoc
// @Summary     Service health check
// @Tags        system
// @Produce     json
// @Success     200 {object} map[string]interface{}
// @Router      /health [get]
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

// GetSymbols godoc
// @Summary     List available symbols
// @Tags        market-data
// @Produce     json
// @Param       asset_class query string false "Filter by asset class (stock, crypto)"
// @Success     200 {object} model.SymbolsResponse
// @Failure     404 {object} map[string]interface{}
// @Router      /symbols [get]
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

// GetOHLCV godoc
// @Summary     OHLCV bars for a symbol
// @Tags        market-data
// @Produce     json
// @Param       symbol query string true  "Symbol (e.g. VCB, BTC-USDT)"
// @Param       from   query string false "Start date YYYY-MM-DD (default: 30 days ago)"
// @Param       to     query string false "End date YYYY-MM-DD (default: today)"
// @Success     200 {object} model.OHLCVResponse
// @Failure     400 {object} map[string]interface{}
// @Failure     404 {object} map[string]interface{}
// @Router      /ohlcv [get]
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

// GetIndicators godoc
// @Summary     Technical indicators for a symbol
// @Tags        market-data
// @Produce     json
// @Param       symbol query string true  "Symbol"
// @Param       from   query string false "Start date YYYY-MM-DD"
// @Param       to     query string false "End date YYYY-MM-DD"
// @Success     200 {object} model.IndicatorsResponse
// @Failure     400 {object} map[string]interface{}
// @Failure     404 {object} map[string]interface{}
// @Router      /indicators [get]
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

// GetDigest godoc
// @Summary     Daily market digest (top gainers/losers/volume)
// @Tags        digest
// @Produce     json
// @Param       date     query string false "Date YYYY-MM-DD (default: today, falls back to latest)"
// @Param       category query string false "Filter category: gainer, loser, volume"
// @Param       limit    query int    false "Max results per category (default: 10)"
// @Success     200 {object} model.DigestResponse
// @Failure     404 {object} map[string]interface{}
// @Router      /digest [get]
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

// GetDigestLive godoc
// @Summary     Live intraday digest computed from WS snapshot cache
// @Tags        digest
// @Produce     json
// @Param       category query string false "Filter: gainer, loser, volume"
// @Param       limit    query int    false "Max results per category (default: 10)"
// @Success     200 {object} map[string]interface{}
// @Failure     503 {object} map[string]interface{}
// @Router      /digest/live [get]
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

// GetScreener godoc
// @Summary     Weekly fundamental screener
// @Tags        market-data
// @Produce     json
// @Param       year query string false "Year (default: current)"
// @Param       week query string false "ISO week number (default: current)"
// @Success     200 {object} model.ScreenerResponse
// @Failure     404 {object} map[string]interface{}
// @Router      /screener [get]
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

// GetSnapshot godoc
// @Summary     Latest live price snapshot for a symbol
// @Tags        live
// @Produce     json
// @Param       symbol query string true "Symbol (e.g. BTC-USDT)"
// @Success     200 {object} model.PriceSnapshot
// @Failure     400 {object} map[string]interface{}
// @Failure     502 {object} map[string]interface{}
// @Router      /snapshot [get]
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

// --- Auth ---

func (a *App) Register(c *gin.Context) {
	var body struct {
		Email    string `json:"email" binding:"required,email"`
		Name     string `json:"name"  binding:"required"`
		Password string `json:"password" binding:"required,min=8"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		abortError(c, http.StatusBadRequest, err.Error(), "INVALID_BODY")
		return
	}
	user, err := store.CreateUser(c.Request.Context(), a.PG, body.Email, body.Name, body.Password)
	if err != nil {
		abortError(c, http.StatusConflict, "email already registered", "CONFLICT")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"user": user})
}

func (a *App) Login(c *gin.Context) {
	var body struct {
		Email    string `json:"email"    binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		abortError(c, http.StatusBadRequest, err.Error(), "INVALID_BODY")
		return
	}
	user, hash, err := store.GetUserByEmail(c.Request.Context(), a.PG, body.Email)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Password)) != nil {
		abortError(c, http.StatusUnauthorized, "invalid credentials", "UNAUTHORIZED")
		return
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   user.ID,
		"email": user.Email,
		"exp":   time.Now().Add(time.Duration(a.Cfg.JWTExpiryHours) * time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	})
	signed, err := token.SignedString([]byte(a.Cfg.JWTSecret))
	if err != nil {
		abortError(c, http.StatusInternalServerError, "token signing failed", "INTERNAL_ERROR")
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": signed, "user": user})
}

func (a *App) Me(c *gin.Context) {
	userID, _ := c.Get("user_id")
	user, err := store.GetUserByID(c.Request.Context(), a.PG, fmt.Sprintf("%v", userID))
	if err != nil {
		abortError(c, http.StatusNotFound, "user not found", "NOT_FOUND")
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": user})
}

// --- API Keys ---

func (a *App) CreateAPIKey(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var body struct {
		Label string `json:"label"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.Label == "" {
		body.Label = "default"
	}
	key, rawKey, err := store.CreateAPIKey(c.Request.Context(), a.PG, fmt.Sprintf("%v", userID), body.Label)
	if err != nil {
		abortError(c, http.StatusInternalServerError, err.Error(), "INTERNAL_ERROR")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"key": key, "raw_key": rawKey, "note": "Save this key — it will not be shown again"})
}

func (a *App) ListAPIKeys(c *gin.Context) {
	userID, _ := c.Get("user_id")
	keys, err := store.ListAPIKeys(c.Request.Context(), a.PG, fmt.Sprintf("%v", userID))
	if err != nil {
		abortError(c, http.StatusInternalServerError, err.Error(), "INTERNAL_ERROR")
		return
	}
	c.JSON(http.StatusOK, gin.H{"keys": keys})
}

// --- Watchlists ---

func (a *App) ListWatchlists(c *gin.Context) {
	userID, _ := c.Get("user_id")
	lists, err := store.ListWatchlists(c.Request.Context(), a.PG, fmt.Sprintf("%v", userID))
	if err != nil {
		abortError(c, http.StatusInternalServerError, err.Error(), "INTERNAL_ERROR")
		return
	}
	c.JSON(http.StatusOK, gin.H{"watchlists": lists})
}

func (a *App) CreateWatchlist(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var body struct {
		Name    string   `json:"name"    binding:"required"`
		Symbols []string `json:"symbols"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		abortError(c, http.StatusBadRequest, err.Error(), "INVALID_BODY")
		return
	}
	if body.Symbols == nil {
		body.Symbols = []string{}
	}
	w, err := store.CreateWatchlist(c.Request.Context(), a.PG, fmt.Sprintf("%v", userID), body.Name, body.Symbols)
	if err != nil {
		abortError(c, http.StatusConflict, "watchlist name already exists", "CONFLICT")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"watchlist": w})
}

func (a *App) UpdateWatchlist(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := c.Param("id")
	var body struct {
		Symbols []string `json:"symbols" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		abortError(c, http.StatusBadRequest, err.Error(), "INVALID_BODY")
		return
	}
	w, err := store.UpdateWatchlist(c.Request.Context(), a.PG, id, fmt.Sprintf("%v", userID), body.Symbols)
	if err != nil {
		abortError(c, http.StatusNotFound, "watchlist not found", "NOT_FOUND")
		return
	}
	c.JSON(http.StatusOK, gin.H{"watchlist": w})
}

func (a *App) DeleteWatchlist(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := c.Param("id")
	if err := store.DeleteWatchlist(c.Request.Context(), a.PG, id, fmt.Sprintf("%v", userID)); err != nil {
		abortError(c, http.StatusNotFound, "watchlist not found", "NOT_FOUND")
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": id})
}
