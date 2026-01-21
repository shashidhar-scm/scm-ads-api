SWAG_VERSION ?= v1.16.4

.PHONY: swagger
swagger:
	go run github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION) init -g cmd/api/main.go -o docs
