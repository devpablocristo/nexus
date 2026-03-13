# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git

COPY control-workers/go.mod control-workers/go.sum ./control-workers/
COPY pkgs/go-pkg/go.mod pkgs/go-pkg/go.sum ./pkgs/go-pkg/
WORKDIR /src/control-workers
RUN go mod download

WORKDIR /src
COPY control-workers ./control-workers
COPY pkgs/go-pkg ./pkgs/go-pkg

WORKDIR /src/control-workers
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/nexus-control-operators ./cmd/ops-workers

FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates
RUN addgroup -g 1000 app && adduser -D -u 1000 -G app app
COPY --from=builder /out/nexus-control-operators /app/nexus-control-operators
RUN mkdir -p /app/data && chown -R app:app /app

USER app
EXPOSE 8090
ENTRYPOINT ["/app/nexus-control-operators"]
