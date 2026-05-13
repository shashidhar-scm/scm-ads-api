package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"scm/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrPlaceExchangeTokenNotFound indicates the requested Place Exchange token document does not exist.
var ErrPlaceExchangeTokenNotFound = errors.New("place exchange token not found")

// PlaceExchangeTokenRepository manages CRUD operations for Place Exchange tokens.
type PlaceExchangeTokenRepository interface {
	Upsert(ctx context.Context, city, token string) (*models.PlaceExchangeToken, error)
	GetByDocID(ctx context.Context, docID string) (*models.PlaceExchangeToken, error)
	GetByCity(ctx context.Context, city string) (*models.PlaceExchangeToken, error)
}

type placeExchangeTokenRepository struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

// NewPlaceExchangeTokenRepository creates a new repository backed by the provided pgx pool.
func NewPlaceExchangeTokenRepository(pool *pgxpool.Pool) PlaceExchangeTokenRepository {
	return &placeExchangeTokenRepository{pool: pool, queryTimeout: 5 * time.Second}
}

func (r *placeExchangeTokenRepository) ensurePool() error {
	if r.pool == nil {
		return errors.New("place exchange token repository: pgx pool is nil")
	}
	return nil
}

func (r *placeExchangeTokenRepository) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, r.queryTimeout)
}

func normalizeCity(city string) string {
	return strings.ToLower(strings.TrimSpace(city))
}

func docIDFromCity(city string) string {
	city = normalizeCity(city)
	if city == "" {
		return ""
	}
	return fmt.Sprintf("%s-access-token", city)
}

func (r *placeExchangeTokenRepository) Upsert(ctx context.Context, city, token string) (*models.PlaceExchangeToken, error) {
	if err := r.ensurePool(); err != nil {
		return nil, err
	}

	city = normalizeCity(city)
	token = strings.TrimSpace(token)
	if city == "" {
		return nil, errors.New("city is required")
	}
	if token == "" {
		return nil, errors.New("token is required")
	}

	docID := docIDFromCity(city)
	query := `
        INSERT INTO place_exchange_tokens (doc_id, city, token)
        VALUES ($1, $2, $3)
        ON CONFLICT (doc_id) DO UPDATE
        SET token = EXCLUDED.token,
            city = EXCLUDED.city,
            updated_at = NOW()
        RETURNING doc_id, city, token, created_at, updated_at
    `

	execCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	var tokenDoc models.PlaceExchangeToken
	if err := r.pool.QueryRow(execCtx, query, docID, city, token).Scan(
		&tokenDoc.DocID,
		&tokenDoc.City,
		&tokenDoc.Token,
		&tokenDoc.CreatedAt,
		&tokenDoc.UpdatedAt,
	); err != nil {
		return nil, err
	}

	return &tokenDoc, nil
}

func (r *placeExchangeTokenRepository) GetByDocID(ctx context.Context, docID string) (*models.PlaceExchangeToken, error) {
	if err := r.ensurePool(); err != nil {
		return nil, err
	}
	docID = strings.ToLower(strings.TrimSpace(docID))
	if docID == "" {
		return nil, ErrPlaceExchangeTokenNotFound
	}

	query := `SELECT doc_id, city, token, created_at, updated_at FROM place_exchange_tokens WHERE doc_id = $1`

	rowCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	var tokenDoc models.PlaceExchangeToken
	if err := r.pool.QueryRow(rowCtx, query, docID).Scan(
		&tokenDoc.DocID,
		&tokenDoc.City,
		&tokenDoc.Token,
		&tokenDoc.CreatedAt,
		&tokenDoc.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPlaceExchangeTokenNotFound
		}
		return nil, err
	}

	return &tokenDoc, nil
}

func (r *placeExchangeTokenRepository) GetByCity(ctx context.Context, city string) (*models.PlaceExchangeToken, error) {
	docID := docIDFromCity(city)
	if docID == "" {
		return nil, ErrPlaceExchangeTokenNotFound
	}
	return r.GetByDocID(ctx, docID)
}
