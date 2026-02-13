# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/nexus-gateway ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/mock-tools ./cmd/mock-tools
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/migrate ./cmd/migrate
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/cleanup-idempotency ./cmd/cleanup-idempotency

FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates wget
COPY --from=builder /out/nexus-gateway /app/nexus-gateway
COPY --from=builder /out/mock-tools /app/mock-tools
COPY --from=builder /out/migrate /app/migrate
COPY --from=builder /out/cleanup-idempotency /app/cleanup-idempotency
COPY migrations /app/migrations
COPY docs /app/docs

EXPOSE 8080 8081
