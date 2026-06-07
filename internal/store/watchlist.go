package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/davidhoang2406/mekong-api/internal/model"
)

func ListWatchlists(ctx context.Context, pool *pgxpool.Pool, userID string) ([]model.Watchlist, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, user_id, name, symbols,
		       to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SSZ'),
		       to_char(updated_at, 'YYYY-MM-DD"T"HH24:MI:SSZ')
		FROM watchlists WHERE user_id = $1 ORDER BY created_at ASC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list watchlists: %w", err)
	}
	defer rows.Close()
	var out []model.Watchlist
	for rows.Next() {
		var w model.Watchlist
		if err := rows.Scan(&w.ID, &w.UserID, &w.Name, &w.Symbols, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func CreateWatchlist(ctx context.Context, pool *pgxpool.Pool, userID, name string, symbols []string) (model.Watchlist, error) {
	var w model.Watchlist
	err := pool.QueryRow(ctx, `
		INSERT INTO watchlists (user_id, name, symbols)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, name, symbols,
		          to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SSZ'),
		          to_char(updated_at, 'YYYY-MM-DD"T"HH24:MI:SSZ')
	`, userID, name, symbols).Scan(
		&w.ID, &w.UserID, &w.Name, &w.Symbols, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		return model.Watchlist{}, fmt.Errorf("create watchlist: %w", err)
	}
	return w, nil
}

func UpdateWatchlist(ctx context.Context, pool *pgxpool.Pool, id, userID string, symbols []string) (model.Watchlist, error) {
	var w model.Watchlist
	err := pool.QueryRow(ctx, `
		UPDATE watchlists SET symbols = $1, updated_at = now()
		WHERE id = $2 AND user_id = $3
		RETURNING id, user_id, name, symbols,
		          to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SSZ'),
		          to_char(updated_at, 'YYYY-MM-DD"T"HH24:MI:SSZ')
	`, symbols, id, userID).Scan(
		&w.ID, &w.UserID, &w.Name, &w.Symbols, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		return model.Watchlist{}, fmt.Errorf("update watchlist: %w", err)
	}
	return w, nil
}

func DeleteWatchlist(ctx context.Context, pool *pgxpool.Pool, id, userID string) error {
	tag, err := pool.Exec(ctx, `DELETE FROM watchlists WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("delete watchlist: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("not found")
	}
	return nil
}
