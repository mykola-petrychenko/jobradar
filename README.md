# jobradar

A Go service that collects IT job postings from public APIs,
stores them in PostgreSQL, and analyzes the German job market.

## Status

🚧 In active development.

## Stack

- Go
- PostgreSQL (JSONB)
- Docker
- GitHub Actions (planned)

## Architecture

Fetchers (one per API source) → PostgreSQL (raw JSONB) → classifier → analytics.