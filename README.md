# TeleRealm CDN

TeleRealm is a lightweight CDN-style file service built on top of Telegram Bot API storage. You can upload files, generate secure shareable links, manage upload records, and use either protected APIs or path-based public integration.

## Overview

This version includes:

- Dedicated upload workspace page
- Public API integration pages and docs
- CRUD record APIs for uploaded files
- Secure download links via encrypted drive key
- Docker-ready deployment setup

## Attribution And Rights

Original project foundation and core idea:

- Lai Chi Thinh (ThinhPhoenix) - FPT University

Current version note:

- Additional features, API flows, and UI pages were extended and rewired in this iteration.

## Main Features

- Telegram-backed file upload and retrieval
- Secure download route via /drive/:key
- Protected API with bearer bot token
- Public CRUD API by URL scope: /link/:botToken/:chatID
- Browser upload workspace with local temporary config/history
- Public pages:
  - /upload
  - /public-api
  - /demo
  - /docs

## Route Map

### Public Routes

- GET /
- GET /upload
- GET /public-api
- GET /demo
- GET /docs
- GET /ping
- GET /drive/:key

### Public Path-Based API

- POST /link/:botToken/:chatID
- GET /link/:botToken/:chatID
- GET /link/:botToken/:chatID/:id
- PATCH /link/:botToken/:chatID/:id
- DELETE /link/:botToken/:chatID/:id

### Protected API (Authorization: Bearer <bot_token>)

- POST /send
- POST /files
- GET /files
- GET /files/:id
- PATCH /files/:id
- DELETE /files/:id
- GET /url?file_id=...
- GET /info?file_id=...
- GET /verify?chat_id=...

## Quick Start (Local)

### Prerequisites

- Go 1.23+
- Telegram bot token from BotFather

### Run

```bash
go mod download
go run main.go
```

Server default: http://localhost:7777

## Quick Start (Docker)

### Build and run with compose

```bash
docker compose build
docker compose up -d
```

### Stop

```bash
docker compose down
```

## Example API Calls

### Protected upload

```bash
curl -X POST \
  -H "Authorization: Bearer <bot_token>" \
  -F "chat_id=<chat_id>" \
  -F "document=@/path/to/file.png" \
  http://localhost:7777/send
```

### Public scoped upload

```bash
curl -X POST \
  -F "document=@/path/to/file.png" \
  http://localhost:7777/link/<bot_token>/<chat_id>
```

### Public scoped list

```bash
curl http://localhost:7777/link/<bot_token>/<chat_id>
```

## Security Notes

- Do not expose bot tokens in untrusted frontend contexts.
- Path-based public API includes bot token in URL, so prefer protected API in production.
- Local browser storage on upload page is for convenience only.
- Rotate bot token immediately if leaked.

## Contributing

Contributions are welcome through pull requests.
