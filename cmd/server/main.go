// @title           Mekong Market Data API
// @version         1.0
// @description     Real-time and historical market data for Vietnamese stocks and crypto.
// @host            app.mekong.local
// @BasePath        /api/v1
// @schemes         http https
// @securityDefinitions.apikey BearerAuth
// @in              header
// @name            Authorization
// @description     JWT token — prefix value with "Bearer "
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	_ "github.com/davidhoang2406/mekong-api/docs"
	"github.com/davidhoang2406/mekong-api/internal/app"
	"github.com/davidhoang2406/mekong-api/internal/config"
	"github.com/davidhoang2406/mekong-api/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}
	if err := cfg.Validate(); err != nil {
		slog.Error("validate config", "err", err)
		os.Exit(1)
	}

	gin.SetMode(cfg.GinMode)

	duckDB, err := store.InitDuckDB(cfg)
	if err != nil {
		slog.Error("init duckdb", "err", err)
		os.Exit(1)
	}
	defer duckDB.Close()
	slog.Info("duckdb connected")

	ctx := context.Background()
	pg, err := store.InitPostgres(ctx, cfg)
	if err != nil {
		slog.Error("init postgres", "err", err)
		os.Exit(1)
	}
	defer pg.Close()
	slog.Info("postgres connected")

	a := app.New(duckDB, pg, cfg)
	r := a.SetupRouter()

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
	}
}
