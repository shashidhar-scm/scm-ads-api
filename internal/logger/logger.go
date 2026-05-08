package logger

import (
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"scm/internal/config"
)

// New constructs a zerolog logger configured from application config.
func New(cfg *config.Config) *zerolog.Logger {
	zerolog.TimeFieldFormat = time.RFC3339Nano

	level := zerolog.InfoLevel
	if cfg != nil {
		if parsedLevel, err := zerolog.ParseLevel(strings.ToLower(strings.TrimSpace(cfg.LogLevel))); err == nil {
			level = parsedLevel
		}
	}

	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	}

	base := zerolog.New(consoleWriter).
		Level(level).
		With().
		Timestamp().
		Logger()

	log.Logger = base
	return &base
}
