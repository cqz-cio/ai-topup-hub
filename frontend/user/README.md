# Dujiao-Next User Web

The customer-facing storefront for browsing products, placing orders, and completing payments.

> This directory is part of the [dujiao-next](https://github.com/dujiao-next/dujiao-next)
> single repository and is no longer released on its own. Production assets are embedded
> into the server binary via `go:embed` and served by the same process, on the same port,
> as `frontend/admin`.

## Tech Stack

Vue 3 · TypeScript · Vite · Tailwind CSS v4 · Pinia · vue-i18n

## Local Development

```bash
pnpm install
pnpm run dev          # http://localhost:5173
```

Requires the backend to be running: `go run ./cmd/server` from the repository root.

The dev server proxies `/api`, `/uploads`, `/sitemap.xml` and `/robots.txt` to
`localhost:8080` with `changeOrigin: false`, preserving the original Host so the backend
can resolve reseller tenant domains.

## Build

```bash
pnpm run build
```

Assets are served from the site root `/` through the backend's `NoRoute` fallback.
See `internal/web/handler.go`.

You normally don't run these by hand — `make build-fullstack`, the Docker build, and the
GitHub Actions release workflow all build and embed the frontends for you.

## Documentation

https://dujiao-next.com
