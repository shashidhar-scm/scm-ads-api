// internal/handlers/base.go
package handlers

import (
	"database/sql"
	"scm/internal/config"
)

type BaseHandler struct {
	DB   *sql.DB
	Cfg  *config.Config
}

func NewBaseHandler(db *sql.DB, cfg *config.Config) *BaseHandler {
	return &BaseHandler{
		DB:   db,
		Cfg:  cfg,
	}
}