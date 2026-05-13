package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// LegacyRevisionRepository tracks document revision generations per region.
type LegacyRevisionRepository interface {
	EnsureRevision(ctx context.Context, docType, region, docID, hashSuffix string) (rev string, seq int64, err error)
	GetRegionUpdateSeq(ctx context.Context, docType, region string) (int64, error)
}

type legacyRevisionRepository struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func NewLegacyRevisionRepository(pool *pgxpool.Pool) LegacyRevisionRepository {
	return &legacyRevisionRepository{pool: pool, queryTimeout: 5 * time.Second}
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

func (r *legacyRevisionRepository) EnsureRevision(ctx context.Context, docType, region, docID, hashSuffix string) (string, int64, error) {
	if err := r.ensurePool(); err != nil {
		return "", 0, err
	}
	docType = strings.TrimSpace(docType)
	region = strings.TrimSpace(region)
	docID = strings.TrimSpace(docID)
	hashSuffix = strings.TrimSpace(hashSuffix)
	if docType == "" || region == "" || docID == "" || hashSuffix == "" {
		return "", 0, errors.New("docType, region, docID, and hashSuffix are required")
	}

	execCtx, cancel := r.withTimeout(ctx)
	defer cancel()

	tx, err := r.pool.BeginTx(execCtx, pgx.TxOptions{})
	if err != nil {
		return "", 0, err
	}
	defer tx.Rollback(execCtx)

	var generation int
	var storedHash string
	err = tx.QueryRow(execCtx, `
        SELECT generation, hash_suffix
        FROM legacy_doc_revisions
        WHERE doc_type = $1 AND region = $2 AND doc_id = $3
    `, docType, region, docID).Scan(&generation, &storedHash)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return "", 0, err
		}
		generation = 0
	}

	changed := false
	if generation == 0 {
		generation = 1
		changed = true
	} else if storedHash != hashSuffix {
		generation++
		changed = true
	}

	if changed {
		if _, err := tx.Exec(execCtx, `
            INSERT INTO legacy_doc_revisions (doc_type, region, doc_id, generation, hash_suffix, updated_at)
            VALUES ($1, $2, $3, $4, $5, NOW())
            ON CONFLICT (doc_type, region, doc_id)
            DO UPDATE SET generation = EXCLUDED.generation, hash_suffix = EXCLUDED.hash_suffix, updated_at = NOW()
        `, docType, region, docID, generation, hashSuffix); err != nil {
			return "", 0, err
		}
	}

	seq := int64(0)
	if changed {
		if err := tx.QueryRow(execCtx, `
            INSERT INTO legacy_region_sequences (doc_type, region, update_seq)
            VALUES ($1, $2, 1)
            ON CONFLICT (doc_type, region)
            DO UPDATE SET update_seq = legacy_region_sequences.update_seq + 1
            RETURNING update_seq
        `, docType, region).Scan(&seq); err != nil {
			return "", 0, err
		}
	} else {
		if err := tx.QueryRow(execCtx, `
            SELECT update_seq FROM legacy_region_sequences WHERE doc_type = $1 AND region = $2
        `, docType, region).Scan(&seq); err != nil {
			if !errors.Is(err, pgx.ErrNoRows) {
				return "", 0, err
			}
			seq = 0
		}
	}

	if err := tx.Commit(execCtx); err != nil {
		return "", 0, err
	}

	rev := fmt.Sprintf("%d-%s", generation, hashSuffix)
	return rev, seq, nil
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
	err := r.pool.QueryRow(rowCtx, `
        SELECT update_seq FROM legacy_region_sequences WHERE doc_type = $1 AND region = $2
    `, docType, region).Scan(&seq)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return seq, nil
}
