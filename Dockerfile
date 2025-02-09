# Start from the official Golang image 
FROM golang:1.23 AS builder

# Set the working directory
WORKDIR /app

# Copy go mod and sum files to download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code into the container
COPY . .

# Build the Go binary
RUN go build -o task-master

# Use a lightweight base image for the final container
FROM alpine:latest

WORKDIR /root/

# Copy the binary from the builder
COPY --from=builder /app/task-master .

# Expose the application's port
EXPOSE 8080

# Run the binary
CMD ["./task-master"]
