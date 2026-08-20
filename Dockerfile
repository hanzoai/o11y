# syntax=docker/dockerfile:1
#
# Hanzo O11y — standalone community server image.
#
# Builds the real server binary `./cmd/community` (NOT `./cmd/server`, which does
# not exist).
#
# THIS MODULE NO LONGER DEPENDS ON github.com/hanzoai/cloud AT ALL. It used to,
# for one field of one struct on one line of mount.go — and that made the module
# graph a CYCLE (o11y → cloud → o11y), which is why this file used to explain
# that a whole-module `go mod download` was unresolvable here and that the build
# had to be scoped to a single package to route around it. It does not any more:
# the route table takes the router it registers into and nothing else, cloud is
# gone from go.mod with 20-odd transitive requirements behind it, and this build
# is scoped to ./cmd/community only because that is the binary the image runs.
#
# cmd/community's graph pulls PRIVATE hanzoai/* forks (hanzoai/sqlite +
# hanzoai/datastore-go — the sqlite + datastore drivers added by the driver
# swap), so the module fetch DOES need git auth. A `gh_token` build secret is
# mounted and wired via git url.insteadOf before the build; GOPRIVATE +
# GOSUMDB=off route hanzoai/* direct and skip the sumdb. The shared lane
# supplies that secret on every image build (KMS, falling back to the org
# GH_PAT) — see `images:` in the repo-root hanzo.yml, which is the whole of
# this repo's CI config.
#
# The browser SPA is served at the edge by hanzoai/static (house-native static
# plugin), not bundled here, so the server runs headless (O11Y_WEB_ENABLED=false).
# The frontend/ tree is a separate concern — its pnpm-lock.yaml is mid-migration
# and not build-ready; bundle it back once that is resolved.

########################################
# Stage 1 — Go build (./cmd/community)
########################################
FROM golang:1.26.5-alpine AS backend
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

# Build ./cmd/community — the binary this image runs. Its graph now reaches the
# module's published route table (github.com/hanzoai/o11y, mounted by
# pkg/query-service/app), which is the whole point: the OpenAPI document and the
# MCP tools ship in the process rather than in a package nothing links.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=secret,id=gh_token \
    if [ -s /run/secrets/gh_token ]; then \
      export GIT_CONFIG_COUNT=1 \
             GIT_CONFIG_KEY_0="url.https://x-access-token:$(cat /run/secrets/gh_token)@github.com/.insteadOf" \
             GIT_CONFIG_VALUE_0="https://github.com/"; \
    fi; \
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
