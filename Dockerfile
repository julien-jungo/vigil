# syntax=docker/dockerfile:1

# ── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
      -ldflags="-s -w -X main.version=${VERSION}" \
      -o vigil ./cmd/vigil

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM mcr.microsoft.com/playwright:v1.51.0-noble

RUN npm install -g @playwright/mcp@0.0.71

COPY --from=builder /app/vigil /usr/local/bin/vigil

ENTRYPOINT ["vigil"]
