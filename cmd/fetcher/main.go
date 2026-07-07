// Command fetcher collects job postings and stores them in PostgreSQL.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/mykola-petrychenko/jobradar/internal/postgres"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	godotenv.Load()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL is not set")
	}

	connectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	store, err := postgres.New(connectCtx, dsn)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer store.Close()

	fmt.Println("database connection OK")
	return nil
}
