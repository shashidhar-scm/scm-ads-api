FROM golang:tip AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

# Install swag for generating swagger docs
RUN go install github.com/swaggo/swag/cmd/swag@latest

COPY . .

# Generate swagger documentation
RUN swag init -g cmd/api/main.go -o docs/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/scm-ads-api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /out/scm-ads-api /app/scm-ads-api
COPY --from=builder /src/migrations /app/migrations
COPY --from=builder /src/docs /app/docs

ENV PORT=8080

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/app/scm-ads-api"]
