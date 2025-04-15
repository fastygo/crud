# Stage 1: Build the application
# Using a specific Go version on Alpine for consistency and smaller size
FROM golang:1.23-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy go module files first to leverage Docker cache
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the entire source code from the build context provided by Coolify (cloned repo)
# Note: .dockerignore prevents unnecessary files from being copied
COPY . .

# Generate Go code from QuickTemplate templates
# Install qtc and run it to ensure templates are compiled
RUN go install github.com/valyala/quicktemplate/qtc@latest && qtc -dir=./internal/templates

# Build the Go application as a static binary for Alpine compatibility
# -ldflags="-w -s" reduces binary size
# CGO_ENABLED=0 is crucial for building a static binary for Alpine
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o /app/cms ./cmd/cms/main.go

# Stage 2: Create the final runtime image
FROM alpine:latest

# Create a non-root user and group
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

# Copy binary and ensure correct ownership
COPY --from=builder --chown=appuser:appgroup /app/cms /app/cms

# Switch to non-root user
USER appuser

# Set default environment variables for authentication and login limits
# These can be overridden at runtime (e.g., docker run -e AUTH_USER=new_user ...)
ENV AUTH_USER="admin"
ENV AUTH_PASS="qwerty123"
ENV LOGIN_LIMIT_ATTEMPT="3"
ENV LOGIN_LOCK_DURATION="1m"

# Assets are embedded in the binary, no need to copy them.

# Expose the default port 8080. Coolify might override or map this automatically.
EXPOSE 8080

# Set the entrypoint to run the compiled application
ENTRYPOINT ["/app/cms"] 