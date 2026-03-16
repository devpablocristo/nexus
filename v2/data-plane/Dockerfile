# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git

COPY data-plane/go.mod data-plane/go.sum ./data-plane/
COPY pkgs/go-pkg/go.mod pkgs/go-pkg/go.sum ./pkgs/go-pkg/
WORKDIR /src/data-plane
RUN go mod download

WORKDIR /src
COPY data-plane ./data-plane
COPY pkgs/go-pkg ./pkgs/go-pkg

WORKDIR /src/data-plane
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/nexus-data-plane ./cmd/api

FROM alpine:3.23
WORKDIR /app
RUN apk add --no-cache ca-certificates wget

COPY --from=builder /out/nexus-data-plane /app/nexus-data-plane

EXPOSE 8080

CMD ["/app/nexus-data-plane"]
