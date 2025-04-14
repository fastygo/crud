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

# Stage 2: Create the final lightweight runtime image
FROM alpine:latest

# Set the working directory
WORKDIR /app

# Copy only the compiled binary from the builder stage
COPY --from=builder /app/cms /app/cms

# Assets are embedded in the binary, no need to copy them.

# Expose the default port 8080. Coolify might override or map this automatically.
EXPOSE 8080

# Set the entrypoint to run the compiled application
ENTRYPOINT ["/app/cms"] 