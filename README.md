# jobradar

Job-market data collection tool. Fetches job postings from public job board
APIs and stores them in PostgreSQL for later analysis.

The goal: build a personal dataset of the German (and wider DACH) IT job
market — which technologies, tools and skills employers actually ask for,
so that learning and practice can be aimed at real demand.

## Planned

- More sources (Bundesagentur für Arbeit, ...)
- Static analysis first: keyword and pattern matching to drop non-IT postings
  and extract what can be read directly from the text
- Claude integration as a core stage: filters are defined in plain language,
  and the model classifies postings against them

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