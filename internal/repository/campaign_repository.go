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
            spent, clicks, ctr, advertiser_id, impressions_based
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
        campaign.Clicks,
        campaign.CTR,
        campaign.AdvertiserID,
        campaign.ImpressionsBased,
    ).Scan(&campaign.ID, &campaign.CreatedAt, &campaign.UpdatedAt)
    fmt.Println("Campaign created:", campaign)
    return err
}

func (r *campaignRepository) GetByID(ctx context.Context, id string) (*models.Campaign, error) {
    query := `
        SELECT 
            c.id, c.name, c.status, c.cities, c.start_date, c.end_date, c.budget,
            c.spent, c.clicks, c.ctr, c.advertiser_id, c.impressions_based,
            COALESCE(ci.total_impressions, 0) AS total_impressions,
            COALESCE(ci.served_impressions, 0) AS served_impressions,
            c.created_at, c.updated_at
        FROM campaigns c
        LEFT JOIN (
            SELECT
                campaign_id,
                COALESCE(SUM(impression_count), 0) AS total_impressions,
                COALESCE(SUM(impressions_served), 0) AS served_impressions
            FROM creatives
            GROUP BY campaign_id
        ) ci ON ci.campaign_id = c.id
        WHERE c.id = $1
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
        &campaign.Clicks,
        &campaign.CTR,
        &campaign.AdvertiserID,
        &campaign.ImpressionsBased,
        &campaign.TotalImpressions,
        &campaign.ServedImpressions,
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
            COALESCE(SUM(ci.total_impressions), 0) AS total_impression,
            COALESCE(SUM(ci.served_impressions), 0) AS served_impression
        FROM campaigns c
        LEFT JOIN (
            SELECT
                campaign_id,
                COALESCE(SUM(impression_count), 0) AS total_impressions,
                COALESCE(SUM(impressions_served), 0) AS served_impressions
            FROM creatives
            GROUP BY campaign_id
        ) ci ON ci.campaign_id = c.id
        WHERE 1=1
    `

    var args []interface{}
    var whereClauses []string
    argPos := 1

    if filter.AdvertiserID != "" {
        whereClauses = append(whereClauses, fmt.Sprintf("c.advertiser_id = $%d", argPos))
        args = append(args, filter.AdvertiserID)
        argPos++
    }

    if filter.CreatedByUserID != nil && strings.TrimSpace(*filter.CreatedByUserID) != "" {
        whereClauses = append(whereClauses, fmt.Sprintf("EXISTS (SELECT 1 FROM advertisers a WHERE a.id = c.advertiser_id AND a.created_by = $%d)", argPos))
        args = append(args, strings.TrimSpace(*filter.CreatedByUserID))
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
        &summary.ServedImpression,
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

func (r *campaignRepository) CompleteActiveEndedBefore(ctx context.Context, now time.Time, activeStatus string, completedStatus string, timeZone string) (int64, error) {
    if activeStatus == "" {
        activeStatus = "active"
    }

    if completedStatus == "" {
        completedStatus = "completed"
    }

    if timeZone == "" {
        timeZone = "UTC"
    }

    query := `
        UPDATE campaigns
        SET status = $2,
            updated_at = NOW() AT TIME ZONE 'UTC'
        WHERE status = $1
          AND DATE(end_date AT TIME ZONE $4) < DATE($3 AT TIME ZONE $4)
    `

    res, err := r.db.ExecContext(ctx, query, activeStatus, completedStatus, now, timeZone)
    if err != nil {
        return 0, err
    }

    rows, err := res.RowsAffected()
    if err != nil {
        return 0, err
    }
    return rows, nil
}

func (r *campaignRepository) Count(ctx context.Context, filter interfaces.CampaignFilter) (int, error) {
    query := `
        SELECT COUNT(*)
        FROM campaigns c
        WHERE 1=1
    `

    var args []interface{}
    var whereClauses []string
    argPos := 1

    if filter.AdvertiserID != "" {
        whereClauses = append(whereClauses, fmt.Sprintf("c.advertiser_id = $%d", argPos))
        args = append(args, filter.AdvertiserID)
        argPos++
    }

    if filter.CreatedByUserID != nil && strings.TrimSpace(*filter.CreatedByUserID) != "" {
        whereClauses = append(whereClauses, fmt.Sprintf("EXISTS (SELECT 1 FROM advertisers a WHERE a.id = c.advertiser_id AND a.created_by = $%d)", argPos))
        args = append(args, strings.TrimSpace(*filter.CreatedByUserID))
        argPos++
    }

    if filter.Status != "" {
        whereClauses = append(whereClauses, fmt.Sprintf("c.status = $%d", argPos))
        args = append(args, filter.Status)
        argPos++
    }

    if !filter.StartDate.IsZero() {
        whereClauses = append(whereClauses, fmt.Sprintf("c.start_date >= $%d", argPos))
        args = append(args, filter.StartDate)
        argPos++
    }

    if !filter.EndDate.IsZero() {
        whereClauses = append(whereClauses, fmt.Sprintf("c.end_date <= $%d", argPos))
        args = append(args, filter.EndDate)
        argPos++
    }

    if len(whereClauses) > 0 {
        query += " AND " + strings.Join(whereClauses, " AND ")
    }

    var total int
    if err := r.db.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
        return 0, err
    }
    return total, nil
}

func (r *campaignRepository) Search(ctx context.Context, term string, limit int, offset int, createdByUserID *string) ([]*models.Campaign, int, error) {
    likeTerm := fmt.Sprintf("%%%s%%", strings.ToLower(term))

    countQuery := `
        SELECT COUNT(*)
        FROM campaigns c
        WHERE (
            LOWER(c.name) LIKE $1
           OR LOWER(COALESCE(array_to_string(c.cities, ' '), '' )) LIKE $1
           OR LOWER(to_char(c.start_date, 'YYYY-MM-DD"T"HH24:MI:SS')) LIKE $1
           OR LOWER(to_char(c.end_date, 'YYYY-MM-DD"T"HH24:MI:SS')) LIKE $1
           OR LOWER(CAST(c.budget AS TEXT)) LIKE $1
           OR LOWER(CAST(c.spent AS TEXT)) LIKE $1
           OR LOWER(CAST(c.clicks AS TEXT)) LIKE $1
        )
    `

    args := []any{likeTerm}
    if createdByUserID != nil && strings.TrimSpace(*createdByUserID) != "" {
        countQuery += " AND EXISTS (SELECT 1 FROM advertisers a WHERE a.id = c.advertiser_id AND a.created_by = $2)"
        args = append(args, strings.TrimSpace(*createdByUserID))
    }

    var total int
    if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
        return nil, 0, fmt.Errorf("search campaigns count: %w", err)
    }

    query := `
        SELECT
            c.id, c.name, c.status, c.cities, c.start_date, c.end_date, c.budget,
            c.spent, c.clicks, c.ctr, c.advertiser_id, c.impressions_based,
            COALESCE(ci.total_impressions, 0) AS total_impressions,
            COALESCE(ci.served_impressions, 0) AS served_impressions,
            c.created_at, c.updated_at
        FROM campaigns c
        LEFT JOIN (
            SELECT
                campaign_id,
                COALESCE(SUM(impression_count), 0) AS total_impressions,
                COALESCE(SUM(impressions_served), 0) AS served_impressions
            FROM creatives
            GROUP BY campaign_id
        ) ci ON ci.campaign_id = c.id
        WHERE (
            LOWER(c.name) LIKE $1
           OR LOWER(COALESCE(array_to_string(c.cities, ' '), '' )) LIKE $1
           OR LOWER(to_char(c.start_date, 'YYYY-MM-DD"T"HH24:MI:SS')) LIKE $1
           OR LOWER(to_char(c.end_date, 'YYYY-MM-DD"T"HH24:MI:SS')) LIKE $1
           OR LOWER(CAST(c.budget AS TEXT)) LIKE $1
           OR LOWER(CAST(c.spent AS TEXT)) LIKE $1
           OR LOWER(CAST(c.clicks AS TEXT)) LIKE $1
        )
    `

    qArgs := []any{likeTerm}
    if createdByUserID != nil && strings.TrimSpace(*createdByUserID) != "" {
        query = strings.ReplaceAll(query, "WHERE (", "WHERE EXISTS (SELECT 1 FROM advertisers a WHERE a.id = c.advertiser_id AND a.created_by = $2) AND (")
        qArgs = append(qArgs, strings.TrimSpace(*createdByUserID))
    }

    query += " ORDER BY updated_at DESC"
    query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(qArgs)+1, len(qArgs)+2)
    qArgs = append(qArgs, limit, offset)

    rows, err := r.db.QueryContext(ctx, query, qArgs...)
    if err != nil {
        return nil, 0, fmt.Errorf("search campaigns: %w", err)
    }
    defer rows.Close()

    var campaigns []*models.Campaign
    for rows.Next() {
        var campaign models.Campaign
        if err := rows.Scan(
            &campaign.ID,
            &campaign.Name,
            &campaign.Status,
            pq.Array(&campaign.Cities),
            &campaign.StartDate,
            &campaign.EndDate,
            &campaign.Budget,
            &campaign.Spent,
            &campaign.Clicks,
            &campaign.CTR,
            &campaign.AdvertiserID,
            &campaign.ImpressionsBased,
            &campaign.TotalImpressions,
            &campaign.ServedImpressions,
            &campaign.CreatedAt,
            &campaign.UpdatedAt,
        ); err != nil {
            return nil, 0, fmt.Errorf("scan campaign: %w", err)
        }
        campaigns = append(campaigns, &campaign)
    }

    if err := rows.Err(); err != nil {
        return nil, 0, fmt.Errorf("iterate campaigns: %w", err)
    }

    return campaigns, total, nil
}

