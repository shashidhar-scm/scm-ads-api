// internal/interfaces/campaign_repository.go
package interfaces

import (
    "context"
    "time"
    "scm/internal/models"
)

// CampaignFilter defines the filter criteria for listing campaigns
type CampaignFilter struct {
    AdvertiserID string
    Status       string
    StartDate    time.Time
    EndDate      time.Time
    Limit        int
    Offset       int
}

// CampaignRepository defines the interface for campaign data operations
type CampaignRepository interface {
    Create(ctx context.Context, campaign *models.Campaign) error
    GetByID(ctx context.Context, id string) (*models.Campaign, error)
    List(ctx context.Context, filter CampaignFilter) ([]*models.Campaign, error)
    Summary(ctx context.Context, filter CampaignFilter) (*models.CampaignSummary, error)
    ActivateScheduledStartingOn(ctx context.Context, startDate time.Time, scheduledStatus string, timeZone string) (int64, error)
    CompleteActiveEndedBefore(ctx context.Context, now time.Time, activeStatus string, completedStatus string, timeZone string) (int64, error)
    Update(ctx context.Context, id string, campaign *models.Campaign) error
    Delete(ctx context.Context, id string) error
}