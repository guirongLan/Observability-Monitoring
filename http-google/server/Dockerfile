# Base image: Use the official Golang image with version 1.24.4
FROM golang:1.24.4

# Set the working directory inside the container to /usr/src/app
# All following commands (COPY, RUN, etc.) will be executed relative to this path
WORKDIR /usr/src/app

# Copy the Go module files from the host to the container
# These are used to manage project dependencies
COPY go.mod go.sum ./

# Download and verify dependencies specified in go.mod and go.sum
RUN go mod download && go mod verify

# Copy all Go source files from the host to the working directory in the container
COPY *.go ./ 

# Build the Go application with verbose output (-v)
# The compiled binary is named 'http-server' and placed in /usr/local/bin
RUN go build -v -o /usr/local/bin/http-server .

# Set an environment variable PORT to 8080 (useful for configuration)
ENV PORT=8080

# Expose port 8080 so it can be accessed from outside the container
EXPOSE 8080

# Command to run when the container starts
# It runs the compiled Go server
CMD ["/usr/local/bin/http-server"]
