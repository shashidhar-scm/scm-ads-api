package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"scm/internal/models"
)

type ProjectRepository interface {
	Upsert(ctx context.Context, project *models.Project) error
	GetByName(ctx context.Context, name string) (*models.Project, error)
	List(ctx context.Context, limit int, offset int) ([]*models.Project, error)
	Count(ctx context.Context) (int, error)
}

type projectRepository struct {
	db *sql.DB
}

func NewProjectRepository(db *sql.DB) ProjectRepository {
	return &projectRepository{db: db}
}

func (r *projectRepository) Upsert(ctx context.Context, project *models.Project) error {
	ownerJSON, err := json.Marshal(project.Owner)
	if err != nil {
		return fmt.Errorf("marshal owner: %w", err)
	}
	languagesJSON, err := json.Marshal(project.Languages)
	if err != nil {
		return fmt.Errorf("marshal languages: %w", err)
	}
	regionJSON, err := json.Marshal(project.Region)
	if err != nil {
		return fmt.Errorf("marshal region: %w", err)
	}

	query := `
		INSERT INTO projects (
			id, owner, languages, name, company, description, max_devices,
			profile_img, header, sub_type, production, city_poster_frequency,
			ad_poster_frequency, city_poster_play_time, loop_length,
			smallbiz_support, proxy, address, latitude, longitude,
			is_transit, scm_health, priority, replicas, region, status, role,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29)
		ON CONFLICT (name) DO UPDATE SET
			id = EXCLUDED.id,
			owner = EXCLUDED.owner,
			languages = EXCLUDED.languages,
			company = EXCLUDED.company,
			description = EXCLUDED.description,
			max_devices = EXCLUDED.max_devices,
			profile_img = EXCLUDED.profile_img,
			header = EXCLUDED.header,
			sub_type = EXCLUDED.sub_type,
			production = EXCLUDED.production,
			city_poster_frequency = EXCLUDED.city_poster_frequency,
			ad_poster_frequency = EXCLUDED.ad_poster_frequency,
			city_poster_play_time = EXCLUDED.city_poster_play_time,
			loop_length = EXCLUDED.loop_length,
			smallbiz_support = EXCLUDED.smallbiz_support,
			proxy = EXCLUDED.proxy,
			address = EXCLUDED.address,
			latitude = EXCLUDED.latitude,
			longitude = EXCLUDED.longitude,
			is_transit = EXCLUDED.is_transit,
			scm_health = EXCLUDED.scm_health,
			priority = EXCLUDED.priority,
			replicas = EXCLUDED.replicas,
			region = EXCLUDED.region,
			status = EXCLUDED.status,
			role = EXCLUDED.role,
			updated_at = NOW()
	`

	now := time.Now().UTC()
	_, err = r.db.ExecContext(ctx, query,
		project.ID, ownerJSON, languagesJSON, project.Name, project.Company,
		project.Description, project.MaxDevices, project.ProfileImg,
		project.Header, project.SubType, project.Production,
		project.CityPosterFrequency, project.AdPosterFrequency,
		project.CityPosterPlayTime, project.LoopLength,
		project.SmallbizSupport, project.Proxy, project.Address,
		project.Latitude, project.Longitude, project.IsTransit,
		project.ScmHealth, project.Priority, project.Replicas,
		regionJSON, project.Status, project.Role,
		now, now,
	)
	if err != nil {
		return fmt.Errorf("upsert project: %w", err)
	}
	return nil
}

func (r *projectRepository) GetByName(ctx context.Context, name string) (*models.Project, error) {
	query := `
		SELECT id, owner, languages, name, company, description, max_devices,
			profile_img, header, sub_type, production, city_poster_frequency,
			ad_poster_frequency, city_poster_play_time, loop_length,
			smallbiz_support, proxy, address, latitude, longitude,
			is_transit, scm_health, priority, replicas, region, status, role,
			created_at, updated_at
		FROM projects
		WHERE name = $1
	`

	var project models.Project
	var ownerJSON, languagesJSON, regionJSON []byte
	err := r.db.QueryRowContext(ctx, query, name).Scan(
		&project.ID, &ownerJSON, &languagesJSON, &project.Name, &project.Company,
		&project.Description, &project.MaxDevices, &project.ProfileImg,
		&project.Header, &project.SubType, &project.Production,
		&project.CityPosterFrequency, &project.AdPosterFrequency,
		&project.CityPosterPlayTime, &project.LoopLength,
		&project.SmallbizSupport, &project.Proxy, &project.Address,
		&project.Latitude, &project.Longitude, &project.IsTransit,
		&project.ScmHealth, &project.Priority, &project.Replicas,
		&regionJSON, &project.Status, &project.Role,
		&project.CreatedAt, &project.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("project not found")
		}
		return nil, fmt.Errorf("get project: %w", err)
	}

	if err := json.Unmarshal(ownerJSON, &project.Owner); err != nil {
		return nil, fmt.Errorf("unmarshal owner: %w", err)
	}
	if err := json.Unmarshal(languagesJSON, &project.Languages); err != nil {
		return nil, fmt.Errorf("unmarshal languages: %w", err)
	}
	if err := json.Unmarshal(regionJSON, &project.Region); err != nil {
		return nil, fmt.Errorf("unmarshal region: %w", err)
	}

	return &project, nil
}

func (r *projectRepository) List(ctx context.Context, limit int, offset int) ([]*models.Project, error) {
	query := `
		SELECT id, owner, languages, name, company, description, max_devices,
			profile_img, header, sub_type, production, city_poster_frequency,
			ad_poster_frequency, city_poster_play_time, loop_length,
			smallbiz_support, proxy, address, latitude, longitude,
			is_transit, scm_health, priority, replicas, region, status, role,
			created_at, updated_at
		FROM projects
		ORDER BY created_at DESC
	`

	args := []interface{}{}
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
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []*models.Project
	for rows.Next() {
		var project models.Project
		var ownerJSON, languagesJSON, regionJSON []byte
		if err := rows.Scan(
			&project.ID, &ownerJSON, &languagesJSON, &project.Name, &project.Company,
			&project.Description, &project.MaxDevices, &project.ProfileImg,
			&project.Header, &project.SubType, &project.Production,
			&project.CityPosterFrequency, &project.AdPosterFrequency,
			&project.CityPosterPlayTime, &project.LoopLength,
			&project.SmallbizSupport, &project.Proxy, &project.Address,
			&project.Latitude, &project.Longitude, &project.IsTransit,
			&project.ScmHealth, &project.Priority, &project.Replicas,
			&regionJSON, &project.Status, &project.Role,
			&project.CreatedAt, &project.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}

		if err := json.Unmarshal(ownerJSON, &project.Owner); err != nil {
			return nil, fmt.Errorf("unmarshal owner: %w", err)
		}
		if err := json.Unmarshal(languagesJSON, &project.Languages); err != nil {
			return nil, fmt.Errorf("unmarshal languages: %w", err)
		}
		if err := json.Unmarshal(regionJSON, &project.Region); err != nil {
			return nil, fmt.Errorf("unmarshal region: %w", err)
		}

		projects = append(projects, &project)
	}

	return projects, rows.Err()
}

func (r *projectRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM projects").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count projects: %w", err)
	}
	return count, nil
}
