# Build stage
FROM golang:1.25-alpine AS builder

# Set the current working directory inside the container
WORKDIR /app

# Install necessary packages
RUN apk add --no-cache ca-certificates tzdata git

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source code into the container
COPY . .

# Build the Go app with CGO disabled for a statically linked binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o server ./cmd

# Final stage (Lightweight Alpine image)
FROM alpine:latest

# Set the current working directory
WORKDIR /app

# Copy the Pre-built binary file and CA certificates from the previous stage
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /app/server .

# Expose port 8000 to the outside world
EXPOSE 8000

# Command to run the executable
CMD ["./server"]