func (r *campaignRepository) List(ctx context.Context, filter interfaces.CampaignFilter) ([]*models.Campaign, error) {
    query := `
        SELECT 
            c.id, c.name, c.status, c.cities, c.start_date, c.end_date, c.budget,
            c.spent, c.clicks, c.ctr, c.advertiser_id, c.impressions_based,
            COALESCE(ci.total_impressions, 0) AS total_impressions,
            COALESCE(ci.served_impressions, 0) AS served_impressions,
            c.created_at, c.updated_at
        FROM campaigns c
        LEFT JOIN (
            SELECT
                campaign_id,
                COALESCE(SUM(impression_count), 0) AS total_impressions,
                COALESCE(SUM(impressions_served), 0) AS served_impressions
            FROM creatives
            GROUP BY campaign_id
        ) ci ON ci.campaign_id = c.id
        WHERE 1=1
    `

	var args []interface{}
	var whereClauses []string
	argPos := 1

    if filter.AdvertiserID != "" {
        whereClauses = append(whereClauses, fmt.Sprintf("c.advertiser_id = $%d", argPos))
        args = append(args, filter.AdvertiserID)
        argPos++
    }

	if filter.CreatedByUserID != nil && strings.TrimSpace(*filter.CreatedByUserID) != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("EXISTS (SELECT 1 FROM advertisers a WHERE a.id = c.advertiser_id AND a.created_by = $%d)", argPos))
		args = append(args, strings.TrimSpace(*filter.CreatedByUserID))
		argPos++
	}

    if filter.Status != "" {
        whereClauses = append(whereClauses, fmt.Sprintf("c.status = $%d", argPos))
        args = append(args, filter.Status)
        argPos++
    }

    if !filter.StartDate.IsZero() {
        whereClauses = append(whereClauses, fmt.Sprintf("c.start_date >= $%d", argPos))
        args = append(args, filter.StartDate)
        argPos++
    }

    if !filter.EndDate.IsZero() {
        whereClauses = append(whereClauses, fmt.Sprintf("c.end_date <= $%d", argPos))
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
            &campaign.Clicks,
            &campaign.CTR,
            &campaign.AdvertiserID,
            &campaign.ImpressionsBased,
            &campaign.TotalImpressions,
            &campaign.ServedImpressions,
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

func (r *campaignRepository) ListByEndDate(ctx context.Context, endDate time.Time) ([]*models.Campaign, error) {
    query := `
        SELECT 
            c.id, c.name, c.status, c.cities, c.start_date, c.end_date, c.budget,
            c.spent, c.clicks, c.ctr, c.advertiser_id, c.impressions_based,
            COALESCE(ci.total_impressions, 0) AS total_impressions,
            COALESCE(ci.served_impressions, 0) AS served_impressions,
            c.created_at, c.updated_at
        FROM campaigns c
        LEFT JOIN (
            SELECT
                campaign_id,
                COALESCE(SUM(impression_count), 0) AS total_impressions,
                COALESCE(SUM(impressions_served), 0) AS served_impressions
            FROM creatives
            GROUP BY campaign_id
        ) ci ON ci.campaign_id = c.id
        WHERE DATE(c.end_date) = DATE($1)
    `

    rows, err := r.db.QueryContext(ctx, query, endDate)
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
            &campaign.Clicks,
            &campaign.CTR,
            &campaign.AdvertiserID,
            &campaign.ImpressionsBased,
            &campaign.TotalImpressions,
            &campaign.ServedImpressions,
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
            clicks = $8, 
            ctr = $9, 
            advertiser_id = $10,
            impressions_based = $11,
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
        campaign.Clicks,
        campaign.CTR,
        campaign.AdvertiserID,
        campaign.ImpressionsBased,
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
    var creativeCount int64
    if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM creatives WHERE campaign_id = $1`, id).Scan(&creativeCount); err != nil {
        return err
    }
    if creativeCount > 0 {
        return &interfaces.DeletionBlockedError{
            Resource: "campaign",
            References: map[string]int64{
                "creatives": creativeCount,
            },
        }
    }

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

func (r *campaignRepository) ListByStartDate(ctx context.Context, startDate time.Time) ([]*models.Campaign, error) {
    query := `
        SELECT 
            c.id, c.name, c.status, c.cities, c.start_date, c.end_date, c.budget,
            c.spent, c.clicks, c.ctr, c.advertiser_id, c.impressions_based,
            COALESCE(ci.total_impressions, 0) AS total_impressions,
            COALESCE(ci.served_impressions, 0) AS served_impressions,
            c.created_at, c.updated_at
        FROM campaigns c
        LEFT JOIN (
            SELECT
                campaign_id,
                COALESCE(SUM(impression_count), 0) AS total_impressions,
                COALESCE(SUM(impressions_served), 0) AS served_impressions
            FROM creatives
            GROUP BY campaign_id
        ) ci ON ci.campaign_id = c.id
        WHERE DATE(c.start_date) = DATE($1)
    `

    rows, err := r.db.QueryContext(ctx, query, startDate)
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
            &campaign.Clicks,
            &campaign.CTR,
            &campaign.AdvertiserID,
            &campaign.ImpressionsBased,
            &campaign.TotalImpressions,
            &campaign.ServedImpressions,
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