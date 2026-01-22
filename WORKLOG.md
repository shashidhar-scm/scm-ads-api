# SCM Ads API — Work Log

## 2025-12-24

### Context switch
- Project: `scm-ads-api`
- Goal: maintain a running log of changes/decisions for future reference.

### Codebase map (high level)
- Entrypoint: `cmd/api/main.go`
  - Loads config (`internal/config`)
  - Ensures DB exists, connects (`internal/db`)
  - Runs migrations (`internal/db/migrations`)
  - Starts background campaign jobs (activator + completer)
  - Initializes S3 client (`internal/config/s3.go`)
  - Registers routes (`internal/routes/routes.go`)
- Routes: `internal/routes/routes.go`
  - `GET /` basic status
  - `GET /health` DB ping
  - `GET /api/v1/debug/env` sanitized config/env dump
  - `Route /api/v1`
    - Public: auth, users, public creatives
    - Protected (JWT): campaigns, advertisers, creatives, devices

### Internal package layout (what lives where)
- `internal/config`
  - `config.go`: builds `Config` from env (DB URL, JWT, SMTP, CityPost console)
  - `s3.go`: builds `S3Config` (AWS region, bucket, public base URL)
- `internal/db`
  - `database.go`: `db.New()` opens postgres + `Ping()`
  - `CreateDatabaseIfNotExists(...)` is called from `main.go` (defined elsewhere under `internal/db`)
- `internal/routes`
  - `routes.go`: Chi router + middleware + registers all route groups
  - `*_routes.go`: per-domain route registration (auth/users/campaigns/advertisers/creatives/devices)
  - `swagger_routes.go`: `/swagger/*` endpoints via `http-swagger`
- `internal/handlers`
  - Request/response handlers per domain: auth, users, campaigns, advertisers, creatives, devices
  - `json_response.go`: pagination helpers + error helpers
- `internal/repository`
  - DB access layer (Postgres queries) per domain: users, advertisers, campaigns, creatives, password reset tokens
- `internal/models`
  - Domain structs + request/response DTOs (e.g. `Campaign`, `Creative`, `User`, `PasswordResetToken`)
- `internal/middleware`
  - `jwt_auth.go`: JWT validation middleware, attaches `CtxUserID` + `CtxEmail` to request context
- `internal/services`
  - `smtp_email_sender.go`: SMTP implementation of `EmailSender` used by auth flows
  - `citypost_console_client.go`: CityPost Console API client used by device listing
  - `email_sender.go`: interface `EmailSender`

### Key flows

#### Auth + JWT
- Routes: `POST /api/v1/auth/signup`, `/login`, `/forgot-password`, `/reset-password`
- Handler: `internal/handlers/auth_handler.go`
- Passwords: stored as bcrypt hash.
- JWT:
  - Signed with `HS256` using `cfg.JWTSecret`
  - Claims include `sub` (user ID), `email`, `iat`, `exp`
- Protected routes use `internal/middleware/jwt_auth.go`.
  - Reads `Authorization: Bearer <token>`
  - On success stores `user_id` and `email` in request context.

#### Campaigns
- Protected under `/api/v1/campaigns`
- Status values come from `internal/models/campaign.go`:
  - `draft`, `active`, `paused`, `scheduled`, `completed`
- Background jobs (from `cmd/api/main.go`):
  - Activator: moves scheduled campaigns to active based on scheduler time + timezone.
  - Completer: marks active campaigns as completed when ended.
  - Env knobs: `CAMPAIGN_SCHEDULER_TZ`, `CAMPAIGN_SCHEDULER_TIME`, `CAMPAIGN_COMPLETER_TIME`, plus status override envs.

#### Creatives + S3
- Protected routes under `/api/v1/creatives`
  - Upload: `POST /api/v1/creatives/upload` (multipart)
  - List by campaign: `GET /api/v1/creatives/campaign/{campaignID}`
- Public route:
  - `GET /api/v1/creatives/device/{device}` (serves creatives for a device)
- Handler: `internal/handlers/creative_handler.go`
- Upload behavior:
  - Validates `campaign_id` exists via campaign repository.
  - Uploads files to S3 under `creatives/<creativeID><ext>`.
  - Stores both `url` (public) + `file_path` (internal S3 object key) in DB.
- S3 config:
  - `AWS_REGION`, `S3_BUCKET_NAME`, `CREATIVE_PUBLIC_BASE_URL`
  - Credentials: uses env credentials if present; otherwise AWS default chain.

