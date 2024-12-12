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

# Build the Go application
RUN go build -o ./my-finance-backend
# Start a new stage from scratch
FROM alpine:latest

# Set the working directory
WORKDIR /root/

# Copy the compiled binary from the builder stage
COPY --from=builder /app/my-finance-backend .

# Copy the .env files
COPY .env ./
COPY .env.prod ./

RUN chmod +x ./my-finance-backend

# Expose the port the app runs on
EXPOSE 8080

# Command to run the executable
CMD ["./my-finance-backend"]