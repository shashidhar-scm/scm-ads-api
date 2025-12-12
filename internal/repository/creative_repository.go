package repository

import (
    "context"
    "database/sql"
    "fmt"
    "time"

    "scm/internal/models"
)

type CreativeRepository interface {
    Create(ctx context.Context, creative *models.Creative) error
    GetByID(ctx context.Context, id string) (*models.Creative, error)
    ListByCampaign(ctx context.Context, campaignID string) ([]*models.Creative, error)
    Update(ctx context.Context, id string, req *models.UpdateCreativeRequest) error
    Delete(ctx context.Context, id string) error
}

type creativeRepository struct {
    db *sql.DB
}

func NewCreativeRepository(db *sql.DB) CreativeRepository {
    return &creativeRepository{db: db}
}


func (r *creativeRepository) Create(ctx context.Context, creative *models.Creative) error {
    query := `
        INSERT INTO creatives (
            id, name, type, url, file_path, size, campaign_id, advertiser_id, created_at, updated_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
        RETURNING created_at, updated_at
    `
    
    err := r.db.QueryRowContext(
        ctx,
        query,
        creative.ID,
        creative.Name,
        creative.Type,
        creative.URL,
        creative.FilePath,
        creative.Size,
        creative.CampaignID,
        creative.AdvertiserID,
        creative.CreatedAt,
        creative.UpdatedAt,
    ).Scan(&creative.CreatedAt, &creative.UpdatedAt)
    
    return err
}

func (r *creativeRepository) GetByID(ctx context.Context, id string) (*models.Creative, error) {
    query := `
        SELECT id, name, type, url, file_path, size, campaign_id, advertiser_id, created_at, updated_at
        FROM creatives
        WHERE id = $1
    `
    
    var creative models.Creative
    err := r.db.QueryRowContext(ctx, query, id).Scan(
        &creative.ID,
        &creative.Name,
        &creative.Type,
        &creative.URL,
        &creative.FilePath,
        &creative.Size,
        &creative.CampaignID,
        &creative.AdvertiserID,
        &creative.CreatedAt,
        &creative.UpdatedAt,
    )
    
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, fmt.Errorf("creative not found")
        }
        return nil, err
    }
    
    return &creative, nil
}

func (r *creativeRepository) ListByCampaign(ctx context.Context, campaignID string) ([]*models.Creative, error) {
    query := `
        SELECT id, name, type, url, file_path, size, campaign_id, advertiser_id, created_at, updated_at
        FROM creatives
        WHERE campaign_id = $1
        ORDER BY created_at DESC
    `
    
    rows, err := r.db.QueryContext(ctx, query, campaignID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var creatives []*models.Creative
    for rows.Next() {
        var creative models.Creative
        if err := rows.Scan(
            &creative.ID,
            &creative.Name,
            &creative.Type,
            &creative.URL,
            &creative.FilePath,
            &creative.Size,
            &creative.CampaignID,
            &creative.AdvertiserID,
            &creative.CreatedAt,
            &creative.UpdatedAt,
        ); err != nil {
            return nil, err
        }
        creatives = append(creatives, &creative)
    }
    
    return creatives, rows.Err()
}

func (r *creativeRepository) Update(ctx context.Context, id string, req *models.UpdateCreativeRequest) error {
    query := `
        UPDATE creatives
        SET name = $1, updated_at = $2
        WHERE id = $3
        RETURNING updated_at
    `
    
    var updatedAt time.Time
    err := r.db.QueryRowContext(
        ctx,
        query,
        req.Name,
        time.Now().UTC(),
        id,
    ).Scan(&updatedAt)
    
    return err
}

func (r *creativeRepository) Delete(ctx context.Context, id string) error {
    query := `DELETE FROM creatives WHERE id = $1`
    result, err := r.db.ExecContext(ctx, query, id)
    if err != nil {
        return err
    }
    
    rowsAffected, err := result.RowsAffected()
    if err != nil {
        return err
    }
    
    if rowsAffected == 0 {
        return fmt.Errorf("creative not found")
    }
    
    return nil
}