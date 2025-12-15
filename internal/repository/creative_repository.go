package repository

import (
    "context"
    "database/sql"
    "fmt"

    "github.com/lib/pq"
    "scm/internal/models"
)

type CreativeRepository interface {
    Create(ctx context.Context, creative *models.Creative) error
    GetByID(ctx context.Context, id string) (*models.Creative, error)
    ListAll(ctx context.Context) ([]*models.Creative, error)
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
            id, name, type, url, file_path, size, campaign_id, selected_days, time_slots, devices, uploaded_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
        RETURNING uploaded_at
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
        pq.Array(creative.SelectedDays),
        pq.Array(creative.TimeSlots),
        pq.Array(creative.Devices),
        creative.UploadedAt,
    ).Scan(&creative.UploadedAt)
    
    return err
}

func (r *creativeRepository) GetByID(ctx context.Context, id string) (*models.Creative, error) {
    query := `
        SELECT id, name, type, url, file_path, size, campaign_id, selected_days, time_slots, devices, uploaded_at
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
        pq.Array(&creative.SelectedDays),
        pq.Array(&creative.TimeSlots),
        pq.Array(&creative.Devices),
        &creative.UploadedAt,
    )
    
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, fmt.Errorf("creative not found")
        }
        return nil, err
    }
    
    return &creative, nil
}

func (r *creativeRepository) ListAll(ctx context.Context) ([]*models.Creative, error) {
    query := `
        SELECT
            id, name, type, url, file_path, size, campaign_id, selected_days, time_slots, devices, uploaded_at
        FROM creatives
        ORDER BY uploaded_at DESC
    `

    rows, err := r.db.QueryContext(ctx, query)
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
            pq.Array(&creative.SelectedDays),
            pq.Array(&creative.TimeSlots),
            pq.Array(&creative.Devices),
            &creative.UploadedAt,
        ); err != nil {
            return nil, err
        }
        creatives = append(creatives, &creative)
    }

    return creatives, rows.Err()
}

func (r *creativeRepository) ListByCampaign(ctx context.Context, campaignID string) ([]*models.Creative, error) {
    query := `
        SELECT
            id, name, type, url, file_path, size, campaign_id, selected_days, time_slots, devices, uploaded_at
        FROM creatives
        WHERE campaign_id = $1
        ORDER BY uploaded_at DESC
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
            pq.Array(&creative.SelectedDays),
            pq.Array(&creative.TimeSlots),
            pq.Array(&creative.Devices),
            &creative.UploadedAt,
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
        SET name = COALESCE($1, name)
        WHERE id = $2
        RETURNING id
    `
    
    err := r.db.QueryRowContext(
        ctx,
        query,
        req.Name,
        id,
    ).Scan(&id)
    
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