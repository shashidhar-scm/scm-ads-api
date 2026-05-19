package routes

import (
	"context"

	"scm/internal/models"
)

type noopPlaceExchangeTokenRepo struct{}

func (n *noopPlaceExchangeTokenRepo) Upsert(ctx context.Context, city, token string) (*models.PlaceExchangeToken, error) {
	return &models.PlaceExchangeToken{DocID: city + "-access-token", City: city, Token: token}, nil
}

func (n *noopPlaceExchangeTokenRepo) GetByDocID(ctx context.Context, docID string) (*models.PlaceExchangeToken, error) {
	return nil, nil
}

func (n *noopPlaceExchangeTokenRepo) GetByCity(ctx context.Context, city string) (*models.PlaceExchangeToken, error) {
	return nil, nil
}

type noopLegacyRevisionRepo struct{}

func (n *noopLegacyRevisionRepo) GetRegionUpdateSeq(ctx context.Context, docType, region string) (int64, error) {
	return 0, nil
}

func (n *noopLegacyRevisionRepo) GetRevision(ctx context.Context, docType, region, docID string) (string, int64, error) {
	return "", 0, nil
}
