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
		if ra.AdvertiserID == nil || strings.TrimSpace(*ra.AdvertiserID) == "" {
			if _, err := tx.ExecContext(ctx, `INSERT INTO user_roles (user_id, role_id, advertiser_id) VALUES ($1, $2, NULL) ON CONFLICT DO NOTHING`, userID, roleID); err != nil {
				return err
			}
			continue
		}
		advID := strings.TrimSpace(*ra.AdvertiserID)
		if _, err := tx.ExecContext(ctx, `INSERT INTO user_roles (user_id, role_id, advertiser_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, userID, roleID, advID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *userRoleRepository) ListUserRoleAssignments(ctx context.Context, userID string) ([]models.UserRoleAssignment, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT role_id, advertiser_id FROM user_roles WHERE user_id = $1 ORDER BY created_at ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.UserRoleAssignment
	for rows.Next() {
		var roleID string
		var advertiserID sql.NullString
		if err := rows.Scan(&roleID, &advertiserID); err != nil {
			return nil, err
		}
		var adv *string
		if advertiserID.Valid {
			v := advertiserID.String
			adv = &v
		}
		out = append(out, models.UserRoleAssignment{RoleID: roleID, AdvertiserID: adv})
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
			  AND ur.advertiser_id IS NULL
			  AND ro.name = 'super_admin'
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
			  AND (
				ur.advertiser_id IS NULL
				OR ($3::uuid IS NOT NULL AND ur.advertiser_id = $3::uuid)
			  )
		)
	`
	var adv any
	if advertiserID == nil || strings.TrimSpace(*advertiserID) == "" {
		adv = nil
	} else {
		adv = strings.TrimSpace(*advertiserID)
	}
	var ok bool
	if err := r.db.QueryRowContext(ctx, query, userID, permission, adv).Scan(&ok); err != nil {
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
			  AND ur.advertiser_id IS NOT NULL
		)
	`
	var ok bool
	if err := r.db.QueryRowContext(ctx, query, userID, permission).Scan(&ok); err != nil {
		return false, err
	}
	return ok, nil
}
