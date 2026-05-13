// cmd/api/main.go
//
// @title SCM Ads API
// @version 1.0.0
// @description SCM Ads API documentation. Access is controlled via JWT + RBAC permissions. Super admins/admins have global access; advertiser users are scoped server-side to resources they own (via advertisers.created_by).
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter JWT token with 'Bearer ' prefix. Example: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
package main

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/rs/zerolog"

	"scm/internal/config"
	"scm/internal/db"
	"scm/internal/db/migrations"
	"scm/internal/logger"
	"scm/internal/platform/database"
	"scm/internal/repository"
	"scm/internal/routes"
	"scm/internal/services"
)

func getEnv(key, defaultValue string) string {
	v := os.Getenv(key)
	if strings.TrimSpace(v) == "" {
		return defaultValue
	}

	return v
}

func startPlaceExchangeTokenRefresher(
	ctx context.Context,
	cfg *config.Config,
	tokenRepo repository.PlaceExchangeTokenRepository,
	consoleClient *services.CityPostConsoleClient,
	placeClient *services.PlaceExchangeClient,
	logger *zerolog.Logger,
) {
	if tokenRepo == nil || consoleClient == nil || placeClient == nil {
		return
	}
	refreshTime := cfg.PlaceExchangeRefreshTime
	if strings.TrimSpace(refreshTime) == "" {
		refreshTime = "00:01"
	}
	hour, minute := parseHHMM(refreshTime, 0, 1)
	timezone := strings.TrimSpace(cfg.PlaceExchangeRefreshTZ)
	if timezone == "" {
		timezone = "UTC"
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		if logger != nil {
			logger.Warn().Err(err).Str("tz", timezone).Msg("Invalid PLACE_EXCHANGE_REFRESH_TZ, falling back to UTC")
		}
		loc = time.UTC
		timezone = "UTC"
	}

	go func() {
		runCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		err := refreshPlaceExchangeTokensOnce(runCtx, tokenRepo, consoleClient, placeClient, logger)
		cancel()
		if err != nil && logger != nil {
			logger.Warn().Err(err).Msg("Initial Place Exchange token refresh failed")
		}
	}()

	go func() {
		for {
			now := time.Now()
			runAt := nextRunAt(now, loc, hour, minute)
			delay := time.Until(runAt)

			t := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				t.Stop()
				return
			case <-t.C:
			}

			runCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
			err := refreshPlaceExchangeTokensOnce(runCtx, tokenRepo, consoleClient, placeClient, logger)
			cancel()
			if err != nil && logger != nil {
				logger.Warn().Err(err).Msg("Failed to refresh Place Exchange tokens")
			}
		}
	}()
}

func refreshPlaceExchangeTokensOnce(
	ctx context.Context,
	tokenRepo repository.PlaceExchangeTokenRepository,
	consoleClient *services.CityPostConsoleClient,
	placeClient *services.PlaceExchangeClient,
	logger *zerolog.Logger,
) error {
	token, err := placeClient.FetchToken(ctx)
	if err != nil {
		return err
	}
	cities, err := consoleClient.ListProductionProjectNames(ctx)
	if err != nil {
		return err
	}
	if len(cities) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(cities))
	updated := 0
	var lastErr error
	for _, city := range cities {
		c := strings.TrimSpace(city)
		if c == "" {
			continue
		}
		c = strings.ToLower(c)
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		if _, err := tokenRepo.Upsert(ctx, c, token); err != nil {
			lastErr = err
			if logger != nil {
				logger.Warn().Err(err).Str("city", c).Msg("Failed to upsert Place Exchange token")
			}
			continue
		}
		updated++
	}
	if logger != nil && updated > 0 {
		logger.Info().Int("cities", updated).Msg("Refreshed Place Exchange tokens")
	}
	return lastErr
}

func getEnvInt(key string, defaultValue int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return defaultValue
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return defaultValue
	}
	return i
}

