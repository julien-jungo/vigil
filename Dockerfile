# syntax=docker/dockerfile:1

# ── Build stage ───────────────────────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

WORKDIR /app

RUN apk add --no-cache make

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN make build VERSION=${VERSION}

# ── MCP install stage ─────────────────────────────────────────────────────────
FROM mcr.microsoft.com/playwright:v1.51.0-noble AS mcp-installer
# renovate: datasource=npm depName=@playwright/mcp
RUN npm install -g @playwright/mcp@0.0.71 --prefix /opt/mcp

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM mcr.microsoft.com/playwright:v1.51.0-noble

RUN apt-get update \
    && apt-get upgrade -y --no-install-recommends \
    && rm -rf /var/lib/apt/lists/* \
    && rm -rf /usr/lib/node_modules/npm /usr/lib/node_modules/corepack /usr/lib/node_modules/yarn \
    && rm -f /usr/bin/npm /usr/bin/npx /usr/bin/corepack /usr/bin/yarn /usr/bin/yarnpkg

COPY --from=mcp-installer /opt/mcp /opt/mcp
RUN ln -sf /opt/mcp/bin/playwright-mcp /usr/bin/playwright-mcp \
    && playwright-mcp --help > /dev/null

COPY --from=builder /app/vigil /usr/local/bin/vigil

ENTRYPOINT ["vigil"]
