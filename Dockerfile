# ==============================================================================
# Stage 1: Frontend Build (Vue 3 + Vite)
# ==============================================================================
FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend-builder
ARG APP_VERSION=dev
ENV VITE_APP_VERSION=$APP_VERSION
WORKDIR /app/web

# Copy package descriptors first to leverage caching
COPY web/package*.json ./
RUN npm ci --ignore-scripts

# Copy source code and build
COPY web/ ./
RUN npm run build

# ==============================================================================
# Stage 2: Development environment for Go
# ==============================================================================
FROM golang:alpine AS dev
WORKDIR /app
ENV GOTOOLCHAIN=auto
RUN apk add --no-cache git curl build-base
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
CMD ["go", "run", "./cmd/server/main.go"]

# ==============================================================================
# Stage 3: Backend Build (Go binary)
# ==============================================================================
FROM --platform=$BUILDPLATFORM golang:alpine AS backend-builder
ARG APP_VERSION=dev
WORKDIR /app
ENV GOTOOLCHAIN=auto
RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
# Copy compiled frontend assets to web/dist
COPY --from=frontend-builder /app/web/dist ./web/dist

# Target architecture parameters provided by Docker Buildx
ARG TARGETOS
ARG TARGETARCH

# Build statically linked Go binary with injected AppVersion
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -ldflags="-s -w -X main.AppVersion=${APP_VERSION}" -o /app/teslacost ./cmd/server

# ==============================================================================
# Stage 4: Production Runner (Scratch or Minimal Alpine)
# ==============================================================================
FROM alpine:3.24 AS prod
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S teslacost && adduser -S -G teslacost -H teslacost \
    && mkdir -p /data/documents && chown teslacost:teslacost /data/documents

COPY --from=backend-builder /app/teslacost /app/teslacost
COPY --from=backend-builder /app/migrations /app/migrations

EXPOSE 8080
ENV PORT=8080
ENV STORAGE_DIR=/data/documents
USER teslacost

# Declare the document volume so Docker knows to persist it.
VOLUME ["/data/documents"]

# Uses busybox wget (bundled with alpine) so no extra package is needed.
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD wget -q --spider http://127.0.0.1:8080/api/health || exit 1

CMD ["/app/teslacost"]