func startPopImpressionsSync(ctx context.Context, db *sql.DB, popAPI services.PopAPI, logger *zerolog.Logger) {
	if db == nil || popAPI == nil {
		return
	}

	intervalSeconds := getEnvInt("POP_IMPRESSIONS_SYNC_INTERVAL_SECONDS", 300)
	if intervalSeconds <= 0 {
		intervalSeconds = 300
	}
	interval := time.Duration(intervalSeconds) * time.Second

	go func() {
		runCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		err := syncPopImpressionsOnce(runCtx, db, popAPI, logger)
		cancel()
		if err != nil {
			logger.Warn().Err(err).Msg("POP impressions sync failed")
		}

		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}

			runCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
			err := syncPopImpressionsOnce(runCtx, db, popAPI, logger)
			cancel()
			if err != nil {
				logger.Warn().Err(err).Msg("POP impressions sync failed")
			}
		}
	}()
}

func syncPopImpressionsOnce(ctx context.Context, db *sql.DB, popAPI services.PopAPI, logger *zerolog.Logger) error {
	rows, err := db.QueryContext(ctx, `
        SELECT id
        FROM campaigns
        WHERE status = 'active'
          AND impressions_based = true
    `)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var campaignID string
		if err := rows.Scan(&campaignID); err != nil {
			return err
		}
		campaignID = strings.TrimSpace(campaignID)
		if campaignID == "" {
			continue
		}

		imps, err := popAPI.CampaignImpressions(ctx, campaignID)
		if err != nil {
			if logger != nil {
				logger.Warn().Err(err).Str("campaign_id", campaignID).Msg("POP impressions fetch failed")
			}
			continue
		}
		if imps == nil {
			continue
		}

		for _, p := range imps.Posters {
			creativeID := strings.TrimSpace(p.PosterID)
			if creativeID == "" {
				continue
			}
			if _, err := db.ExecContext(ctx, `
                UPDATE creatives
                SET impressions_served = GREATEST(impressions_served, $2)
                WHERE id = $1
            `, creativeID, p.Impressions); err != nil {
				if logger != nil {
					logger.Warn().Err(err).
						Str("creative_id", creativeID).
						Str("campaign_id", campaignID).
						Msg("POP impressions update failed")
				}
				continue
			}
		}
	}
	return rows.Err()
}

func startScheduledCampaignCompleter(ctx context.Context, campaignRepo interface {
	CompleteActiveEndedBefore(ctx context.Context, now time.Time, activeStatus string, completedStatus string, timeZone string) (int64, error)
}, notifier *services.CampaignNotifier, logger *zerolog.Logger) {
	tzName := getEnv("CAMPAIGN_SCHEDULER_TZ", "UTC")
	activeStatus := getEnv("CAMPAIGN_ACTIVE_STATUS", "active")
	completedStatus := getEnv("CAMPAIGN_COMPLETED_STATUS", "completed")
	hhmm := getEnv("CAMPAIGN_COMPLETER_TIME", "00:02")
	hour, minute := parseHHMM(hhmm, 0, 2)

	loc, err := time.LoadLocation(tzName)
	if err != nil {
		if logger != nil {
			logger.Warn().Err(err).Str("tz", tzName).Msg("Invalid CAMPAIGN_SCHEDULER_TZ, falling back to UTC")
		}
		tzName = "UTC"
		loc = time.UTC
	}

	go func() {
		for {
			now := time.Now()
			runAt := nextRunAt(now, loc, hour, minute)
			delay := time.Until(runAt)

			t := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				t.Stop()
				return
			case <-t.C:
			}

			runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			now = time.Now()
			rows, err := campaignRepo.CompleteActiveEndedBefore(runCtx, now, activeStatus, completedStatus, tzName)
			cancel()
			if err != nil {
				if logger != nil {
					logger.Warn().Err(err).
						Str("active_status", activeStatus).
						Str("completed_status", completedStatus).
						Str("tz", tzName).
						Msg("Failed to complete ended campaigns")
				}
				continue
			}
			if rows > 0 {
				if logger != nil {
					logger.Info().
						Int64("rows", rows).
						Str("active_status", activeStatus).
						Str("completed_status", completedStatus).
						Str("tz", tzName).
						Msg("Completed ended campaigns")
				}
				if notifier != nil {
					if err := notifier.SendCompletionNotifications(ctx, now); err != nil {
						if logger != nil {
							logger.Warn().Err(err).Msg("Failed to send completion notifications")
						}
					}
				}
			}
		}
	}()
}

