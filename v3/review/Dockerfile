FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY pkgs/go-pkg/ pkgs/go-pkg/
COPY review/ review/

WORKDIR /app/review
RUN go mod download
RUN CGO_ENABLED=0 go build -o /nexus-review ./cmd/api

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /nexus-review /usr/local/bin/nexus-review
EXPOSE 8080
CMD ["nexus-review"]
