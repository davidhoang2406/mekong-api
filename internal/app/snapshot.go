package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/davidhoang2406/mekong-api/internal/model"
)

func (a *App) GetSnapshot(c *gin.Context) {
	symbol := c.Query("symbol")
	if symbol == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "symbol is required", "code": "MISSING_PARAM", "status": 400,
		})
		return
	}

	wsHost := a.Cfg.WSInternalURL
	if wsHost == "" {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
			"error": "snapshot service not configured (Phase 3)", "code": "NOT_CONFIGURED", "status": 503,
		})
		return
	}

	url := fmt.Sprintf("%s/internal/snapshot?symbol=%s", wsHost, symbol)
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{
			"error": "snapshot service unreachable", "code": "BAD_GATEWAY", "status": 502,
		})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var snap model.PriceSnapshot
	if err := json.Unmarshal(body, &snap); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "invalid snapshot response", "code": "PARSE_ERROR", "status": 500,
		})
		return
	}

	c.JSON(http.StatusOK, snap)
}
