package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"scm/internal/models"
)

type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id string) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	GetByIdentifier(ctx context.Context, identifier string) (*models.User, error)
	List(ctx context.Context, limit int, offset int) ([]models.User, error)
	Count(ctx context.Context) (int, error)
	ListAll(ctx context.Context) ([]models.User, error)
	UpdateProfile(ctx context.Context, id string, req *models.UpdateUserRequest) error
	UpdatePasswordHash(ctx context.Context, userID string, passwordHash string) error
	Delete(ctx context.Context, id string) error
}

type userRepository struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func NewUserRepository(pool *pgxpool.Pool) UserRepository {
	return &userRepository{pool: pool, queryTimeout: 5 * time.Second}
}

func (r *userRepository) ensurePool() error {
	if r.pool == nil {
		return errors.New("user repository: pgx pool is nil")
	}
	return nil
}

func (r *userRepository) translateNoRows(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return sql.ErrNoRows
	}
	return err
}

func (r *userRepository) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, r.queryTimeout)
}

func (r *userRepository) Create(ctx context.Context, user *models.User) error {
	if err := r.ensurePool(); err != nil {
		return err
	}

	query := `
		INSERT INTO users (id, email, name, user_name, phone_number, password_hash, created_at, status)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), $6, $7, COALESCE(NULLIF($8, ''), 'pending'))
		RETURNING created_at
	`

	execCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	err := r.pool.QueryRow(execCtx, query, user.ID, user.Email, user.Name, user.UserName, user.PhoneNumber, user.PasswordHash, user.CreatedAt, user.Status).Scan(&user.CreatedAt)
	return err
}

func (r *userRepository) GetByID(ctx context.Context, id string) (*models.User, error) {
	if err := r.ensurePool(); err != nil {
		return nil, err
	}

	query := `
		SELECT id, email, name, user_name, phone_number, status, password_hash, last_login_at, created_at
		FROM users
		WHERE id = $1
	`

	rowCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	var u models.User
	var name sql.NullString
	var userName sql.NullString
	var phoneNumber sql.NullString
	var lastLoginAt sql.NullTime
	err := r.pool.QueryRow(rowCtx, query, id).Scan(&u.ID, &u.Email, &name, &userName, &phoneNumber, &u.Status, &u.PasswordHash, &lastLoginAt, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}
	if name.Valid {
		u.Name = name.String
	}
	if userName.Valid {
		u.UserName = userName.String
	}
	if phoneNumber.Valid {
		u.PhoneNumber = phoneNumber.String
	}
	if lastLoginAt.Valid {
		v := lastLoginAt.Time
		u.LastLoginAt = &v
	}
	return &u, nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	if err := r.ensurePool(); err != nil {
		return nil, err
	}

	query := `
		SELECT id, email, name, user_name, phone_number, status, password_hash, last_login_at, created_at
		FROM users
		WHERE email = $1
	`

	rowCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	var u models.User
	var name sql.NullString
	var userName sql.NullString
	var phoneNumber sql.NullString
	var lastLoginAt sql.NullTime
	err := r.pool.QueryRow(rowCtx, query, email).Scan(&u.ID, &u.Email, &name, &userName, &phoneNumber, &u.Status, &u.PasswordHash, &lastLoginAt, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}
	if name.Valid {
		u.Name = name.String
	}
	if userName.Valid {
		u.UserName = userName.String
	}
	if phoneNumber.Valid {
		u.PhoneNumber = phoneNumber.String
	}
	if lastLoginAt.Valid {
		v := lastLoginAt.Time
		u.LastLoginAt = &v
	}
	return &u, nil
}

func (r *userRepository) GetByIdentifier(ctx context.Context, identifier string) (*models.User, error) {
	if err := r.ensurePool(); err != nil {
		return nil, err
	}

	query := `
		SELECT id, email, name, user_name, phone_number, status, password_hash, last_login_at, created_at
		FROM users
		WHERE LOWER(email) = LOWER($1)
		   OR LOWER(user_name) = LOWER($1)
		   OR phone_number = $1
		LIMIT 1
	`

	rowCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	var u models.User
	var name sql.NullString
	var userName sql.NullString
	var phoneNumber sql.NullString
	var lastLoginAt sql.NullTime
	err := r.pool.QueryRow(rowCtx, query, identifier).Scan(&u.ID, &u.Email, &name, &userName, &phoneNumber, &u.Status, &u.PasswordHash, &lastLoginAt, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}
	if name.Valid {
		u.Name = name.String
	}
	if userName.Valid {
		u.UserName = userName.String
	}
	if phoneNumber.Valid {
		u.PhoneNumber = phoneNumber.String
	}
	if lastLoginAt.Valid {
		v := lastLoginAt.Time
		u.LastLoginAt = &v
	}
	return &u, nil
}

