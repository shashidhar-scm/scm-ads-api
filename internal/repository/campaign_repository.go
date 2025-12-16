package repository

import (
    "context"
    "database/sql"
    "errors"
    "fmt"
	"time"
    "strings"
	"log"

    "github.com/lib/pq"
    "scm/internal/interfaces"
    "scm/internal/models"
)

type campaignRepository struct {
    db *sql.DB
}

// Remove the CampaignFilter type from here since it's now in the interfaces package

func NewCampaignRepository(db *sql.DB) interfaces.CampaignRepository {
    return &campaignRepository{db: db}
}

func (r *campaignRepository) Create(ctx context.Context, campaign *models.Campaign) error {
    cities := campaign.Cities
    if cities == nil {
        cities = []string{}
    }

    query := `
        INSERT INTO campaigns (
            name, status, cities, start_date, end_date, budget, 
            spent, impressions, clicks, ctr, advertiser_id
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
        RETURNING id, created_at, updated_at
    `
    
    err := r.db.QueryRowContext(
        ctx,
        query,
        campaign.Name,
        campaign.Status,
        pq.Array(cities),
        campaign.StartDate,
        campaign.EndDate,
        campaign.Budget,
        campaign.Spent,
        campaign.Impressions,
        campaign.Clicks,
        campaign.CTR,
        campaign.AdvertiserID,
    ).Scan(&campaign.ID, &campaign.CreatedAt, &campaign.UpdatedAt)
    fmt.Println("Campaign created:", campaign)
    return err
}

func (r *campaignRepository) GetByID(ctx context.Context, id string) (*models.Campaign, error) {
    query := `
        SELECT 
            id, name, status, cities, start_date, end_date, budget,
            spent, impressions, clicks, ctr, advertiser_id,
            created_at, updated_at
        FROM campaigns 
        WHERE id = $1
    `
    
    var campaign models.Campaign
    err := r.db.QueryRowContext(ctx, query, id).Scan(
        &campaign.ID,
        &campaign.Name,
        &campaign.Status,
        pq.Array(&campaign.Cities),
        &campaign.StartDate,
        &campaign.EndDate,
        &campaign.Budget,
        &campaign.Spent,
        &campaign.Impressions,
        &campaign.Clicks,
        &campaign.CTR,
        &campaign.AdvertiserID,
        &campaign.CreatedAt,
        &campaign.UpdatedAt,
    )
    
    if err != nil {
		log.Println("Error fetching campaign with ID:", id, "Error:", err)
        if errors.Is(err, sql.ErrNoRows) {
			log.Println("Campaign not found with ID:", id)
            return nil, sql.ErrNoRows
        }
        return nil, err
    }
    
    return &campaign, nil
}

func (r *campaignRepository) Summary(ctx context.Context, filter interfaces.CampaignFilter) (*models.CampaignSummary, error) {
    query := `
        SELECT
            COALESCE(SUM(CASE WHEN status = 'active' THEN 1 ELSE 0 END), 0) AS active_campaign_count,
            COALESCE(SUM(budget), 0) AS total_budget,
            COALESCE(SUM(impressions), 0) AS total_impression
        FROM campaigns
        WHERE 1=1
    `

    var args []interface{}
    var whereClauses []string
    argPos := 1

    if filter.AdvertiserID != "" {
        whereClauses = append(whereClauses, fmt.Sprintf("advertiser_id = $%d", argPos))
        args = append(args, filter.AdvertiserID)
        argPos++
    }

    if filter.Status != "" {
        whereClauses = append(whereClauses, fmt.Sprintf("status = $%d", argPos))
        args = append(args, filter.Status)
        argPos++
    }

    if !filter.StartDate.IsZero() {
        whereClauses = append(whereClauses, fmt.Sprintf("start_date >= $%d", argPos))
        args = append(args, filter.StartDate)
        argPos++
    }

    if !filter.EndDate.IsZero() {
        whereClauses = append(whereClauses, fmt.Sprintf("end_date <= $%d", argPos))
        args = append(args, filter.EndDate)
        argPos++
    }

    if len(whereClauses) > 0 {
        query += " AND " + strings.Join(whereClauses, " AND ")
    }

    var summary models.CampaignSummary
    if err := r.db.QueryRowContext(ctx, query, args...).Scan(
        &summary.ActiveCampaignCount,
        &summary.TotalBudget,
        &summary.TotalImpression,
    ); err != nil {
        return nil, err
    }

    return &summary, nil
}

