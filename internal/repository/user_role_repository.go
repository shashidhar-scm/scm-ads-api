package repository

import (
	"context"
	"database/sql"
	"strings"

	"scm/internal/models"
)

type UserRoleRepository interface {
	ReplaceUserRoles(ctx context.Context, userID string, roles []models.UserRoleAssignment) error
	ListUserRoleAssignments(ctx context.Context, userID string) ([]models.UserRoleAssignment, error)
	HasPermission(ctx context.Context, userID string, permission string, advertiserID *string) (bool, error)
	HasPermissionInAnyScope(ctx context.Context, userID string, permission string) (bool, error)
	IsSuperAdmin(ctx context.Context, userID string) (bool, error)
	IsAdmin(ctx context.Context, userID string) (bool, error)
	ListScopedAdvertiserIDs(ctx context.Context, userID string) ([]string, error)
}

type userRoleRepository struct {
	db *sql.DB
}

func NewUserRoleRepository(db *sql.DB) UserRoleRepository {
	return &userRoleRepository{db: db}
}

func (r *userRoleRepository) ReplaceUserRoles(ctx context.Context, userID string, roles []models.UserRoleAssignment) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id = $1`, userID); err != nil {
		return err
	}

	for _, ra := range roles {
		roleID := strings.TrimSpace(ra.RoleID)
		if roleID == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO user_roles (user_id, role_id, advertiser_id) VALUES ($1, $2, NULL) ON CONFLICT DO NOTHING`, userID, roleID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *userRoleRepository) ListUserRoleAssignments(ctx context.Context, userID string) ([]models.UserRoleAssignment, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT role_id FROM user_roles WHERE user_id = $1 ORDER BY created_at ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.UserRoleAssignment
	for rows.Next() {
		var roleID string
		if err := rows.Scan(&roleID); err != nil {
			return nil, err
		}
		out = append(out, models.UserRoleAssignment{RoleID: roleID})
	}
	return out, rows.Err()
}

func (r *userRoleRepository) IsSuperAdmin(ctx context.Context, userID string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1
			FROM user_roles ur
			JOIN roles ro ON ro.id = ur.role_id
			WHERE ur.user_id = $1
			  AND ro.name = 'super_admin'
		)
	`
	var ok bool
	if err := r.db.QueryRowContext(ctx, query, userID).Scan(&ok); err != nil {
		return false, err
	}
	return ok, nil
}

func (r *userRoleRepository) IsAdmin(ctx context.Context, userID string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1
			FROM user_roles ur
			JOIN roles ro ON ro.id = ur.role_id
			WHERE ur.user_id = $1
			  AND ro.name = 'admin'
		)
	`
	var ok bool
	if err := r.db.QueryRowContext(ctx, query, userID).Scan(&ok); err != nil {
		return false, err
	}
	return ok, nil
}

func (r *userRoleRepository) ListScopedAdvertiserIDs(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT advertiser_id
		FROM user_roles
		WHERE user_id = $1
		  AND advertiser_id IS NOT NULL
		ORDER BY advertiser_id ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		id = strings.TrimSpace(id)
		if id != "" {
			ids = append(ids, id)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if ids == nil {
		ids = []string{}
	}
	return ids, nil
}

func (r *userRoleRepository) HasPermission(ctx context.Context, userID string, permission string, advertiserID *string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1
			FROM user_roles ur
			JOIN roles ro ON ro.id = ur.role_id
			JOIN role_permissions rp ON rp.role_id = ro.id
			JOIN permissions p ON p.id = rp.permission_id
			WHERE ur.user_id = $1
			  AND p.name = $2
		)
	`
	var ok bool
	if err := r.db.QueryRowContext(ctx, query, userID, permission).Scan(&ok); err != nil {
		return false, err
	}
	return ok, nil
}

func (r *userRoleRepository) HasPermissionInAnyScope(ctx context.Context, userID string, permission string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1
			FROM user_roles ur
			JOIN roles ro ON ro.id = ur.role_id
			JOIN role_permissions rp ON rp.role_id = ro.id
			JOIN permissions p ON p.id = rp.permission_id
			WHERE ur.user_id = $1
			  AND p.name = $2
		)
	`
	var ok bool
	if err := r.db.QueryRowContext(ctx, query, userID, permission).Scan(&ok); err != nil {
		return false, err
	}
	return ok, nil
}
