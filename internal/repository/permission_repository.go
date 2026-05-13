package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"scm/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PermissionRepository interface {
	Create(ctx context.Context, permission *models.Permission) error
	GetByID(ctx context.Context, id string) (*models.Permission, error)
	List(ctx context.Context, limit int, offset int) ([]models.Permission, error)
	Count(ctx context.Context) (int, error)
	Update(ctx context.Context, id string, req *models.UpdatePermissionRequest) error
	Delete(ctx context.Context, id string) error
}

type permissionRepository struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func NewPermissionRepository(pool *pgxpool.Pool) PermissionRepository {
	return &permissionRepository{pool: pool, queryTimeout: 5 * time.Second}
}

func (r *permissionRepository) ensurePool() error {
	if r.pool == nil {
		return errors.New("permission repository: pgx pool is nil")
	}
	return nil
}

func (r *permissionRepository) translateNoRows(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return sql.ErrNoRows
	}
	return err
}

func (r *permissionRepository) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, r.queryTimeout)
}

func (r *permissionRepository) Create(ctx context.Context, permission *models.Permission) error {
	if err := r.ensurePool(); err != nil {
		return err
	}

	query := `
		INSERT INTO permissions (id, name, description, created_at)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at
	`
	if permission.ID == "" {
		return fmt.Errorf("permission id is required")
	}
	if permission.CreatedAt.IsZero() {
		permission.CreatedAt = time.Now().UTC()
	}

	execCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	return r.pool.QueryRow(execCtx, query, permission.ID, permission.Name, permission.Description, permission.CreatedAt).Scan(&permission.CreatedAt)
}

func (r *permissionRepository) GetByID(ctx context.Context, id string) (*models.Permission, error) {
	if err := r.ensurePool(); err != nil {
		return nil, err
	}

	query := `
		SELECT id, name, COALESCE(description, ''), created_at
		FROM permissions
		WHERE id = $1
	`
	var out models.Permission
	rowCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	if err := r.pool.QueryRow(rowCtx, query, id).Scan(&out.ID, &out.Name, &out.Description, &out.CreatedAt); err != nil {
		return nil, r.translateNoRows(err)
	}
	if strings.TrimSpace(out.Description) == "" {
		out.Description = ""
	}
	return &out, nil
}

func (r *permissionRepository) List(ctx context.Context, limit int, offset int) ([]models.Permission, error) {
	if err := r.ensurePool(); err != nil {
		return nil, err
	}

	query := `
		SELECT id, name, COALESCE(description, ''), created_at
		FROM permissions
		ORDER BY name
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
	var out []models.Permission
	for rows.Next() {
		var p models.Permission
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt); err != nil {
			return nil, err
		}
		if strings.TrimSpace(p.Description) == "" {
			p.Description = ""
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *permissionRepository) Count(ctx context.Context) (int, error) {
	if err := r.ensurePool(); err != nil {
		return 0, err
	}

	var total int
	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	if err := r.pool.QueryRow(queryCtx, `SELECT COUNT(*) FROM permissions`).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (r *permissionRepository) Update(ctx context.Context, id string, req *models.UpdatePermissionRequest) error {
	if err := r.ensurePool(); err != nil {
		return err
	}

	set := []string{}
	args := []any{}
	argPos := 1
	if req.Name != nil {
		set = append(set, fmt.Sprintf("name = $%d", argPos))
		args = append(args, *req.Name)
		argPos++
	}
	if req.Description != nil {
		set = append(set, fmt.Sprintf("description = $%d", argPos))
		args = append(args, *req.Description)
		argPos++
	}
	if len(set) == 0 {
		return nil
	}
	args = append(args, id)
	query := fmt.Sprintf("UPDATE permissions SET %s WHERE id = $%d", strings.Join(set, ", "), argPos)
	execCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	_, err := r.pool.Exec(execCtx, query, args...)
	return err
}

func (r *permissionRepository) Delete(ctx context.Context, id string) error {
	if err := r.ensurePool(); err != nil {
		return err
	}

	execCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	_, err := r.pool.Exec(execCtx, `DELETE FROM permissions WHERE id = $1`, id)
	return err
}
