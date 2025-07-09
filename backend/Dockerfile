FROM golang:1.24-alpine

# Install build essentials
RUN apk add --no-cache git

# Create a non-root user for security
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Create logs and uploads directories with proper permissions
RUN mkdir -p logs uploads tmp && \
    chown -R appuser:appgroup logs uploads tmp && \
    chmod -R 755 logs uploads tmp

# Build the application
RUN go build -o bin/server cmd/server/main.go && \
    chown appuser:appgroup bin/server

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Start the server
CMD ["./bin/server"] 