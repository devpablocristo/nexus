# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git

COPY control-workers/go.mod control-workers/go.sum ./control-workers/
WORKDIR /src/control-workers
RUN go mod download

WORKDIR /src
COPY control-workers ./control-workers

WORKDIR /src/control-workers
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/nexus-control-workers ./cmd/api

FROM alpine:3.21
WORKDIR /app
RUN apk add --no-cache ca-certificates wget

COPY --from=builder /out/nexus-control-workers /app/nexus-control-workers

EXPOSE 8082

CMD ["/app/nexus-control-workers"]
