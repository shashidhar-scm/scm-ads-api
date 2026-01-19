package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"scm/internal/models"
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
	db *sql.DB
}

func NewPermissionRepository(db *sql.DB) PermissionRepository {
	return &permissionRepository{db: db}
}

func (r *permissionRepository) Create(ctx context.Context, permission *models.Permission) error {
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
	return r.db.QueryRowContext(ctx, query, permission.ID, permission.Name, permission.Description, permission.CreatedAt).Scan(&permission.CreatedAt)
}

func (r *permissionRepository) GetByID(ctx context.Context, id string) (*models.Permission, error) {
	query := `
		SELECT id, name, COALESCE(description, ''), created_at
		FROM permissions
		WHERE id = $1
	`
	var out models.Permission
	if err := r.db.QueryRowContext(ctx, query, id).Scan(&out.ID, &out.Name, &out.Description, &out.CreatedAt); err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.Description) == "" {
		out.Description = ""
	}
	return &out, nil
}

func (r *permissionRepository) List(ctx context.Context, limit int, offset int) ([]models.Permission, error) {
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
	rows, err := r.db.QueryContext(ctx, query, args...)
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
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM permissions`).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (r *permissionRepository) Update(ctx context.Context, id string, req *models.UpdatePermissionRequest) error {
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
	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}

func (r *permissionRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM permissions WHERE id = $1`, id)
	return err
}
