package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/joho/godotenv"
)

// config holds all settings the fetcher needs to run.
type config struct {
	databaseURL      string
	debugDumpDir     string
	dbConnectTimeout time.Duration
}

const defaultDBConnectTimeout = 15 * time.Second

// loadEnvFile loads variables from a .env file into the process
// environment, if one exists. A missing file is not an error - the
// program is expected to run fine from real environment variables
// alone, e.g. under systemd.
func loadEnvFile() error {
	err := godotenv.Load()
	switch {
	case err == nil:
		return nil
	case errors.Is(err, fs.ErrNotExist):
		return nil
	default:
		return fmt.Errorf("reading .env file: %w", err)
	}
}

// loadConfig reads configuration from environment variables.
// Call loadEnvFile first if a .env file should be honored.
func loadConfig() (config, error) {
	cfg := config{
		databaseURL:      os.Getenv("DATABASE_URL"),
		debugDumpDir:     os.Getenv("JOBRADAR_DEBUG_DIR"),
		dbConnectTimeout: defaultDBConnectTimeout,
	}

	if cfg.databaseURL == "" {
		return config{}, errors.New("DATABASE_URL is required")
	}

	if raw := os.Getenv("DB_CONNECT_TIMEOUT"); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return config{}, fmt.Errorf("DB_CONNECT_TIMEOUT %q: %w", raw, err)
		}
		if d <= 0 {
			return config{}, fmt.Errorf("DB_CONNECT_TIMEOUT must be positive, got %s", d)
		}
		cfg.dbConnectTimeout = d
	}

	return cfg, nil
}
