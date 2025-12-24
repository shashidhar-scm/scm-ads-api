// cmd/api/main.go
//
// @title SCM Ads API
// @version 1.0.0
// @description SCM Ads API documentation
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter JWT token with 'Bearer ' prefix. Example: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "strconv"
    "strings"
    "syscall"
    "time"

    _ "github.com/lib/pq"
    "scm/internal/config"
    "scm/internal/db"
    "scm/internal/db/migrations"
    "scm/internal/repository"
    "scm/internal/routes"
)

func getEnv(key, defaultValue string) string {
    v := os.Getenv(key)
    if strings.TrimSpace(v) == "" {
        return defaultValue
    }

    return v
}

func startScheduledCampaignCompleter(ctx context.Context, campaignRepo interface {
	CompleteActiveEndedBefore(ctx context.Context, now time.Time, activeStatus string, completedStatus string, timeZone string) (int64, error)
}) {
	tzName := getEnv("CAMPAIGN_SCHEDULER_TZ", "UTC")
	activeStatus := getEnv("CAMPAIGN_ACTIVE_STATUS", "active")
	completedStatus := getEnv("CAMPAIGN_COMPLETED_STATUS", "completed")
	hhmm := getEnv("CAMPAIGN_COMPLETER_TIME", "00:02")
	hour, minute := parseHHMM(hhmm, 0, 2)

	loc, err := time.LoadLocation(tzName)
	if err != nil {
		log.Printf("Invalid CAMPAIGN_SCHEDULER_TZ=%q, falling back to UTC: %v", tzName, err)
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
			rows, err := campaignRepo.CompleteActiveEndedBefore(runCtx, time.Now(), activeStatus, completedStatus, tzName)
			cancel()
			if err != nil {
				log.Printf("Failed to complete ended campaigns (active=%s completed=%s tz=%s): %v", activeStatus, completedStatus, tzName, err)
				continue
			}
			if rows > 0 {
				log.Printf("Completed %d campaign(s) (active=%s completed=%s tz=%s)", rows, activeStatus, completedStatus, tzName)
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

func startScheduledCampaignActivator(ctx context.Context, campaignRepo interface {
    ActivateScheduledStartingOn(ctx context.Context, startDate time.Time, scheduledStatus string, timeZone string) (int64, error)
}) {
    tzName := getEnv("CAMPAIGN_SCHEDULER_TZ", "UTC")
    scheduledStatus := getEnv("CAMPAIGN_SCHEDULED_STATUS", "scheduled")
    hhmm := getEnv("CAMPAIGN_SCHEDULER_TIME", "00:01")
    hour, minute := parseHHMM(hhmm, 0, 1)

    loc, err := time.LoadLocation(tzName)
    if err != nil {
        log.Printf("Invalid CAMPAIGN_SCHEDULER_TZ=%q, falling back to UTC: %v", tzName, err)
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
                log.Printf("Failed to activate scheduled campaigns (status=%s tz=%s): %v", scheduledStatus, tzName, err)
                continue
            }
            if rows > 0 {
                log.Printf("Activated %d scheduled campaign(s) for today (status=%s tz=%s)", rows, scheduledStatus, tzName)
            }
        }
    }()
}

func main() {
    // Load configuration
    cfg := config.Load()

    // Create database if it doesn't exist
    if err := db.CreateDatabaseIfNotExists(cfg.DatabaseURL); err != nil {
        log.Fatalf("Failed to ensure database exists: %v", err)
    }

    // Initialize database
    database, err := db.New(cfg.DatabaseURL)
    if err != nil {
        log.Fatalf("Failed to connect to database: %v", err)
    }
    defer database.Close()

    // Run database migrations
    if err := migrations.RunMigrations(database.DB); err != nil {
        log.Fatalf("Failed to run migrations: %v", err)
    }

    // Background jobs
    jobsCtx, cancelJobs := context.WithCancel(context.Background())
    defer cancelJobs()
    campaignRepo := repository.NewCampaignRepository(database.DB)
    startScheduledCampaignActivator(jobsCtx, campaignRepo)
	startScheduledCampaignCompleter(jobsCtx, campaignRepo)

	// Initialize S3 configuration
    s3Config, err := config.NewS3Config()
    if err != nil {
        log.Fatalf("Failed to create S3 client: %v", err)
    }

    // Create router and setup routes
    router := routes.SetupRoutes(database.DB, cfg, s3Config)

    // Create server
    server := &http.Server{
        Addr:    ":" + cfg.Port,
        Handler: router,
    }

    // Graceful shutdown
    go func() {
        log.Printf("Server starting on port %s", cfg.Port)
        if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("Failed to start server: %v", err)
        }
    }()

    // Wait for interrupt signal
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    log.Println("Shutting down server...")

    cancelJobs()

    // Give server 5 seconds to finish current requests
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := server.Shutdown(ctx); err != nil {
        log.Fatalf("Server forced to shutdown: %v", err)
    }

    log.Println("Server exiting")
}