package util

import (
	"log/slog"
	"os"
)

func SetupLogger(level slog.Level, enviroment string) *slog.Logger {
	loggerOpts := slog.HandlerOptions{
		Level:     level,
		AddSource: false,
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &loggerOpts))
	if enviroment == "PROD" {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &loggerOpts))
	}

	slog.SetDefault(logger)
	return logger
}
