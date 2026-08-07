# jobradar

Fetches job postings from the Arbeitnow public API and stores them
in PostgreSQL.

## Requirements

- Go 1.26+
- PostgreSQL 18+

## Setup

```bash
cp .env.example .env
# fill in DATABASE_URL in .env
docker exec -i <postgres-container> psql -U postgres -d jobradar < migrations/0001_postings.sql
```

## Run

```bash
go run ./cmd/fetcher
```

## Development

```bash
gofmt -l .
go vet ./...
golangci-lint run
go build ./...
```