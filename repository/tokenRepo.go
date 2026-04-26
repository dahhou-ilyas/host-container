package repository

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TokenRepo struct {
	pool *pgxpool.Pool
}

func NewTokenRepo(pool *pgxpool.Pool) *TokenRepo {
	return &TokenRepo{pool: pool}
}

func GenerateRawToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func HashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// Create invalidates any existing unused tokens of the same type for the user,
// then inserts a new one. Returns the raw (unhashed) token to be sent by email.
func (r *TokenRepo) Create(ctx context.Context, userID int64, tokenType string, ttl time.Duration) (string, error) {
	raw, err := GenerateRawToken()
	if err != nil {
		return "", err
	}
	hash := HashToken(raw)

	// Invalidate old tokens so only the latest link works
	_, _ = r.pool.Exec(ctx,
		`UPDATE email_tokens SET used_at = NOW()
		 WHERE user_id = $1 AND type = $2 AND used_at IS NULL`,
		userID, tokenType,
	)

	_, err = r.pool.Exec(ctx,
		`INSERT INTO email_tokens (user_id, token_hash, type, expires_at)
		 VALUES ($1, $2, $3, $4)`,
		userID, hash, tokenType, time.Now().Add(ttl),
	)
	if err != nil {
		return "", err
	}
	return raw, nil
}

type TokenRecord struct {
	ID     string
	UserID int64
	Type   string
}

// Consume validates the raw token, marks it as used, and returns the record.
// Returns pgx.ErrNoRows (via the caller) when the token is invalid, expired, or already used.
func (r *TokenRepo) Consume(ctx context.Context, rawToken, tokenType string) (*TokenRecord, error) {
	hash := HashToken(rawToken)
	var rec TokenRecord
	err := r.pool.QueryRow(ctx,
		`UPDATE email_tokens
		 SET used_at = NOW()
		 WHERE token_hash = $1
		   AND type       = $2
		   AND used_at    IS NULL
		   AND expires_at > NOW()
		 RETURNING id, user_id, type`,
		hash, tokenType,
	).Scan(&rec.ID, &rec.UserID, &rec.Type)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, pgx.ErrNoRows
		}
		return nil, err
	}
	return &rec, nil
}
