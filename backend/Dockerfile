FROM golang:1.24-alpine

# Install build essentials
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN go build -o bin/server cmd/server/main.go

# Expose port
EXPOSE 8080

# Start the server
CMD ["./bin/server"] 