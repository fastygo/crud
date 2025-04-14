# Stage 1: Build the application
FROM golang:1.23-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy go module files
COPY go.mod go.sum ./

# Download dependencies. Dependencies will be cached if the go.mod/sum files don't change.
RUN go mod download

# Copy the entire source code
# Note: .dockerignore prevents unnecessary files from being copied
COPY . .

# Generate Go code from QuickTemplate templates
# Ensure qtc is available or install it if needed in the build image,
# but typically it's expected to be run before building the container.
# For robustness, we can install and run it here.
RUN go install github.com/valyala/quicktemplate/qtc@latest && qtc -dir=./internal/templates

# Build the Go application
# -ldflags="-w -s" reduces binary size by removing debug information
# CGO_ENABLED=0 attempts to build a static binary, which is ideal for Alpine.
# This might fail if dependencies require CGO (fasthttp sometimes does). If it fails, remove CGO_ENABLED=0.
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o /app/cms ./cmd/cms/main.go

# Stage 2: Create the final runtime image
FROM alpine:latest

# Set the working directory
WORKDIR /app

# Copy the built binary from the builder stage
COPY --from=builder /app/cms /app/cms

# The assets (including initial.db) are embedded in the binary via //go:embed,
# so we don't need to copy the 'assets' directory separately.

# Expose the port the application listens on (default is 8080)
EXPOSE 8080

# Command to run the application
ENTRYPOINT ["/app/cms"] 