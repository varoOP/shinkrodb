package logger

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// NewLogger creates a new zerolog logger with console output
func NewLogger() zerolog.Logger {
	output := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05"}
	return log.Output(output).With().Timestamp().Logger()
}

// NewLoggerWithLevel creates a new logger with a specific log level
func NewLoggerWithLevel(level zerolog.Level) zerolog.Logger {
	logger := NewLogger()
	return logger.Level(level)
}

