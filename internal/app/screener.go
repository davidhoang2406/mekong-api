package app

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/davidhoang2406/mekong-api/internal/store"
)

func (a *App) GetScreener(c *gin.Context) {
	now := time.Now()
	year := c.DefaultQuery("year", now.Format("2006"))
	_, isoWeek := now.ISOWeek()
	week := c.DefaultQuery("week", fmt.Sprintf("%02d", isoWeek))

	results, err := store.QueryScreener(a.DuckDB, a.Cfg.MinioAnalysisBucket, year, week)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(), "code": "QUERY_ERROR", "status": 500,
		})
		return
	}
	if len(results) == 0 {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"error": "no screener data for week", "code": "NOT_FOUND", "status": 404,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"year":    year,
		"week":    week,
		"results": results,
	})
}
