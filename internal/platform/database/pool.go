package database

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"scm/internal/config"
)

// PoolConfig captures pgx pool tuning knobs.
type PoolConfig struct {
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
	ConnectTimeout    time.Duration
}

// NewPool builds a pgx pool from the provided connection string and tuning config.
func NewPool(ctx context.Context, connString string, cfg PoolConfig) (*pgxpool.Pool, error) {
	baseCfg, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, err
	}

	if cfg.MaxConns > 0 {
		baseCfg.MaxConns = cfg.MaxConns
	}
	if cfg.MinConns >= 0 {
		baseCfg.MinConns = cfg.MinConns
	}
	if cfg.MaxConnLifetime > 0 {
		baseCfg.MaxConnLifetime = cfg.MaxConnLifetime
	}
	if cfg.MaxConnIdleTime > 0 {
		baseCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	}
	if cfg.HealthCheckPeriod > 0 {
		baseCfg.HealthCheckPeriod = cfg.HealthCheckPeriod
	}
	if cfg.ConnectTimeout > 0 {
		baseCfg.ConnConfig.ConnectTimeout = cfg.ConnectTimeout
	}

	return pgxpool.NewWithConfig(ctx, baseCfg)
}

// PoolConfigFromApp builds a PoolConfig from high-level application config.
func PoolConfigFromApp(cfg *config.Config) PoolConfig {
	if cfg == nil {
		return PoolConfig{}
	}
	return PoolConfig{
		MaxConns:          int32(cfg.DBMaxConnections),
		MinConns:          int32(cfg.DBMinConnections),
		MaxConnLifetime:   time.Duration(cfg.DBMaxConnLifetimeMin) * time.Minute,
		MaxConnIdleTime:   time.Duration(cfg.DBMaxConnIdleMin) * time.Minute,
		HealthCheckPeriod: time.Duration(cfg.DBHealthCheckSeconds) * time.Second,
		ConnectTimeout:    time.Duration(cfg.DBConnectTimeoutSec) * time.Second,
	}
}
