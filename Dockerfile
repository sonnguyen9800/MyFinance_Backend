# Use the official Golang image as the base image
FROM golang:1.20 AS builder

# Set the working directory
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the rest of the application code
COPY . .

# Build the Go application with more verbose output
RUN CGO_ENABLED=0 GOOS=linux go build -v -o my-finance-backend -a -installsuffix cgo

# Start a new stage from scratch
FROM alpine:latest

# Install CA certificates and wget for debugging
RUN apk --no-cache add ca-certificates wget

# Set the working directory
WORKDIR /root/

# Copy the compiled binary from the builder stage
COPY --from=builder /app/my-finance-backend .

# Verify the binary exists
RUN ls -l my-finance-backend

# Make the binary executable
RUN chmod +x my-finance-backend

# Expose the port the app runs on
EXPOSE 8080

# Print debugging information before running
ENTRYPOINT ["/bin/sh", "-c", "pwd && ls -l . && ./my-finance-backend"]