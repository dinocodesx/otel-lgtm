# Multi-stage build for minimal image size
FROM golang:1.23.6-alpine AS builder

# Install necessary packages for building
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY app/go.mod app/go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY app/main.go ./

# Build the binary with optimizations for size
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -a -installsuffix cgo \
    -ldflags='-w -s -extldflags "-static"' \
    -o server main.go

# Final stage - minimal runtime image
FROM scratch

# Copy ca-certificates for HTTPS requests
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy the binary
COPY --from=builder /app/server /server

# Expose port
EXPOSE 8080

# Set environment variables
ENV PORT=8080
ENV TZ=UTC

# Run the binary
ENTRYPOINT ["/server"]