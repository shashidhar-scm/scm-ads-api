package interfaces

import (
	"context"
	"scm/internal/models"
)

// AdvertiserRepository defines the interface for advertiser data operations
type AdvertiserRepository interface {
	Create(ctx context.Context, advertiser *models.Advertiser) error
	GetByID(ctx context.Context, id string) (*models.Advertiser, error)
	GetByIDInSet(ctx context.Context, id string, allowedIDs []string) (*models.Advertiser, error)
	List(ctx context.Context, limit int, offset int) ([]models.Advertiser, error)
	ListByIDs(ctx context.Context, allowedIDs []string, limit int, offset int) ([]models.Advertiser, error)
	Count(ctx context.Context) (int, error)
	CountByIDs(ctx context.Context, allowedIDs []string) (int, error)
	Search(ctx context.Context, term string, limit int, offset int) ([]models.Advertiser, int, error)
	SearchByIDs(ctx context.Context, allowedIDs []string, term string, limit int, offset int) ([]models.Advertiser, int, error)
	Update(ctx context.Context, id string, req *models.UpdateAdvertiserRequest) error
	Delete(ctx context.Context, id string) error
}
