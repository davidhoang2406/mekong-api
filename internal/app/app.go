package app

import (
	"database/sql"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/davidhoang2406/mekong-api/internal/config"
	"github.com/davidhoang2406/mekong-api/internal/store"
)

type App struct {
	DuckDB *sql.DB
	PG     *pgxpool.Pool
	Cache  *store.Cache
	Cfg    config.Config
}

func New(duckDB *sql.DB, pg *pgxpool.Pool, cfg config.Config) *App {
	return &App{
		DuckDB: duckDB,
		PG:     pg,
		Cache:  store.NewCache(time.Duration(cfg.CacheTTLSeconds) * time.Second),
		Cfg:    cfg,
	}
}
