# Dujiao-Next API

Dujiao-Next API is the backend service for the Dujiao-Next ecosystem. It provides public APIs, user/auth APIs, order and payment workflows, and admin APIs.

## Tech Stack

- Go
- Gin
- GORM
- SQLite / MySQL / PostgreSQL

## What This Service Does

- Serves REST APIs for user, order, and payment flows
- Handles payment callbacks/webhooks
- Supports product, fulfillment, and configuration management

## Quick Start

```bash
go mod tidy
go run cmd/server/main.go
```

The default health check endpoint is:

- `GET /health`

## Online Documentation

- https://dujiaoka.com
