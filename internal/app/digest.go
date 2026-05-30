package app

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/davidhoang2406/mekong-api/internal/store"
)

func (a *App) GetDigest(c *gin.Context) {
	now := time.Now()
	date := c.DefaultQuery("date", now.Format("2006-01-02"))
	category := c.DefaultQuery("category", "")
	limitStr := c.DefaultQuery("limit", "10")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "limit must be a positive integer", "code": "INVALID_PARAM", "status": 400,
		})
		return
	}

	// Parse YYYY-MM-DD into parts for Hive partition filter.
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "date must be YYYY-MM-DD", "code": "INVALID_PARAM", "status": 400,
		})
		return
	}
	year := t.Format("2006")
	month := t.Format("01")
	day := t.Format("02")

	entries, err := store.QueryDigest(a.DuckDB, a.Cfg.MinioAnalysisBucket, year, month, day, category, limit)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(), "code": "QUERY_ERROR", "status": 500,
		})
		return
	}
	if len(entries) == 0 {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"error": "no digest data for date", "code": "NOT_FOUND", "status": 404,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"date":   date,
		"digest": entries,
	})
}
