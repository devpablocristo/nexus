# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git

COPY nexus-core/go.mod nexus-core/go.sum ./nexus-core/
COPY pkgs/go-pkg/go.mod pkgs/go-pkg/go.sum ./pkgs/go-pkg/
WORKDIR /src/nexus-core
RUN go mod download

WORKDIR /src
COPY nexus-core ./nexus-core
COPY pkgs/go-pkg ./pkgs/go-pkg

WORKDIR /src/nexus-core
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/nexus-core ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/mock-tools ./cmd/mock-tools
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/migrate ./cmd/migrate
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/cleanup-idempotency ./cmd/cleanup-idempotency

FROM alpine:3.23
WORKDIR /app
RUN apk add --no-cache ca-certificates wget
RUN addgroup -g 1000 app && adduser -D -u 1000 -G app app
COPY --from=builder /out/nexus-core /app/nexus-core
COPY --from=builder /out/mock-tools /app/mock-tools
COPY --from=builder /out/migrate /app/migrate
COPY --from=builder /out/cleanup-idempotency /app/cleanup-idempotency
COPY nexus-core/migrations /app/migrations
COPY nexus-core/docs/openapi.yaml /app/docs/openapi.yaml
RUN chown -R app:app /app

USER app
EXPOSE 8080 8081
HEALTHCHECK --interval=10s --timeout=3s --retries=3 \
  CMD wget -qO- http://localhost:8080/readyz >/dev/null 2>&1 || exit 1