func (r *campaignRepository) ActivateScheduledStartingOn(ctx context.Context, startDate time.Time, scheduledStatus string, timeZone string) (int64, error) {
    if scheduledStatus == "" {
        scheduledStatus = "scheduled"
    }

    if timeZone == "" {
        timeZone = "UTC"
    }

    query := `
        UPDATE campaigns
        SET status = 'active',
            updated_at = NOW() AT TIME ZONE 'UTC'
        WHERE status = $1
          AND DATE(start_date AT TIME ZONE $3) = DATE($2 AT TIME ZONE $3)
    `

    res, err := r.db.ExecContext(ctx, query, scheduledStatus, startDate, timeZone)
    if err != nil {
        return 0, err
    }

    rows, err := res.RowsAffected()
    if err != nil {
        return 0, err
    }
    return rows, nil
}

// List retrieves a list of campaigns based on the provided filter
func (r *campaignRepository) List(ctx context.Context, filter interfaces.CampaignFilter) ([]*models.Campaign, error) {
    query := `
        SELECT 
            id, name, status, cities, start_date, end_date, budget,
            spent, impressions, clicks, ctr, advertiser_id,
            created_at, updated_at
        FROM campaigns
        WHERE 1=1
    `
    
    var args []interface{}
    var whereClauses []string
    argPos := 1

    if filter.AdvertiserID != "" {
        whereClauses = append(whereClauses, fmt.Sprintf("advertiser_id = $%d", argPos))
        args = append(args, filter.AdvertiserID)
        argPos++
    }

    if filter.Status != "" {
        whereClauses = append(whereClauses, fmt.Sprintf("status = $%d", argPos))
        args = append(args, filter.Status)
        argPos++
    }

    if !filter.StartDate.IsZero() {
        whereClauses = append(whereClauses, fmt.Sprintf("start_date >= $%d", argPos))
        args = append(args, filter.StartDate)
        argPos++
    }

    if !filter.EndDate.IsZero() {
        whereClauses = append(whereClauses, fmt.Sprintf("end_date <= $%d", argPos))
        args = append(args, filter.EndDate)
        argPos++
    }

    if len(whereClauses) > 0 {
        query += " AND " + strings.Join(whereClauses, " AND ")
    }

    // Add ordering and pagination
    query += " ORDER BY created_at DESC"

    if filter.Limit > 0 {
        query += fmt.Sprintf(" LIMIT $%d", argPos)
        args = append(args, filter.Limit)
        argPos++
    }

    if filter.Offset > 0 {
        query += fmt.Sprintf(" OFFSET $%d", argPos)
        args = append(args, filter.Offset)
    }

    rows, err := r.db.QueryContext(ctx, query, args...)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var campaigns []*models.Campaign
    for rows.Next() {
        var campaign models.Campaign
        err := rows.Scan(
            &campaign.ID,
            &campaign.Name,
            &campaign.Status,
            pq.Array(&campaign.Cities),
            &campaign.StartDate,
            &campaign.EndDate,
            &campaign.Budget,
            &campaign.Spent,
            &campaign.Impressions,
            &campaign.Clicks,
            &campaign.CTR,
            &campaign.AdvertiserID,
            &campaign.CreatedAt,
            &campaign.UpdatedAt,
        )
        if err != nil {
            return nil, err
        }
        campaigns = append(campaigns, &campaign)
    }

    return campaigns, rows.Err()
}

// Update updates a campaign with the given ID
func (r *campaignRepository) Update(ctx context.Context, id string, campaign *models.Campaign) error {
    cities := campaign.Cities
    if cities == nil {
        cities = []string{}
    }

    query := `
        UPDATE campaigns 
        SET name = $1, 
            status = $2, 
            cities = $3,
            start_date = $4, 
            end_date = $5, 
            budget = $6, 
            spent = $7, 
            impressions = $8, 
            clicks = $9, 
            ctr = $10, 
            advertiser_id = $11,
            updated_at = NOW() AT TIME ZONE 'UTC'
        WHERE id = $12
        RETURNING updated_at
    `

    err := r.db.QueryRowContext(
        ctx,
        query,
        campaign.Name,
        campaign.Status,
        pq.Array(cities),
        campaign.StartDate,
        campaign.EndDate,
        campaign.Budget,
        campaign.Spent,
        campaign.Impressions,
        campaign.Clicks,
        campaign.CTR,
        campaign.AdvertiserID,
        id,
    ).Scan(&campaign.UpdatedAt)

    if err != nil {
        if err == sql.ErrNoRows {
            return fmt.Errorf("campaign not found")
        }
        return fmt.Errorf("failed to update campaign: %w", err)
    }

    return nil
}

// Delete removes a campaign by ID
func (r *campaignRepository) Delete(ctx context.Context, id string) error {
    result, err := r.db.ExecContext(ctx, "DELETE FROM campaigns WHERE id = $1", id)
    if err != nil {
        return err
    }

    rowsAffected, err := result.RowsAffected()
    if err != nil {
        return err
    }

    if rowsAffected == 0 {
        return sql.ErrNoRows
    }

    return nil
}