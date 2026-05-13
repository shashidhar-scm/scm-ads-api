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
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func NewRoleRepository(pool *pgxpool.Pool) RoleRepository {
	return &roleRepository{pool: pool, queryTimeout: 5 * time.Second}
}

func (r *roleRepository) ensurePool() error {
	if r.pool == nil {
		return errors.New("role repository: pgx pool is nil")
	}
	return nil
}

func (r *roleRepository) translateNoRows(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return sql.ErrNoRows
	}
	return err
}

func (r *roleRepository) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, r.queryTimeout)
}

func (r *roleRepository) Create(ctx context.Context, role *models.Role) error {
	if err := r.ensurePool(); err != nil {
		return err
	}

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
	execCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	return r.pool.QueryRow(execCtx, query, role.ID, role.Name, role.Description, role.CreatedAt).Scan(&role.CreatedAt)
}

func (r *roleRepository) GetByID(ctx context.Context, id string) (*models.Role, error) {
	if err := r.ensurePool(); err != nil {
		return nil, err
	}

	query := `
		SELECT id, name, COALESCE(description, ''), is_system, created_at
		FROM roles
		WHERE id = $1
	`
	var out models.Role
	rowCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	if err := r.pool.QueryRow(rowCtx, query, id).Scan(&out.ID, &out.Name, &out.Description, &out.IsSystem, &out.CreatedAt); err != nil {
		return nil, r.translateNoRows(err)
	}
	if strings.TrimSpace(out.Description) == "" {
		out.Description = ""
	}
	return &out, nil
}

func (r *roleRepository) List(ctx context.Context, limit int, offset int) ([]models.Role, error) {
	if err := r.ensurePool(); err != nil {
		return nil, err
	}

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
	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	rows, err := r.pool.Query(queryCtx, query, args...)
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
	if err := r.ensurePool(); err != nil {
		return 0, err
	}

	var total int
	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	if err := r.pool.QueryRow(queryCtx, `SELECT COUNT(*) FROM roles`).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (r *roleRepository) Update(ctx context.Context, id string, req *models.UpdateRoleRequest) error {
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
	query := fmt.Sprintf("UPDATE roles SET %s WHERE id = $%d", strings.Join(set, ", "), argPos)
	execCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	_, err := r.pool.Exec(execCtx, query, args...)
	return err
}

func (r *roleRepository) Delete(ctx context.Context, id string) error {
	if err := r.ensurePool(); err != nil {
		return err
	}

	execCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	_, err := r.pool.Exec(execCtx, `DELETE FROM roles WHERE id = $1`, id)
	return err
}

func (r *roleRepository) SetPermissions(ctx context.Context, roleID string, permissionIDs []string) error {
	if err := r.ensurePool(); err != nil {
		return err
	}

	execCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	tx, err := r.pool.BeginTx(execCtx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(execCtx)

	if _, err := tx.Exec(execCtx, `DELETE FROM role_permissions WHERE role_id = $1`, roleID); err != nil {
		return err
	}

	for _, pid := range permissionIDs {
		pid = strings.TrimSpace(pid)
		if pid == "" {
			continue
		}
		if _, err := tx.Exec(execCtx, `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, roleID, pid); err != nil {
			return err
		}
	}

	if err := tx.Commit(execCtx); err != nil {
		return err
	}
	return nil
}

func (r *roleRepository) ListPermissionIDs(ctx context.Context, roleID string) ([]string, error) {
	if err := r.ensurePool(); err != nil {
		return nil, err
	}

	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	rows, err := r.pool.Query(queryCtx, `SELECT permission_id FROM role_permissions WHERE role_id = $1 ORDER BY permission_id`, roleID)
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
