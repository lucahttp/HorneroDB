# ==========================================
# STAGE 1: Build the UI (React / Vite)
# ==========================================
FROM node:22-alpine AS ui-builder

WORKDIR /app
# We only need the web/ui folder for this stage
COPY web/ui/package.json web/ui/package-lock.json* ./
# `npm ci` installs exactly what the lockfile pins (deterministic builds)
RUN npm ci

# Copy the rest of the UI source code
COPY web/ui/ ./
# Build the UI
RUN npm run build


# ==========================================
# STAGE 2: Build the Go Backend
# ==========================================
FROM golang:1.25-alpine AS go-builder

WORKDIR /usr/src/app

# Copy go mod files and download deps
COPY go.mod go.sum ./
RUN go mod download

# Copy backend source code
COPY . .

# Copy the built UI from the previous stage into the Go embed folder
# hornerodb uses `go:embed ui/dist/*` in web/embed.go
COPY --from=ui-builder /app/dist /usr/src/app/web/ui/dist

# Build the server binary (static, so it runs on a bare Alpine image)
RUN CGO_ENABLED=0 GOOS=linux go build -v -ldflags="-s -w" -o /hornerodb ./cmd/server


# ==========================================
# STAGE 3: Final lightweight image
# ==========================================
FROM alpine:3.23

WORKDIR /app

# Install CA certificates (TLS), timezone data and wget (healthcheck).
# No `apk upgrade` here: it is non-deterministic and breaks reproducible builds.
RUN apk --no-cache add ca-certificates tzdata wget

# Run as a dedicated non-root user
RUN addgroup -S hornero && adduser -S hornero -G hornero

# Copy the compiled binary from the go builder
COPY --from=go-builder --chown=hornero:hornero /hornerodb /app/hornerodb

# Set the environment variable to trigger the embedded UI serving
ENV HORNERO_ENV=production
# Gin release mode (no debug output, no template reloading)
ENV GIN_MODE=release
# Default HTTP port (override with SERVER_PORT and adjust the healthcheck too)
ENV SERVER_PORT=8080

EXPOSE 8080

USER hornero

# Liveness probe used by Docker/Compose and orchestrators.
# The port is fixed to the image default so the probe never depends on
# build-time variable substitution; docker-compose overrides this probe with
# one that follows SERVER_PORT.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

CMD ["/app/hornerodb"]
