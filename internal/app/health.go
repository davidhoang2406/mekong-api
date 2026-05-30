package app

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
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
