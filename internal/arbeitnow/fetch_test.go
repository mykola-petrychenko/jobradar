package arbeitnow

import (
	"context"
	"log/slog"
	"os"
	"testing"
)

func TestFetchOnePage(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	c := New(logger)

	_, err := c.fetchPage(context.Background(), endpoint, 3)
	if err != nil {
		t.Fatalf("fetchPage failed: %v", err)
	}
}

func TestFetchTwoPage(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	c := New(logger)

	p1, err := c.fetchPage(context.Background(), endpoint, 1)
	if err != nil {
		t.Fatalf("page 1 failed: %v", err)
	}

	p2, err := c.fetchPage(context.Background(), p1.Links.Next, 2)
	if err != nil {
		t.Fatalf("page 2 failed: %v", err)
	}

	_, err = c.fetchPage(context.Background(), p2.Links.Next, 3)
	if err != nil {
		t.Fatalf("page 3 failed: %v", err)
	}
}
