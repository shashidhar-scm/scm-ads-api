package repository

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// LegacyRevisionRepository exposes read-only revision helpers.
type LegacyRevisionRepository interface {
	GetRevision(ctx context.Context, docType, region, docID string) (rev string, seq int64, err error)
	GetRegionUpdateSeq(ctx context.Context, docType, region string) (int64, error)
}

type legacyRevisionRepository struct {
	pool         *pgxpool.Pool
	schema       string
	queryTimeout time.Duration
}

func NewLegacyRevisionRepository(pool *pgxpool.Pool, schema string) LegacyRevisionRepository {
	return &legacyRevisionRepository{pool: pool, schema: strings.TrimSpace(schema), queryTimeout: 5 * time.Second}
}

func (r *legacyRevisionRepository) ensurePool() error {
	if r.pool == nil {
		return errors.New("legacy revision repository: pgx pool is nil")
	}
	return nil
}

func (r *legacyRevisionRepository) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, r.queryTimeout)
}

var identRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func mustIdent(s string) string {
	s = strings.TrimSpace(s)
	if !identRe.MatchString(s) {
		panic(fmt.Sprintf("invalid sql identifier: %q", s))
	}
	return s
}

func pqIdent(s string) string {
	return `"` + mustIdent(s) + `"`
}

func (r *legacyRevisionRepository) table(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if r.schema == "" {
		return pqIdent(name)
	}
	return pqIdent(r.schema) + "." + pqIdent(name)
}

func (r *legacyRevisionRepository) GetRegionUpdateSeq(ctx context.Context, docType, region string) (int64, error) {
	if err := r.ensurePool(); err != nil {
		return 0, err
	}
	docType = strings.TrimSpace(docType)
	region = strings.TrimSpace(region)
	if docType == "" || region == "" {
		return 0, errors.New("docType and region are required")
	}

	rowCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	var seq int64
	query := fmt.Sprintf(`SELECT update_seq FROM %s WHERE doc_type = $1 AND region = $2`, r.table("legacy_region_sequences"))
	err := r.pool.QueryRow(rowCtx, query, docType, region).Scan(&seq)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return seq, nil
}

func (r *legacyRevisionRepository) GetRevision(ctx context.Context, docType, region, docID string) (string, int64, error) {
	if err := r.ensurePool(); err != nil {
		return "", 0, err
	}
	docType = strings.TrimSpace(docType)
	region = strings.TrimSpace(region)
	docID = strings.TrimSpace(docID)
	if docType == "" || region == "" || docID == "" {
		return "", 0, errors.New("docType, region, and docID are required")
	}

	rowCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	var generation int
	var storedHash string
	query := fmt.Sprintf(`SELECT generation, hash_suffix FROM %s WHERE doc_type = $1 AND region = $2 AND doc_id = $3`, r.table("legacy_doc_revisions"))
	err := r.pool.QueryRow(rowCtx, query, docType, region, docID).Scan(&generation, &storedHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Document not tracked yet by replicator
			return "", 0, errors.New("revision not found")
		}
		return "", 0, err
	}

	// Return the stored revision from the replicator (source of truth)
	rev := fmt.Sprintf("%d-%s", generation, storedHash)
	seq, err := r.GetRegionUpdateSeq(ctx, docType, region)
	if err != nil {
		return rev, 0, nil
	}
	return rev, seq, nil
}
