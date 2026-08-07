package main

import (
	"log/slog"
	"os"
	"strings"
)

func newLogger() *slog.Logger {
	opts := &slog.HandlerOptions{Level: logLevel()}

	if strings.EqualFold(os.Getenv("JOBRADAR_LOG_FORMAT"), "json") {
		return slog.New(slog.NewJSONHandler(os.Stderr, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stderr, opts))
}

func logLevel() slog.Level {
	var lvl slog.Level
	// "debug", "info", "warn", "error"
	if err := lvl.UnmarshalText([]byte(os.Getenv("JOBRADAR_LOG_LEVEL"))); err != nil {
		return slog.LevelInfo
	}
	return lvl
}
