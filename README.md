# Dujiao-Next API

Dujiao-Next API is the backend service for the Dujiao-Next ecosystem. It provides public APIs, user/auth APIs, order and payment workflows, and admin APIs.

## Tech Stack

- Go
- Gin
- GORM
- SQLite / PostgreSQL

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

- https://dujiao-next.com

## Star History

<a href="https://www.star-history.com/?repos=dujiao-next%2Fdujiao-next&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=dujiao-next/dujiao-next&type=date&theme=dark&legend=top-left&sealed_token=pLO1UK6ooAVrG-Ax2T2YaXxp2jAmvLNEOCMtlLr3tVrDSS1GHTeQIEjhMpafFToiXGjdEOkjTK4QERxqQjl8-xjwmo4ngQqOwxBZpzcVfqpF6braIFEhJRM1iAVRA7wbrUAQltZSRwebK_w0CUDg-cChnGbROE1WTSted0VXWtKg28dhOY9-GCn7KXsH" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=dujiao-next/dujiao-next&type=date&legend=top-left&sealed_token=pLO1UK6ooAVrG-Ax2T2YaXxp2jAmvLNEOCMtlLr3tVrDSS1GHTeQIEjhMpafFToiXGjdEOkjTK4QERxqQjl8-xjwmo4ngQqOwxBZpzcVfqpF6braIFEhJRM1iAVRA7wbrUAQltZSRwebK_w0CUDg-cChnGbROE1WTSted0VXWtKg28dhOY9-GCn7KXsH" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=dujiao-next/dujiao-next&type=date&legend=top-left&sealed_token=pLO1UK6ooAVrG-Ax2T2YaXxp2jAmvLNEOCMtlLr3tVrDSS1GHTeQIEjhMpafFToiXGjdEOkjTK4QERxqQjl8-xjwmo4ngQqOwxBZpzcVfqpF6braIFEhJRM1iAVRA7wbrUAQltZSRwebK_w0CUDg-cChnGbROE1WTSted0VXWtKg28dhOY9-GCn7KXsH" />
 </picture>
</a>