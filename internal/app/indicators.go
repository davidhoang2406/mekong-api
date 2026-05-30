package app

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/davidhoang2406/mekong-api/internal/store"
)

func (a *App) GetIndicators(c *gin.Context) {
	symbol := c.Query("symbol")
	if symbol == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "symbol is required", "code": "MISSING_PARAM", "status": 400,
		})
		return
	}

	now := time.Now()
	from := c.DefaultQuery("from", now.AddDate(0, 0, -30).Format("2006-01-02"))
	to := c.DefaultQuery("to", now.Format("2006-01-02"))

	indicators, err := store.QueryIndicators(a.DuckDB, a.Cfg.MinioAnalysisBucket, symbol, from, to)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(), "code": "QUERY_ERROR", "status": 500,
		})
		return
	}
	if len(indicators) == 0 {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"error": "no indicators found for symbol/date range", "code": "NOT_FOUND", "status": 404,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"symbol":     symbol,
		"indicators": indicators,
	})
}
