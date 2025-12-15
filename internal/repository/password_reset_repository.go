package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"scm/internal/models"
)

type PasswordResetRepository interface {
	Create(ctx context.Context, token *models.PasswordResetToken) error
	GetValidByTokenHash(ctx context.Context, tokenHash string) (*models.PasswordResetToken, error)
	MarkUsed(ctx context.Context, id string, usedAt time.Time) error
}

type passwordResetRepository struct {
	db *sql.DB
}

func NewPasswordResetRepository(db *sql.DB) PasswordResetRepository {
	return &passwordResetRepository{db: db}
}

func (r *passwordResetRepository) Create(ctx context.Context, token *models.PasswordResetToken) error {
	query := `
		INSERT INTO password_reset_tokens (id, user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at
	`

	err := r.db.QueryRowContext(ctx, query, token.ID, token.UserID, token.TokenHash, token.ExpiresAt, token.CreatedAt).Scan(&token.CreatedAt)
	return err
}

func (r *passwordResetRepository) GetValidByTokenHash(ctx context.Context, tokenHash string) (*models.PasswordResetToken, error) {
	query := `
		SELECT id, user_id, token_hash, expires_at, used_at, created_at
		FROM password_reset_tokens
		WHERE token_hash = $1
		AND used_at IS NULL
		AND expires_at > (NOW() AT TIME ZONE 'UTC')
		ORDER BY created_at DESC
		LIMIT 1
	`

	var t models.PasswordResetToken
	var usedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, query, tokenHash).Scan(&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &usedAt, &t.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("reset token not found")
		}
		return nil, err
	}
	if usedAt.Valid {
		t.UsedAt = &usedAt.Time
	}
	return &t, nil
}

func (r *passwordResetRepository) MarkUsed(ctx context.Context, id string, usedAt time.Time) error {
	query := `UPDATE password_reset_tokens SET used_at = $1 WHERE id = $2 AND used_at IS NULL`
	res, err := r.db.ExecContext(ctx, query, usedAt, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("reset token not found")
	}
	return nil
}
