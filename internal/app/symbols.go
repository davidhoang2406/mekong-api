package app

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/davidhoang2406/mekong-api/internal/store"
)

func (a *App) GetSymbols(c *gin.Context) {
	assetClass := c.Query("asset_class")

	cacheKey := "symbols:" + assetClass
	if cached, ok := a.Cache.Get(cacheKey); ok {
		c.JSON(http.StatusOK, cached)
		return
	}

	symbols, err := store.QuerySymbols(a.DuckDB, a.Cfg.MinioAnalysisBucket, assetClass)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(), "code": "QUERY_ERROR", "status": 500,
		})
		return
	}
	if len(symbols) == 0 {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"error": "no symbols found", "code": "NOT_FOUND", "status": 404,
		})
		return
	}

	resp := gin.H{"symbols": symbols}
	a.Cache.Set(cacheKey, resp)
	c.JSON(http.StatusOK, resp)
}