#### Devices (CityPost Console)
- Protected route: `GET /api/v1/devices`
- Handler: `internal/handlers/device_handler.go`
- Upstream: `internal/services/citypost_console_client.go`
  - Logs in at `POST <baseURL>/login/`
  - Fetches devices at `GET <baseURL>/device/?project=<p>&region=<r>`
  - Supports configurable auth scheme (default `Bearer`) + token caching.
- Request params:
  - `target=project:region` can be repeated, OR use `project` + `region`.

### Response conventions
- Pagination helper: `internal/handlers/json_response.go`
  - Query params: `page`, `page_size`
  - Response shape:
    - `{"data": <...>, "pagination": {"page":..., "page_size":..., "total":..., "total_pages":...}}`
- Error shape in many handlers:
  - `{"error": <code>, "message": <message>}`

### Important env/config knobs
- Server:
  - `PORT`, `ENVIRONMENT`, `CORS_ALLOWED_ORIGINS`
- Postgres:
  - `DATABASE_URL` (preferred)
  - Or: `PSQL_HOST`, `PSQL_PORT`, `PSQL_USER`, `PSQL_PASSWORD`, `PSQL_DB_NAME`
- JWT:
  - `JWT_SECRET`, `JWT_EXPIRES_IN_SECONDS`
- Auth behavior:
  - `AUTH_VERBOSE_ERRORS`, `AUTH_RETURN_RESET_TOKEN`, `AUTH_RESET_PASSWORD_URL`
- SMTP:
  - `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`, `SMTP_FROM`, `SMTP_USE_TLS`
- S3 / creatives:
  - `AWS_REGION`, `S3_BUCKET_NAME`, `CREATIVE_PUBLIC_BASE_URL`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`
- CityPost console (devices):
  - `CITYPOST_CONSOLE_BASE_URL`, `CITYPOST_CONSOLE_USERNAME`, `CITYPOST_CONSOLE_PASSWORD`, `CITYPOST_CONSOLE_AUTH_SCHEME`

### Console API sync feature (added 2025-12-24)
- **Route:** `POST /api/v1/sync/console` (JWT-protected)
- **Flow:**
  1. Fetch projects from `https://consoleapi.citypost.us/scm-cloud/projectsList?production=true` and `...?production=false`
  2. Merge and upsert projects into `projects` table (unique on `name`)
  3. Login to console API (reuse `CityPostConsoleClient.login` token)
  4. For each project, fetch devices from `https://consoleapi.citypost.us/scm-cloud/device/?project={{project.name}}`
  5. Upsert devices into `devices` table (unique on `host_name`)
- **Models:**
  - `Project`: mirrors console API fields; nested objects stored as JSONB (`owner`, `languages`, `region`)
  - `Device`: mirrors console API fields; nested objects stored as JSONB (`device_type`, `region`, `device_config`)
- **Repositories:** `ProjectRepository` and `DeviceRepository` with `Upsert`, `GetByName`/`GetByHostName`, `List`, `Count`
- **CityPostConsoleClient extensions:** `ListProjects`, `ListDevicesByProject`
- **Handler:** `SyncHandler.SyncConsole` orchestrates the sync and returns counts and errors
- **Migration:** `0006_projects_devices_schema.up.sql` creates `projects` and `devices` tables with JSONB columns and indexes
- **Response shape:** `{"synced": {"projects": int, "devices": int}, "errors": []string}`

### Next
- Add read routes for projects and devices (list/get) if needed.
- Test the sync endpoint manually or via integration test.

## Recent Updates (Dec 24, 2025)

### Device Filtering Enhancement
- **Removed**: `/api/v1/devices/project/{projectID}` route
- **Added**: Query parameter filtering for devices list endpoint:
  - `?project_id=123` - Filter by project ID (exact match)
  - `?city=NewYork` - Filter by city from `device_config.city` (exact match)
  - `?region=East` - Filter by region code (exact match against `region.code`)
  - `?device_type=LCD` - Filter by device type (partial text search in device_type JSONB)

### Device Counts by Region
- **Added**: `GET /api/v1/devices/counts/regions`
  - **Purpose**: Return total device counts grouped by `city` + `region` in a single response.
  - **Optional filter**: `?city=kcmo`
  - **Response shape**: `{"data": [{"city":"kcmo","region":"kc","count":123}, ...]}`

