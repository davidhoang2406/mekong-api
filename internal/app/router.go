package app

import (
	"github.com/gin-gonic/gin"

	"github.com/davidhoang2406/mekong-api/internal/middleware"
)

func (a *App) SetupRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), middleware.SlogLogger())

	v1 := r.Group("/api/v1")
	{
		v1.GET("/health", a.Health)
		v1.GET("/symbols", a.GetSymbols)
		v1.GET("/ohlcv", a.GetOHLCV)
		v1.GET("/indicators", a.GetIndicators)
		v1.GET("/digest", a.GetDigest)
		v1.GET("/screener", a.GetScreener)
		v1.GET("/snapshot", a.GetSnapshot)
	}

	return r
}
