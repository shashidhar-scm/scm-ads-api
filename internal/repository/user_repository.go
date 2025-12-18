package repository

import (
	"context"
	"database/sql"
	"fmt"

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
	db *sql.DB
}

func NewUserRepository(db *sql.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (id, email, name, user_name, phone_number, password_hash, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at
	`

	err := r.db.QueryRowContext(ctx, query, user.ID, user.Email, user.Name, user.UserName, user.PhoneNumber, user.PasswordHash, user.CreatedAt).Scan(&user.CreatedAt)
	return err
}

func (r *userRepository) GetByID(ctx context.Context, id string) (*models.User, error) {
	query := `
		SELECT id, email, name, user_name, phone_number, password_hash, created_at
		FROM users
		WHERE id = $1
	`

	var u models.User
	err := r.db.QueryRowContext(ctx, query, id).Scan(&u.ID, &u.Email, &u.Name, &u.UserName, &u.PhoneNumber, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT id, email, name, user_name, phone_number, password_hash, created_at
		FROM users
		WHERE email = $1
	`

	var u models.User
	err := r.db.QueryRowContext(ctx, query, email).Scan(&u.ID, &u.Email, &u.Name, &u.UserName, &u.PhoneNumber, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) GetByIdentifier(ctx context.Context, identifier string) (*models.User, error) {
	query := `
		SELECT id, email, name, user_name, phone_number, password_hash, created_at
		FROM users
		WHERE LOWER(email) = LOWER($1)
		   OR LOWER(user_name) = LOWER($1)
		   OR phone_number = $1
		LIMIT 1
	`

	var u models.User
	err := r.db.QueryRowContext(ctx, query, identifier).Scan(&u.ID, &u.Email, &u.Name, &u.UserName, &u.PhoneNumber, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) List(ctx context.Context, limit int, offset int) ([]models.User, error) {
	query := `
		SELECT id, email, name, user_name, phone_number, created_at
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

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.UserName, &u.PhoneNumber, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}

	return users, rows.Err()
}

func (r *userRepository) Count(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM users`
	var total int
	if err := r.db.QueryRowContext(ctx, query).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (r *userRepository) ListAll(ctx context.Context) ([]models.User, error) {
	query := `
		SELECT id, email, name, user_name, phone_number, created_at
		FROM users
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.UserName, &u.PhoneNumber, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}

	return users, rows.Err()
}

func (r *userRepository) UpdateProfile(ctx context.Context, id string, req *models.UpdateUserRequest) error {
	query := `
		UPDATE users
		SET email = COALESCE($1, email),
			name = COALESCE($2, name),
			user_name = COALESCE($3, user_name),
			phone_number = COALESCE($4, phone_number)
		WHERE id = $5
		RETURNING id
	`

	var outID string
	err := r.db.QueryRowContext(ctx, query, req.Email, req.Name, req.UserName, req.PhoneNumber, id).Scan(&outID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("user not found")
		}
		return err
	}
	return nil
}

func (r *userRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (r *userRepository) UpdatePasswordHash(ctx context.Context, userID string, passwordHash string) error {
	query := `UPDATE users SET password_hash = $1 WHERE id = $2`
	res, err := r.db.ExecContext(ctx, query, passwordHash, userID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}