func (r *userRepository) List(ctx context.Context, limit int, offset int) ([]models.User, error) {
	if err := r.ensurePool(); err != nil {
		return nil, err
	}

	query := `
		SELECT id, email, name, user_name, phone_number, status, last_login_at, created_at
		FROM users
		ORDER BY created_at DESC
	`

	args := make([]any, 0, 2)
	argPos := 1
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argPos)
		args = append(args, limit)
		argPos++
	}
	if offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argPos)
		args = append(args, offset)
	}

	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	rows, err := r.pool.Query(queryCtx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		var name sql.NullString
		var userName sql.NullString
		var phoneNumber sql.NullString
		var lastLoginAt sql.NullTime
		if err := rows.Scan(&u.ID, &u.Email, &name, &userName, &phoneNumber, &u.Status, &lastLoginAt, &u.CreatedAt); err != nil {
			return nil, err
		}
		if name.Valid {
			u.Name = name.String
		}
		if userName.Valid {
			u.UserName = userName.String
		}
		if phoneNumber.Valid {
			u.PhoneNumber = phoneNumber.String
		}
		if lastLoginAt.Valid {
			v := lastLoginAt.Time
			u.LastLoginAt = &v
		}
		users = append(users, u)
	}

	return users, rows.Err()
}

func (r *userRepository) Count(ctx context.Context) (int, error) {
	if err := r.ensurePool(); err != nil {
		return 0, err
	}

	query := `SELECT COUNT(*) FROM users`

	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	var total int
	if err := r.pool.QueryRow(queryCtx, query).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (r *userRepository) ListAll(ctx context.Context) ([]models.User, error) {
	if err := r.ensurePool(); err != nil {
		return nil, err
	}

	query := `
		SELECT id, email, name, user_name, phone_number, status, created_at
		FROM users
		ORDER BY created_at DESC
	`

	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	rows, err := r.pool.Query(queryCtx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		var name sql.NullString
		var userName sql.NullString
		var phoneNumber sql.NullString
		if err := rows.Scan(&u.ID, &u.Email, &name, &userName, &phoneNumber, &u.Status, &u.CreatedAt); err != nil {
			return nil, err
		}
		if name.Valid {
			u.Name = name.String
		}
		if userName.Valid {
			u.UserName = userName.String
		}
		if phoneNumber.Valid {
			u.PhoneNumber = phoneNumber.String
		}
		users = append(users, u)
	}

	return users, rows.Err()
}

func (r *userRepository) UpdateProfile(ctx context.Context, id string, req *models.UpdateUserRequest) error {
	if err := r.ensurePool(); err != nil {
		return err
	}

	query := `
		UPDATE users
		SET email = COALESCE($1, email),
			name = COALESCE($2, name),
			user_name = COALESCE($3, user_name),
			phone_number = COALESCE($4, phone_number),
			status = COALESCE($5, status)
		WHERE id = $6
		RETURNING id
	`

	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	var outID string
	err := r.pool.QueryRow(queryCtx, query, req.Email, req.Name, req.UserName, req.PhoneNumber, req.Status, id).Scan(&outID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("user not found")
		}
		return err
	}
	return nil
}

func (r *userRepository) Delete(ctx context.Context, id string) error {
	if err := r.ensurePool(); err != nil {
		return err
	}

	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	res, err := r.pool.Exec(queryCtx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (r *userRepository) UpdatePasswordHash(ctx context.Context, userID string, passwordHash string) error {
	if err := r.ensurePool(); err != nil {
		return err
	}

	query := `UPDATE users SET password_hash = $1 WHERE id = $2`

	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	res, err := r.pool.Exec(queryCtx, query, passwordHash, userID)
	if err != nil {
		return err
	}
	n := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}
