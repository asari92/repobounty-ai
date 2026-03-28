# Build stage
FROM golang:alpine AS builder

WORKDIR /app

# Install necessary build tools
RUN apk add --no-cache gcc musl-dev

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o api ./cmd/api

# Final stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS (e.g., GitHub API, OpenAI API)
RUN apk --no-cache add ca-certificates

# Copy the binary from the builder stage
COPY --from=builder /app/api .

# Expose the API port
EXPOSE 8080

# Command to run the executable
CMD ["./api"]