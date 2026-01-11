# Use the official Golang image as the base
FROM golang:1.24-alpine AS builder

# Set environment variables
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# Set working directory inside the container
WORKDIR /build

# Copy go.mod and go.sum files for dependency installation
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the entire application source
COPY . .

# Build the Go binary
RUN go build -o /app .

# Final lightweight stage
FROM alpine:3.21 AS final


RUN addgroup -S -g 1001 appgroup && \
    adduser -S -u 1001 -G appgroup appuser


RUN mkdir -p /app && chown -R appuser:appgroup /app

# Copy the compiled binary from the builder stage
COPY --from=builder --chown=appuser:appgroup /app /bin/app


RUN addgroup -g 999 docker && addgroup appuser docker


USER appuser

# Expose the application's port
EXPOSE 8000

# Run the application
CMD ["/bin/app"]