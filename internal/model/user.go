package model

type User struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

type APIKey struct {
	ID        string  `json:"id"`
	UserID    string  `json:"user_id"`
	Label     string  `json:"label"`
	RateLimit int     `json:"rate_limit"`
	IsActive  bool    `json:"is_active"`
	CreatedAt string  `json:"created_at"`
	LastUsed  *string `json:"last_used"`
}

type Watchlist struct {
	ID        string   `json:"id"`
	UserID    string   `json:"user_id"`
	Name      string   `json:"name"`
	Symbols   []string `json:"symbols"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
}
