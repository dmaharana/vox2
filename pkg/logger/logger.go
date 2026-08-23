package logger

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Init initializes the global zerolog logger based on level and format.
func Init(levelStr, format string) zerolog.Logger {
	level := parseLevel(levelStr)
	zerolog.SetGlobalLevel(level)

	var output io.Writer = os.Stdout
	if strings.ToLower(format) == "console" || format == "" {
		output = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}
	}

	l := zerolog.New(output).With().Timestamp().Caller().Logger()
	log.Logger = l
	return l
}

func parseLevel(lvl string) zerolog.Level {
	switch strings.ToLower(lvl) {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}

// Get returns the global logger.
func Get() zerolog.Logger {
	return log.Logger
}
