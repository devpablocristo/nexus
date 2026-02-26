# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git

COPY go.mod go.sum ./
COPY third_party/toollab-adapter-go ./third_party/toollab-adapter-go
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/nexus-core ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/mock-tools ./cmd/mock-tools
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/migrate ./cmd/migrate
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/cleanup-idempotency ./cmd/cleanup-idempotency

FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates wget
RUN addgroup -g 1000 app && adduser -D -u 1000 -G app app
COPY --from=builder /out/nexus-core /app/nexus-core
COPY --from=builder /out/mock-tools /app/mock-tools
COPY --from=builder /out/migrate /app/migrate
COPY --from=builder /out/cleanup-idempotency /app/cleanup-idempotency
COPY migrations /app/migrations
COPY docs /app/docs
RUN chown -R app:app /app

USER app
EXPOSE 8080 8081
HEALTHCHECK --interval=10s --timeout=3s --retries=3 \
  CMD wget -qO- http://localhost:8080/readyz >/dev/null 2>&1 || exit 1
