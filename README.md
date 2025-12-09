# SCM Ads API

A high-performance advertising API service built with Go.

## Getting Started

### Prerequisites

- Go 1.16 or higher

### Installation

1. Clone the repository
2. Build the application:
   ```bash
   go build -o bin/scm-ads-api ./cmd/api
   ```
3. Run the application:
   ```bash
   ./bin/scm-ads-api
   ```

The server will start on `http://localhost:8080`

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

- `PORT`: Port to run the server on (default: 8080)

## Development

### Running Tests

```bash
go test ./...
```

### Code Formatting

```bash
gofmt -w .
```
