# Dujiao-Next Admin

The management console for operating products, orders, users, payment channels, and system settings.

> This directory is part of the [dujiao-next](https://github.com/dujiao-next/dujiao-next)
> single repository and is no longer released on its own. Production assets are embedded
> into the server binary via `go:embed` and served by the same process, on the same port,
> as `frontend/user`.

## Tech Stack

Vue 3 · TypeScript · Vite · Tailwind CSS v4 · shadcn-vue / reka-ui

## Local Development

```bash
pnpm install
pnpm run dev          # http://localhost:5174 — proxies /api and /uploads to localhost:8080
```

Requires the backend to be running: `go run ./cmd/server` from the repository root.

## Build

```bash
pnpm run build            # standalone assets, base = /
pnpm run build:fullstack  # embeddable assets, base = ./ with a <base> placeholder injected
```

`build:fullstack` injects `<base href="__DJ_ADMIN_BASE__/">` into `index.html`. The backend
replaces the placeholder with the configured `web.admin_path` at startup, so one build
artifact can be mounted under any custom prefix. See `internal/web/handler.go`.

You normally don't run these by hand — `make build-fullstack`, the Docker build, and the
GitHub Actions release workflow all build and embed the frontends for you.

## Documentation

https://dujiao-next.com
