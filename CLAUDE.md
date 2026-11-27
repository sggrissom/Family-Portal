# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Development Commands

```bash
# Local development server (runs on http://family.localhost:8666)
make local

# Build everything (frontend + Go binary)
make build

# Run all Go tests
make test

# TypeScript type checking
make typecheck

# Run all quality checks (test + typecheck + lint)
make check

# Format all code (Go + Prettier)
make format

# Deploy to production
make deploy
```

## Architecture Overview

**Family Portal** is a full-stack family growth/milestone tracking application:
- **Backend**: Go with vbeam framework, BoltDB database
- **Frontend**: TypeScript/Preact with vlens reactive framework
- **Build**: Makefile orchestrates Go builds and esbuild for frontend

### Backend (`/backend/`)

Uses **vbeam** RPC pattern - backend defines handlers with `RegisterProc()` that auto-generate TypeScript client bindings in `frontend/server.ts`.

Key modules:
- `auth.go`, `refresh_tokens.go` - JWT auth with Google OAuth support
- `person.go`, `growth.go`, `milestone.go` - Core domain entities
- `photos.go`, `photo_worker.go` - Photo management with background processing queue
- `websocket_chat.go` - Real-time family chat via WebSocket

Database: BoltDB with vbolt abstraction layer (buckets, indexes, vpack serialization).

### Frontend (`/frontend/`)

Entry point: `main.tsx` - defines all routes using vlens lazy-loading.

Structure:
- `server.ts` - Auto-generated API bindings (DO NOT edit manually)
- `pages/` - Page components organized by feature
- `components/` - Reusable UI components
- `hooks/` - Custom Preact hooks
- `lib/` - Utilities (auth, dates, helpers)

Styling: CSS-in-JS using vlens `block()` function with CSS variables for theming.

### Configuration (`/cfg/`)

Build tag-based config:
- `local.go` (default) - Development settings
- `release.go` (`//go:build release`) - Production settings

### Key Patterns

- **Type safety**: Go structs → auto-generated TypeScript types via vbeam
- **Auth flow**: JWT tokens with refresh token rotation, LocalStorage caching
- **Theme switching**: `data-theme` attribute on `<html>`, CSS variables in `global.ts`
- **Background jobs**: Photo processing uses worker queue pattern

## Path Aliases

TypeScript uses `@app/*` → `frontend/*` (configured in tsconfig.json)

## Global Constants

Defined at build time (see `global.d.ts`):
- `DEBUG` - true in local, false in release
- `BROWSER` - true when running in browser
- `VERBOSE` - logging verbosity

## Agent skills

### Issue tracker

Issues are tracked in GitHub Issues on `sggrissom/Family-Portal` via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Domain docs

Single-context: one `GLOSSARY.md` and `docs/adr/` at the repo root. See `docs/agents/domain.md`.
