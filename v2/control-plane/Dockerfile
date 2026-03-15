# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS builder
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
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/nexus-control-plane ./cmd/api

FROM alpine:3.21
WORKDIR /app
RUN apk add --no-cache ca-certificates wget

COPY --from=builder /out/nexus-control-plane /app/nexus-control-plane

EXPOSE 8081

CMD ["/app/nexus-control-plane"]
