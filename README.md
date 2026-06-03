# mekong-api

Read-only REST API over the Mekong data lake. Serves market data and analytics
that the Spark batch jobs write to MinIO (Parquet), plus user/watchlist state in
Postgres. Queries Parquet directly via an embedded DuckDB connection (httpfs over
the S3-compatible MinIO endpoint); no separate query engine.

## Stack

- **Go 1.25**, [Gin](https://github.com/gin-gonic/gin) HTTP framework
- **DuckDB** (embedded, `read_parquet` over `s3://`) for analytical reads
- **Postgres** (pgx pool) for users, API keys, watchlists; schema via `golang-migrate`
- **MinIO** as the Parquet object store (`market-analysis` bucket)

## Layout

```
cmd/server/         main: config load, store init, graceful shutdown
internal/app/       App wiring, router, HTTP handlers
internal/store/     duckdb (analytical queries), postgres (pool + migrations), cache
internal/config/    env-based config + validation
internal/model/     response DTOs
internal/middleware/ slog request logging
migrations/         SQL migrations (golang-migrate)
test/unit/          handler, cache, config tests
test/integration/   duckdb tests (build tag: integration)
```

## Endpoints

Base path `/api/v1`:

| Method | Path | Query params |
|---|---|---|
| GET | `/health` | — |
| GET | `/symbols` | `asset_class` (optional) |
| GET | `/ohlcv` | `symbol` (required), `from`, `to` (default last 30d) |
| GET | `/indicators` | `symbol` (required), `from`, `to` |
| GET | `/digest` | `date` (YYYY-MM-DD), `category`, `limit` (default 10) |
| GET | `/screener` | `year`, `week` (default current ISO week) |
| GET | `/snapshot` | `symbol` (required) — proxies the realtime snapshot service |

## Configuration

Env vars (local: use a `.env`, never commit it). Defaults shown:

| Var | Default | Notes |
|---|---|---|
| `PORT` | `8090` | |
| `GIN_MODE` | `release` | |
| `MINIO_ENDPOINT` | `minio:9000` | |
| `MINIO_ACCESS_KEY` / `MINIO_SECRET_KEY` | `minioadmin` | override in prod |
| `MINIO_ANALYSIS_BUCKET` | `market-analysis` | |
| `POSTGRES_URL` | `postgres://mekong:mekong@postgres:5432/mekong_api?sslmode=disable` | override in prod |
| `POSTGRES_MAX_CONNS` | `10` | |
| `CACHE_TTL_SECONDS` | `300` | in-memory response cache |
| `WS_INTERNAL_URL` | _(empty)_ | realtime snapshot upstream (Phase 3) |
| `LOG_LEVEL` | `info` | |

## Develop

```bash
go build ./...
go vet ./...
go test ./...                          # unit
CGO_ENABLED=1 go test -tags=integration ./...   # integration (DuckDB, CGO)
```

Migrations run automatically on startup against `POSTGRES_URL`.
