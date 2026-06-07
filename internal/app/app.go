package app

import (
	"database/sql"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/davidhoang2406/mekong-api/internal/config"
	"github.com/davidhoang2406/mekong-api/internal/store"
)

// oauthStateStore holds short-lived OAuth state values server-side so
// state validation works regardless of cookie domain differences.
type oauthStateStore struct {
	mu     sync.Mutex
	states map[string]time.Time
}

func newOAuthStateStore() *oauthStateStore {
	return &oauthStateStore{states: make(map[string]time.Time)}
}

func (s *oauthStateStore) set(state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Prune expired entries on every write
	now := time.Now()
	for k, exp := range s.states {
		if now.After(exp) {
			delete(s.states, k)
		}
	}
	s.states[state] = now.Add(10 * time.Minute)
}

func (s *oauthStateStore) validate(state string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.states[state]
	if !ok {
		return false
	}
	delete(s.states, state)
	return time.Now().Before(exp)
}

type App struct {
	DuckDB     *sql.DB
	PG         *pgxpool.Pool
	Cache      *store.Cache
	Cfg        config.Config
	oauthState *oauthStateStore
}

func New(duckDB *sql.DB, pg *pgxpool.Pool, cfg config.Config) *App {
	return &App{
		DuckDB:     duckDB,
		PG:         pg,
		Cache:      store.NewCache(time.Duration(cfg.CacheTTLSeconds) * time.Second),
		Cfg:        cfg,
		oauthState: newOAuthStateStore(),
	}
}
