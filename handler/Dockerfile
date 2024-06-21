# syntax=docker/dockerfile:1

# Build the application from source
FROM golang:1.22 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY handler/cmd/main.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /handler

# TODO: uncomment when test will be there
# Run the tests in the container
# FROM builder AS tester
# RUN go test -v ./...

# Deploy the application binary into a lean image
FROM golang:1.21 AS release

WORKDIR /

COPY --from=builder /handler /handler

EXPOSE 8080

ENTRYPOINT ["/handler"]