func parseHHMM(s string, defaultHour int, defaultMinute int) (int, int) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return defaultHour, defaultMinute
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil {
		return defaultHour, defaultMinute
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil {
		return defaultHour, defaultMinute
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return defaultHour, defaultMinute
	}
	return h, m
}

func nextRunAt(now time.Time, loc *time.Location, hour int, minute int) time.Time {
	localNow := now.In(loc)
	run := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), hour, minute, 0, 0, loc)
	if !run.After(localNow) {
		run = run.Add(24 * time.Hour)
	}
	return run
}

func startCampaignNotificationDispatcher(ctx context.Context, notifier *services.CampaignNotifier, logger *zerolog.Logger) {
	tzName := getEnv("CAMPAIGN_SCHEDULER_TZ", "UTC")
	hhmm := getEnv("CAMPAIGN_NOTIFICATION_TIME", "00:03")
	hour, minute := parseHHMM(hhmm, 0, 3)

	loc, err := time.LoadLocation(tzName)
	if err != nil {
		if logger != nil {
			logger.Warn().Err(err).Str("tz", tzName).Msg("Invalid CAMPAIGN_SCHEDULER_TZ, falling back to UTC")
		}
		tzName = "UTC"
		loc = time.UTC
	}

	go func() {
		for {
			now := time.Now()
			runAt := nextRunAt(now, loc, hour, minute)
			delay := time.Until(runAt)

			t := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				t.Stop()
				return
			case <-t.C:
			}

			runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			if err := notifier.SendActivationNotifications(runCtx); err != nil {
				if logger != nil {
					logger.Warn().Err(err).Str("tz", tzName).Msg("Failed to send campaign notifications")
				}
			}
			cancel()
		}
	}()
}

