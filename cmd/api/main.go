// cmd/api/main.go
package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    _ "github.com/lib/pq"
    "github.com/shashi/scm-ads-api/internal/config"
    "github.com/shashi/scm-ads-api/internal/db"
    "github.com/shashi/scm-ads-api/internal/db/migrations"
    "github.com/shashi/scm-ads-api/internal/routes"
)

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

    // Create router and setup routes
    router := routes.SetupRoutes(database.DB, cfg)

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

    // Give server 5 seconds to finish current requests
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := server.Shutdown(ctx); err != nil {
        log.Fatalf("Server forced to shutdown: %v", err)
    }

    log.Println("Server exiting")
}