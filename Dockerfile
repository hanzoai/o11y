# syntax=docker/dockerfile:1
#
# Hanzo O11y — standalone community server image.
#
# Builds the real server binary `./cmd/community` (NOT `./cmd/server`, which does
# not exist). `cmd/community` does not import github.com/hanzoai/cloud, so the
# go.mod `replace github.com/hanzoai/cloud => ../cloud` is inert for this build
# and no sibling checkout is required — cloud lives only in the root `mount.go`
# cloud-embed adapter, which `cmd/community` never pulls in.
#
# cmd/community's graph pulls PRIVATE hanzoai/* forks (hanzoai/sqlite +
# hanzoai/datastore-go — the sqlite + datastore drivers added by the driver
# swap), so the module fetch DOES need git auth. A `gh_token` build secret
# (docker.yaml passes secrets.GH_PAT) is mounted and wired via git
# url.insteadOf before the build; GOPRIVATE + GOSUMDB=off route hanzoai/*
# direct and skip the sumdb.
#
# The browser SPA is served at the edge by hanzoai/static (house-native static
# plugin), not bundled here, so the server runs headless (O11Y_WEB_ENABLED=false).
# The frontend/ tree is a separate concern — its pnpm-lock.yaml is mid-migration
# and not build-ready; bundle it back once that is resolved.

########################################
# Stage 1 — Go build (./cmd/community)
########################################
FROM golang:1.26.4-alpine AS backend
RUN apk add --no-cache git ca-certificates
WORKDIR /src

# hanzoai/* modules fetched via direct git (some private — auth'd by the
# gh_token secret mounted on the build RUN below); trust go.sum.
ENV GOPRIVATE=github.com/hanzoai/* \
    GOSUMDB=off \
    CGO_ENABLED=0 \
    GOOS=linux

COPY . .

# Version metadata is injected via build-args (the build context excludes .git,
# so we must not shell out to `git` here).
ARG VERSION=dev
ARG COMMIT_HASH=unknown
ARG BUILD_TIME=unknown
ARG BRANCH=unknown
ARG VARIANT=community

# Build ONLY ./cmd/community. A bare `go mod download` would fail because
# github.com/hanzoai/cloud is a direct require replaced to ../cloud (absent
# here); building the single package resolves only its pruned graph, which does
# not include cloud.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=secret,id=gh_token \
    if [ -s /run/secrets/gh_token ]; then \
      git config --global url."https://x-access-token:$(cat /run/secrets/gh_token)@github.com/".insteadOf "https://github.com/"; \
    fi && \
    VERPKG=github.com/hanzoai/o11y/pkg/version && \
    go build -trimpath -tags timetzdata \
      -ldflags "-s -w \
        -X ${VERPKG}.version=${VERSION} \
        -X ${VERPKG}.variant=${VARIANT} \
        -X ${VERPKG}.hash=${COMMIT_HASH} \
        -X ${VERPKG}.time=${BUILD_TIME} \
        -X ${VERPKG}.branch=${BRANCH}" \
      -o /out/o11y ./cmd/community

########################################
# Stage 2 — Runtime (minimal)
########################################
FROM alpine:3.20
LABEL org.opencontainers.image.source="https://github.com/hanzoai/o11y" \
      org.opencontainers.image.title="hanzo-o11y" \
      org.opencontainers.image.description="Hanzo O11y — OTLP-native observability (O11y fork), standalone community server" \
      maintainer="hanzoai"

RUN apk add --no-cache ca-certificates && \
    mkdir -p /var/lib/o11y

WORKDIR /root

# Server binary.
COPY --from=backend /out/o11y /usr/local/bin/o11y
# Alert/email templates (emailing default dir = /root/templates/email).
COPY templates/ /root/templates/

# The browser SPA is served by hanzoai/static at the edge; run headless. When a
# deployment bundles/mounts web assets, set O11Y_WEB_ENABLED=true and
# O11Y_WEB_DIRECTORY to their path.
ENV O11Y_WEB_ENABLED=false

# Public query-service HTTP + query API (constants.HTTPHostPort = 0.0.0.0:8080).
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/o11y", "server"]