### Project Filtering Enhancement
- **Added**: Query parameter filtering for projects list endpoint:
  - `GET /api/v1/projects?city=kcmo` - Projects having at least one device in the given city
  - `GET /api/v1/projects?region=kc` - Projects having at least one device in the given region code
  - `GET /api/v1/projects?city=kcmo&region=kc` - Combined filter
- **Implementation**: Uses `EXISTS (SELECT 1 FROM devices d WHERE d.project = p.id AND ...)` since `projects` table does not store a dedicated `city` field.

### Authentication Fix
- **Fixed**: CityPost console authentication now uses proper auth scheme from config
- **Environment Variable**: `CITYPOST_CONSOLE_AUTH_SCHEME` (defaults to "Token")
- **Impact**: Device sync now works correctly with proper token authentication

### Device Response Fix
- **Fixed**: `GET /api/v1/devices/{hostName}` now returns actual device data instead of success message
- **Implementation**: Proper JSON encoding of device object with all fields including JSONB data

### New Repository Methods
- `ListWithFilters(ctx, filters, limit, offset)` - Filtered device listing
- `CountWithFilters(ctx, filters)` - Filtered device counting
- **DeviceFilters struct**: Supports ProjectID, City, Region, DeviceType filters

### Current API Endpoints
- `POST /api/v1/sync/console` - Sync projects and devices from CityPost console
- `GET /api/v1/projects` - List all projects (with pagination)
- `GET /api/v1/projects/{name}` - Get specific project by name
- `GET /api/v1/devices` - List devices with optional filtering (project_id, city, region, device_type)
- `GET /api/v1/devices/counts/regions` - Region-wise device counts (grouped by city + region)
- `GET /api/v1/devices/{hostName}` - Get specific device by hostName

## 2026-01-05
- Automated campaign lifecycle notifications: `cmd/api/main.go` now instantiates a shared SMTP sender (`services.NewSMTPSender`) and `CampaignNotifier`, wiring it into the existing scheduler jobs. `startScheduledCampaignCompleter` triggers completion emails immediately after marking campaigns completed, and a new `startCampaignNotificationDispatcher` sends activation/reminder emails daily based on `CAMPAIGN_NOTIFICATION_TIME`.
- `internal/services/campaign_notifier.go` gained logic to send activation, reminder (next-day), and completion emails by querying `ListByStartDate`/`ListByEndDate`. `CampaignRepository` implements `ListByEndDate` to support completion checks.
- Removed the manual `/api/v1/campaigns/notifications/send` route; campaign notifications are now fully automated.
- POP impressions integration: `internal/services/pop_client.go` consumes the richer `/pop/impressions` response (campaign total plus poster-level breakdown with `impressions` + `play_time`). `CampaignHandler` exposes this data in `GET /api/v1/campaigns/{id}/impressions`, and list endpoints populate `lifetime_impressions` when requested. Tests updated accordingly.

### Next Steps
- Test device filtering with various query parameters
- Validate `/api/v1/devices/counts/regions` output for both filtered and unfiltered requests
- Validate `/api/v1/projects` filtering semantics match expected behavior
- Verify sync functionality with proper authentication
- Consider adding more specific JSONB field queries if needed

## 2026-01-22

### Creatives: Venue Suggestions (OCR + venue taxonomy)
- Added `POST /api/v1/creatives/suggestions` which accepts multipart image uploads, extracts text via AWS Rekognition (`DetectText`), and returns per-file venue suggestions.
- Scoring uses venue `name` + `sub_category[]` with token normalization, synonym mapping, phrase matching, and intent boosts (e.g. retail intent phrases, family keywords).
- Added tests covering scoring behavior and handler validation.
- Documented endpoint in Swagger.

### Rekognition production hardening
- Added better runtime diagnostics for OCR failures:
  - Server logs underlying Rekognition error.
  - Response distinguishes likely configuration/credential errors (`ocr_not_configured`) vs runtime failures (`ocr_failed`).
- Production requires `AWS_REGION` to be set (otherwise defaults to `us-east-1`). CloudTrail can be used to validate `rekognition:DetectText` calls.

### Venue taxonomy tuning (data)
- Recommended enriching `venues.sub_category` with alias tags/synonyms to improve match coverage without changing core venue categories.

### GeoPath (future work)
- GeoPath (geopath.org) explored as a future data source for audience-based kiosk recommendations.
- Current device payload includes lat/lng under `device_config.position` and `device_config.LongLat`.
- Planned path: map device lat/lng to GeoPath markets (polygons) and score devices using GeoPath audience indices + impressions; fallback uses existing geo filters + transit (GTFS) signals + POP/impressions proxies.
