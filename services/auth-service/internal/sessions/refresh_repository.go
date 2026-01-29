package sessions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/md-rashed-zaman/apptremind/libs/db"
)

type RefreshToken struct {
	ID        string
	UserID    string
	Hash      string
	ExpiresAt time.Time
	RevokedAt *time.Time
}

type RefreshRepository struct {
	pool *db.Pool
}

func NewRefreshRepository(pool *db.Pool) *RefreshRepository {
	return &RefreshRepository{pool: pool}
}

func (r *RefreshRepository) Create(ctx context.Context, userID string, rawToken string, expiresAt time.Time) (string, error) {
	id := uuid.NewString()
	hash := hashToken(rawToken)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
	`, id, userID, hash, expiresAt)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (r *RefreshRepository) GetByHash(ctx context.Context, hash string) (RefreshToken, error) {
	var token RefreshToken
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, token_hash, expires_at, revoked_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`, hash).Scan(&token.ID, &token.UserID, &token.Hash, &token.ExpiresAt, &token.RevokedAt)
	if err != nil {
		return RefreshToken{}, err
	}
	return token, nil
}

func (r *RefreshRepository) Revoke(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = now()
		WHERE id = $1
	`, id)
	return err
}

func IsNotFound(err error) bool {
	return err == pgx.ErrNoRows
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func HashToken(raw string) string {
	return hashToken(raw)
}
