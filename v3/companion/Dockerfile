FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /nexus-companion ./cmd/api

FROM alpine:3.23
RUN apk add --no-cache ca-certificates wget
COPY --from=builder /nexus-companion /usr/local/bin/nexus-companion
EXPOSE 8080
CMD ["nexus-companion"]
