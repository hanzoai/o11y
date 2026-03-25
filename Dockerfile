# Stage 1: Build frontend
FROM node:22-alpine AS frontend
RUN corepack enable && corepack prepare pnpm@latest --activate
WORKDIR /app/frontend
COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY frontend/ .
RUN pnpm build

# Stage 2: Build Go binary
FROM golang:1.26-alpine AS backend
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ARG VERSION=""
ARG COMMIT_HASH=""
ARG BUILD_TIME=""
ARG BRANCH=""

RUN VERSION=${VERSION:-$(git describe --tags --always 2>/dev/null || echo "dev")} && \
    COMMIT_HASH=${COMMIT_HASH:-$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")} && \
    BUILD_TIME=${BUILD_TIME:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")} && \
    BRANCH=${BRANCH:-$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")} && \
    LDFLAGS="-X github.com/hanzoai/o11y/pkg/version.version=$VERSION" && \
    LDFLAGS="$LDFLAGS -X github.com/hanzoai/o11y/pkg/version.hash=$COMMIT_HASH" && \
    LDFLAGS="$LDFLAGS -X github.com/hanzoai/o11y/pkg/version.time=$BUILD_TIME" && \
    LDFLAGS="$LDFLAGS -X github.com/hanzoai/o11y/pkg/version.branch=$BRANCH" && \
    CGO_ENABLED=0 go build -ldflags "$LDFLAGS" -o /o11y ./cmd/server/

# Stage 3: Final image
FROM alpine:3.20
LABEL maintainer="hanzoai"
WORKDIR /root

RUN apk add --no-cache ca-certificates

COPY --from=backend /o11y /root/o11y
COPY templates/email /root/templates
COPY --from=frontend /app/frontend/build/ /etc/o11y/web/

RUN chmod 755 /root /root/o11y

ENTRYPOINT ["./o11y", "server"]
