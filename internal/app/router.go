package app

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/davidhoang2406/mekong-api/internal/middleware"
)

func (a *App) SetupRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), middleware.SlogLogger())

	// Swagger UI at /api/docs
	r.GET("/api/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	v1 := r.Group("/api/v1")
	{
		v1.GET("/health", a.Health)
		v1.GET("/symbols", a.GetSymbols)
		v1.GET("/ohlcv", a.GetOHLCV)
		v1.GET("/indicators", a.GetIndicators)
		v1.GET("/digest", a.GetDigest)
		v1.GET("/digest/live", a.GetDigestLive)
		v1.GET("/screener", a.GetScreener)
		v1.GET("/snapshot", a.GetSnapshot)

		// Auth (public)
		v1.POST("/auth/register", a.Register)
		v1.POST("/auth/login", a.Login)
		v1.GET("/auth/google", a.GoogleLogin)
		v1.GET("/auth/google/callback", a.GoogleCallback)
		v1.GET("/auth/github", a.GitHubLogin)
		v1.GET("/auth/github/callback", a.GitHubCallback)

		// Protected routes
		auth := v1.Group("/")
		auth.Use(middleware.JWTAuth(a.Cfg.JWTSecret))
		{
			auth.GET("/users/me", a.Me)
			auth.POST("/users/me/keys", a.CreateAPIKey)
			auth.GET("/users/me/keys", a.ListAPIKeys)

			auth.GET("/watchlists", a.ListWatchlists)
			auth.POST("/watchlists", a.CreateWatchlist)
			auth.PUT("/watchlists/:id", a.UpdateWatchlist)
			auth.DELETE("/watchlists/:id", a.DeleteWatchlist)
		}
	}

	return r
}
