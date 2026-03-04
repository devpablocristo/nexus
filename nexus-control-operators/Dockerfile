# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git

COPY nexus-control-operators/go.mod nexus-control-operators/go.sum ./nexus-control-operators/
WORKDIR /src/nexus-control-operators
RUN go mod download

WORKDIR /src
COPY nexus-control-operators ./nexus-control-operators

WORKDIR /src/nexus-control-operators
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/nexus-control-operators ./cmd/ops-workers

FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates
RUN addgroup -g 1000 app && adduser -D -u 1000 -G app app
COPY --from=builder /out/nexus-control-operators /app/nexus-control-operators
RUN chown -R app:app /app

USER app
ENTRYPOINT ["/app/nexus-control-operators"]
