package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

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

	addr := fmt.Sprintf(":%s", cfg.Port)
	slog.Info("server starting", "addr", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
