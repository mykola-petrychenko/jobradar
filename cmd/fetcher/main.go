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
	"github.com/mykola-petrychenko/jobradar/internal/arbeitnow"
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

	since, err := store.LatestCreatedAt(ctx, "arbeitnow")
	if err != nil {
		return err
	}
	monthAgo := time.Now().AddDate(0, -1, 0).Unix()
	if since < monthAgo {
		since = monthAgo
	}

	client := arbeitnow.New()
	postings, err := client.Fetch(ctx, since)
	if err != nil {
		return fmt.Errorf("fetch arbeitnow: %w", err)
	}

	inserted := 0
	for _, p := range postings {
		ok, err := store.Insert(ctx, p)
		if err != nil {
			return fmt.Errorf("store posting: %w", err)
		}
		if ok {
			inserted++
		}
	}

	fmt.Printf("arbeitnow: fetched %d postings, %d new\n", len(postings), inserted)
	return nil
}
