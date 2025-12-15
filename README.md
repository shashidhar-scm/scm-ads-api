# SCM Ads API

A high-performance advertising API service built with Go.

## API Conventions

- All endpoints return **JSON** (including errors).
- Error responses follow:
  ```json
  {"error":"some_code","message":"Human readable message"}
  ```
- Success responses either return a resource JSON object/array, or:
  ```json
  {"message":"..."}
  ```

## Getting Started

### Prerequisites

- Go (version from `go.mod`)
- PostgreSQL

### Installation

1. Clone the repository
2. Run the application:
   ```bash
   go run ./cmd/api
   ```

The server starts on `http://localhost:8080` by default.

### Health

- `GET /` -> `{ "message": "Application Up and running" }`
- `GET /health` -> includes PostgreSQL connectivity:
  ```json
  {"status":"ok","db":{"status":"ok"}}
  ```

## Project Structure

```
scm-ads-api/
├── cmd/
│   └── api/
│       └── main.go          # Application entry point
├── internal/
│   ├── config/             # Configuration management
│   ├── handlers/           # HTTP request handlers
│   ├── middleware/         # HTTP middleware
│   ├── models/             # Data models
│   ├── repository/         # Database operations
│   ├── routes/             # Route definitions
│   ├── services/           # Business logic
│   └── utils/              # Helper functions
└── go.mod                 # Go module definition
```

## Environment Variables

- `PORT`: Port to run the server on (default: `8080`)
- `ENVIRONMENT`: `development`/`production` (default: `development`)

### PostgreSQL

- `DATABASE_URL`: Full Postgres URL (overrides everything)
- `PSQL_HOST` (default: `localhost`)
- `PSQL_PORT` (default: `5432`)
- `PSQL_USER` (default: `postgres`)
- `PSQL_PASSWORD` (default: `postgres`)
- `PSQL_DB_NAME` (default: `scm_ads`)

### Auth

- `JWT_SECRET` (default: `dev-secret`)
- `JWT_EXPIRES_IN_SECONDS` (default: `86400`)
- `AUTH_VERBOSE_ERRORS` (default: `false`)
- `AUTH_RETURN_RESET_TOKEN` (default: `false`)

### SMTP (Forgot/Reset password)

- `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`, `SMTP_FROM`
- `SMTP_USE_TLS` (default: `false`)

### S3 (Creatives)

- `AWS_REGION` (default: `us-east-1`)
- `S3_BUCKET_NAME` (default: `scm-ads`)
- `CREATIVE_PUBLIC_BASE_URL` (default: `https://scm-ads-posters.citypost.us/`)
- `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` (optional; otherwise AWS SDK default chain)

## Database Migrations

Migrations are in `./migrations` and run automatically on startup.

Key schema notes:
- `campaigns.cities` is a `TEXT[]`
- `creatives` stores assignment fields directly:
  - `campaign_id`
  - `selected_days` (TEXT[])
  - `time_slots` (TEXT[])
  - `devices` (TEXT[])

## Key Endpoints

### Auth

- `POST /api/v1/auth/signup`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/forgot-password`
- `POST /api/v1/auth/reset-password`

### Users

- `GET /api/v1/users/`
- `GET /api/v1/users/{id}`
- `PUT /api/v1/users/{id}`
- `PUT /api/v1/users/{id}/password`
- `DELETE /api/v1/users/{id}`

### Advertisers (JWT-protected)

- `GET /api/v1/advertisers/`
- `POST /api/v1/advertisers/`
- `GET /api/v1/advertisers/{id}`
- `PUT /api/v1/advertisers/{id}`
- `DELETE /api/v1/advertisers/{id}`

### Campaigns (JWT-protected)

- `GET /api/v1/campaigns/`
- `POST /api/v1/campaigns/` (supports `cities: []string`)
- `GET /api/v1/campaigns/{id}`
- `PUT /api/v1/campaigns/{id}`
- `DELETE /api/v1/campaigns/{id}`

### Creatives (JWT-protected)

- `GET /api/v1/creatives/`
- `GET /api/v1/creatives/campaign/{campaignID}`
- `POST /api/v1/creatives/upload` (multipart/form-data)

Upload required fields:
- `campaign_id`
- `selected_days` (comma-separated or repeated form field)
- `time_slots` (comma-separated or repeated form field)
- `devices` (optional)
- `files` (one or more files)

## Development

### Running Tests

```bash
go mod tidy
go test ./...
```

### Code Formatting

```bash
gofmt -w .
```
