package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/davidhoang2406/mekong-api/internal/model"
)

func CreateUser(ctx context.Context, pool *pgxpool.Pool, email, name, password string) (model.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return model.User{}, fmt.Errorf("hash password: %w", err)
	}
	var u model.User
	err = pool.QueryRow(ctx, `
		INSERT INTO users (email, name, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, email, name, to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SSZ')
	`, email, name, string(hash)).Scan(&u.ID, &u.Email, &u.Name, &u.CreatedAt)
	if err != nil {
		return model.User{}, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

func GetUserByEmail(ctx context.Context, pool *pgxpool.Pool, email string) (model.User, string, error) {
	var u model.User
	var hash string
	err := pool.QueryRow(ctx, `
		SELECT id, email, name, password_hash, to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SSZ')
		FROM users WHERE email = $1
	`, email).Scan(&u.ID, &u.Email, &u.Name, &hash, &u.CreatedAt)
	if err != nil {
		return model.User{}, "", fmt.Errorf("get user: %w", err)
	}
	return u, hash, nil
}

func GetUserByID(ctx context.Context, pool *pgxpool.Pool, id string) (model.User, error) {
	var u model.User
	err := pool.QueryRow(ctx, `
		SELECT id, email, name, to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SSZ')
		FROM users WHERE id = $1
	`, id).Scan(&u.ID, &u.Email, &u.Name, &u.CreatedAt)
	if err != nil {
		return model.User{}, fmt.Errorf("get user: %w", err)
	}
	return u, nil
}

func CreateAPIKey(ctx context.Context, pool *pgxpool.Pool, userID, label string) (model.APIKey, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return model.APIKey{}, "", fmt.Errorf("generate key: %w", err)
	}
	rawKey := "mk_" + hex.EncodeToString(raw)

	hash, err := bcrypt.GenerateFromPassword([]byte(rawKey), bcrypt.DefaultCost)
	if err != nil {
		return model.APIKey{}, "", fmt.Errorf("hash key: %w", err)
	}

	var k model.APIKey
	err = pool.QueryRow(ctx, `
		INSERT INTO api_keys (user_id, key_hash, label)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, label, rate_limit, is_active, to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SSZ')
	`, userID, string(hash), label).Scan(
		&k.ID, &k.UserID, &k.Label, &k.RateLimit, &k.IsActive, &k.CreatedAt,
	)
	if err != nil {
		return model.APIKey{}, "", fmt.Errorf("create api key: %w", err)
	}
	return k, rawKey, nil
}

// FindOrCreateSocialUser looks up a user by provider+providerID, creates them if new.
func FindOrCreateSocialUser(ctx context.Context, pool *pgxpool.Pool, provider, providerID, email, name string) (model.User, error) {
	var u model.User
	// Look up existing identity
	err := pool.QueryRow(ctx, `
		SELECT u.id, u.email, u.name, to_char(u.created_at, 'YYYY-MM-DD"T"HH24:MI:SSZ')
		FROM user_identities i JOIN users u ON u.id = i.user_id
		WHERE i.provider = $1 AND i.provider_id = $2
	`, provider, providerID).Scan(&u.ID, &u.Email, &u.Name, &u.CreatedAt)
	if err == nil {
		return u, nil
	}

	// Create user + identity in a transaction
	tx, err := pool.Begin(ctx)
	if err != nil {
		return model.User{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Upsert user by email (may already exist from local login)
	err = tx.QueryRow(ctx, `
		INSERT INTO users (email, name)
		VALUES ($1, $2)
		ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name
		RETURNING id, email, name, to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SSZ')
	`, email, name).Scan(&u.ID, &u.Email, &u.Name, &u.CreatedAt)
	if err != nil {
		return model.User{}, fmt.Errorf("upsert user: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO user_identities (user_id, provider, provider_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (provider, provider_id) DO NOTHING
	`, u.ID, provider, providerID)
	if err != nil {
		return model.User{}, fmt.Errorf("insert identity: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return model.User{}, fmt.Errorf("commit: %w", err)
	}
	return u, nil
}

func ListAPIKeys(ctx context.Context, pool *pgxpool.Pool, userID string) ([]model.APIKey, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, user_id, label, rate_limit, is_active,
		       to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SSZ'),
		       to_char(last_used, 'YYYY-MM-DD"T"HH24:MI:SSZ')
		FROM api_keys WHERE user_id = $1 ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()
	var out []model.APIKey
	for rows.Next() {
		var k model.APIKey
		if err := rows.Scan(&k.ID, &k.UserID, &k.Label, &k.RateLimit, &k.IsActive, &k.CreatedAt, &k.LastUsed); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}
