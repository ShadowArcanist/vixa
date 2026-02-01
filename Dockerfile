# Build Stage
FROM golang:1.23-alpine AS builder

# Set working directory
WORKDIR /app

# Install necessary packages
RUN apk add --no-cache git ca-certificates

# Copy Go modules and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the Go binary named "vixa"
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o vixa ./cmd/main.go


# Runtime Stage
FROM alpine:latest

# Install certificates and timezone data
RUN apk --no-cache add ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/vixa .

# Expose the application port
EXPOSE 8080

# Healthcheck to ensure the vixa process is running
HEALTHCHECK --interval=30s --timeout=5s --retries=3 --start-period=10s \
  CMD pgrep vixa || exit 1

# Default command to run the binary
CMD ["./vixa"]
