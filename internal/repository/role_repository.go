package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"scm/internal/models"
)

type RoleRepository interface {
	Create(ctx context.Context, role *models.Role) error
	GetByID(ctx context.Context, id string) (*models.Role, error)
	List(ctx context.Context, limit int, offset int) ([]models.Role, error)
	Count(ctx context.Context) (int, error)
	Update(ctx context.Context, id string, req *models.UpdateRoleRequest) error
	Delete(ctx context.Context, id string) error
	SetPermissions(ctx context.Context, roleID string, permissionIDs []string) error
	ListPermissionIDs(ctx context.Context, roleID string) ([]string, error)
}

type roleRepository struct {
	db *sql.DB
}

func NewRoleRepository(db *sql.DB) RoleRepository {
	return &roleRepository{db: db}
}

func (r *roleRepository) Create(ctx context.Context, role *models.Role) error {
	query := `
		INSERT INTO roles (id, name, description, is_system, created_at)
		VALUES ($1, $2, $3, FALSE, $4)
		RETURNING created_at
	`
	if role.ID == "" {
		return fmt.Errorf("role id is required")
	}
	if role.CreatedAt.IsZero() {
		role.CreatedAt = time.Now().UTC()
	}
	return r.db.QueryRowContext(ctx, query, role.ID, role.Name, role.Description, role.CreatedAt).Scan(&role.CreatedAt)
}

func (r *roleRepository) GetByID(ctx context.Context, id string) (*models.Role, error) {
	query := `
		SELECT id, name, COALESCE(description, ''), is_system, created_at
		FROM roles
		WHERE id = $1
	`
	var out models.Role
	if err := r.db.QueryRowContext(ctx, query, id).Scan(&out.ID, &out.Name, &out.Description, &out.IsSystem, &out.CreatedAt); err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.Description) == "" {
		out.Description = ""
	}
	return &out, nil
}

func (r *roleRepository) List(ctx context.Context, limit int, offset int) ([]models.Role, error) {
	query := `
		SELECT id, name, COALESCE(description, ''), is_system, created_at
		FROM roles
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
	var out []models.Role
	for rows.Next() {
		var ro models.Role
		if err := rows.Scan(&ro.ID, &ro.Name, &ro.Description, &ro.IsSystem, &ro.CreatedAt); err != nil {
			return nil, err
		}
		if strings.TrimSpace(ro.Description) == "" {
			ro.Description = ""
		}
		out = append(out, ro)
	}
	return out, rows.Err()
}

func (r *roleRepository) Count(ctx context.Context) (int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM roles`).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (r *roleRepository) Update(ctx context.Context, id string, req *models.UpdateRoleRequest) error {
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
	query := fmt.Sprintf("UPDATE roles SET %s WHERE id = $%d", strings.Join(set, ", "), argPos)
	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}

func (r *roleRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM roles WHERE id = $1`, id)
	return err
}

func (r *roleRepository) SetPermissions(ctx context.Context, roleID string, permissionIDs []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM role_permissions WHERE role_id = $1`, roleID); err != nil {
		return err
	}

	for _, pid := range permissionIDs {
		pid = strings.TrimSpace(pid)
		if pid == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, roleID, pid); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *roleRepository) ListPermissionIDs(ctx context.Context, roleID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT permission_id FROM role_permissions WHERE role_id = $1 ORDER BY permission_id`, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
