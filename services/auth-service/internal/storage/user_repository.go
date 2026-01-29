package storage

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/md-rashed-zaman/apptremind/libs/db"
)

type User struct {
	ID           string
	BusinessID   string
	Email        string
	PasswordHash string
	Role         string
}

type UserRepository struct {
	pool *db.Pool
}

func NewUserRepository(pool *db.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) Create(ctx context.Context, user User) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO users (id, business_id, email, password_hash, role)
		VALUES ($1, $2, $3, $4, $5)
	`, user.ID, user.BusinessID, user.Email, user.PasswordHash, user.Role)
	return err
}

func (r *UserRepository) CreateTx(ctx context.Context, tx pgx.Tx, user User) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO users (id, business_id, email, password_hash, role)
		VALUES ($1, $2, $3, $4, $5)
	`, user.ID, user.BusinessID, user.Email, user.PasswordHash, user.Role)
	return err
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (User, error) {
	var user User
	err := r.pool.QueryRow(ctx, `
		SELECT id, business_id, email, password_hash, role
		FROM users
		WHERE email = $1
	`, email).Scan(&user.ID, &user.BusinessID, &user.Email, &user.PasswordHash, &user.Role)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (User, error) {
	var user User
	err := r.pool.QueryRow(ctx, `
		SELECT id, business_id, email, password_hash, role
		FROM users
		WHERE id = $1
	`, id).Scan(&user.ID, &user.BusinessID, &user.Email, &user.PasswordHash, &user.Role)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func IsNotFound(err error) bool {
	return err == pgx.ErrNoRows
}