func startScheduledCampaignActivator(ctx context.Context, campaignRepo interface {
	ActivateScheduledStartingOn(ctx context.Context, startDate time.Time, scheduledStatus string, timeZone string) (int64, error)
}, logger *zerolog.Logger) {
	tzName := getEnv("CAMPAIGN_SCHEDULER_TZ", "UTC")
	scheduledStatus := getEnv("CAMPAIGN_SCHEDULED_STATUS", "scheduled")
	hhmm := getEnv("CAMPAIGN_SCHEDULER_TIME", "00:01")
	hour, minute := parseHHMM(hhmm, 0, 1)

	loc, err := time.LoadLocation(tzName)
	if err != nil {
		if logger != nil {
			logger.Warn().Err(err).Str("tz", tzName).Msg("Invalid CAMPAIGN_SCHEDULER_TZ, falling back to UTC")
		}
		tzName = "UTC"
		loc = time.UTC
	}

	go func() {
		for {
			now := time.Now()
			runAt := nextRunAt(now, loc, hour, minute)
			delay := time.Until(runAt)

			t := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				t.Stop()
				return
			case <-t.C:
			}

			runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			rows, err := campaignRepo.ActivateScheduledStartingOn(runCtx, time.Now(), scheduledStatus, tzName)
			cancel()
			if err != nil {
				if logger != nil {
					logger.Warn().Err(err).
						Str("status", scheduledStatus).
						Str("tz", tzName).
						Msg("Failed to activate scheduled campaigns")
				}
				continue
			}
			if rows > 0 {
				if logger != nil {
					logger.Info().
						Int64("rows", rows).
						Str("status", scheduledStatus).
						Str("tz", tzName).
						Msg("Activated scheduled campaigns")
				}
			}
		}
	}()
}

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	appLogger := logger.New(cfg)

	// Initialize pgx pool (will gradually replace database/sql usages)
	poolCfg := database.PoolConfigFromApp(cfg)
	var (
		poolCtx    context.Context
		cancelPool context.CancelFunc
	)
	if poolCfg.ConnectTimeout > 0 {
		poolCtx, cancelPool = context.WithTimeout(context.Background(), poolCfg.ConnectTimeout)
	} else {
		poolCtx, cancelPool = context.WithCancel(context.Background())
	}
	defer cancelPool()
	pgxPool, err := database.NewPool(poolCtx, cfg.DatabaseURL, poolCfg)
	if err != nil {
		appLogger.Fatal().Err(err).Msg("Failed to initialize pgx pool")
	}
	defer pgxPool.Close()

	campaignRepo := repository.NewCampaignRepository(pgxPool)
	creativeRepo := repository.NewCreativeRepository(pgxPool)
	placeExchangeTokenRepo := repository.NewPlaceExchangeTokenRepository(pgxPool)
	legacyRevisionRepo := repository.NewLegacyRevisionRepository(pgxPool)

	// Create database if it doesn't exist
	if err := db.CreateDatabaseIfNotExists(cfg.DatabaseURL); err != nil {
		appLogger.Fatal().Err(err).Msg("Failed to ensure database exists")
	}

	// Initialize database
	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		appLogger.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer database.Close()

	var replicatorDB *sql.DB
	if strings.TrimSpace(cfg.ReplicatorDatabaseURL) != "" {
		rdb, err := db.New(cfg.ReplicatorDatabaseURL)
		if err != nil {
			appLogger.Fatal().Err(err).Msg("Failed to connect to replicator database")
		}
		replicatorDB = rdb.DB
		defer rdb.Close()
	}

	// Run database migrations
	if err := migrations.RunMigrations(database.DB); err != nil {
		appLogger.Fatal().Err(err).Msg("Failed to run migrations")
	}

	// Background jobs
	jobsCtx, cancelJobs := context.WithCancel(context.Background())
	defer cancelJobs()
	userRepo := repository.NewUserRepository(pgxPool)
	emailSender := services.NewSMTPSender(
		cfg.SMTPHost,
		cfg.SMTPPort,
		cfg.SMTPUser,
		cfg.SMTPPassword,
		cfg.SMTPFrom,
		cfg.SMTPUseTLS,
	)
	notifier := services.NewCampaignNotifier(campaignRepo, userRepo, emailSender)
	citypostClient := services.NewCityPostConsoleClient(
		cfg.CityPostConsoleBaseURL,
		cfg.CityPostConsoleUsername,
		cfg.CityPostConsolePassword,
	)
	citypostClient.SetAuthScheme(cfg.CityPostConsoleAuthScheme)
	placeExchangeClient := services.NewPlaceExchangeClient(
		cfg.PlaceExchangeURL,
		cfg.PlaceExchangeUsername,
		cfg.PlaceExchangePassword,
	)
	startScheduledCampaignActivator(jobsCtx, campaignRepo, appLogger)
	startScheduledCampaignCompleter(jobsCtx, campaignRepo, notifier, appLogger)
	startCampaignNotificationDispatcher(jobsCtx, notifier, appLogger)
	startPlaceExchangeTokenRefresher(jobsCtx, cfg, placeExchangeTokenRepo, citypostClient, placeExchangeClient, appLogger)

	popClient := services.NewPopClient(cfg.PopAPIBaseURL)
	startPopImpressionsSync(jobsCtx, database.DB, popClient, appLogger)

	// Initialize S3 configuration
	s3Config, err := config.NewS3Config()
	if err != nil {
		appLogger.Fatal().Err(err).Msg("Failed to create S3 client")
	}

	// Create router and setup routes
	advertiserRepo := repository.NewAdvertiserRepository(pgxPool)
	deviceRepo := repository.NewDeviceRepository(pgxPool)
	projectRepo := repository.NewProjectRepository(pgxPool)
	roleRepo := repository.NewRoleRepository(pgxPool)
	permissionRepo := repository.NewPermissionRepository(pgxPool)
	userRoleRepo := repository.NewUserRoleRepository(pgxPool)
	router, err := routes.SetupRoutes(
		database.DB,
		replicatorDB,
		cfg,
		s3Config,
		campaignRepo,
		creativeRepo,
		userRepo,
		advertiserRepo,
		deviceRepo,
		projectRepo,
		roleRepo,
		permissionRepo,
		userRoleRepo,
		placeExchangeTokenRepo,
		legacyRevisionRepo,
	)
	if err != nil {
		appLogger.Fatal().Err(err).Msg("Failed to setup routes")
	}

	// Create server
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	// Graceful shutdown
	go func() {
		appLogger.Info().Str("port", cfg.Port).Msg("Server starting")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.Fatal().Err(err).Msg("Failed to start server")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	appLogger.Info().Msg("Shutting down server...")

	cancelJobs()

	// Give server 5 seconds to finish current requests
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		appLogger.Fatal().Err(err).Msg("Server forced to shutdown")
	}

	appLogger.Info().Msg("Server exiting")
}
