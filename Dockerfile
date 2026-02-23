# ==========================================
# STAGE 1: Build the UI (React / Vite)
# ==========================================
FROM node:22-alpine AS ui-builder

WORKDIR /app
# We only need the web/ui folder for this stage
COPY web/ui/package.json web/ui/package-lock.json* ./
RUN npm install

# Copy the rest of the UI source code
COPY web/ui/ .
# Build the UI
RUN npm run build


# ==========================================
# STAGE 2: Build the Go Backend
# ==========================================
FROM golang:1.24-alpine AS go-builder

WORKDIR /usr/src/app

# Copy go mod files and download deps
COPY go.mod go.sum ./
RUN go mod download

# Copy backend source code
COPY . .

# Copy the built UI from the previous stage into the Go embed folder
# hornerodb uses `go:embed ui/dist/*` in web/embed.go
COPY --from=ui-builder /app/dist /usr/src/app/web/ui/dist

# Build the server binary
RUN CGO_ENABLED=0 GOOS=linux go build -v -o /hornerodb ./cmd/server


# ==========================================
# STAGE 3: Final lightweight image
# ==========================================
FROM alpine:latest

WORKDIR /app

# Install CA certificates for external HTTPS requests if needed
RUN apk --no-cache add ca-certificates tzdata

# Copy the compiled binary from the go builder
COPY --from=go-builder /hornerodb /app/hornerodb

# Set the environment variable to trigger the embedded UI serving
ENV HORNERO_ENV=production

EXPOSE 8080

CMD ["/app/hornerodb"]