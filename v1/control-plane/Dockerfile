# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git

COPY control-plane/go.mod control-plane/go.sum ./control-plane/
COPY pkgs/go-pkg/go.mod pkgs/go-pkg/go.sum ./pkgs/go-pkg/
WORKDIR /src/control-plane
RUN go mod download

WORKDIR /src
COPY control-plane ./control-plane
COPY pkgs/go-pkg ./pkgs/go-pkg

WORKDIR /src/control-plane
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/nexus-saas ./cmd/nexus-saas
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/migrate ./cmd/migrate

FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates wget
RUN addgroup -g 1000 app && adduser -D -u 1000 -G app app

COPY --from=builder /out/nexus-saas /app/nexus-saas
COPY --from=builder /out/migrate /app/migrate
COPY control-plane/migrations /app/migrations
COPY control-plane/docs/openapi.yaml /app/docs/openapi.yaml
RUN chown -R app:app /app

USER app
EXPOSE 8082
HEALTHCHECK --interval=10s --timeout=3s --retries=3 \
  CMD wget -qO- http://localhost:8082/health >/dev/null 2>&1 || exit 1
CMD ["/app/nexus-saas"]
