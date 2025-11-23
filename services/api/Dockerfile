# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd/ cmd/
COPY internal/ internal/

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /tee-api ./cmd/api

# Runtime stage
FROM alpine:latest

# Install Docker CLI to spawn sibling containers
RUN apk add --no-cache docker-cli

# Copy the binary from builder
COPY --from=builder /tee-api /usr/local/bin/tee-api

# Expose port
EXPOSE 8080

# Run the application
CMD ["tee-api"]
