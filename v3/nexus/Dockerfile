FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /nexus ./cmd/api

FROM alpine:3.23
RUN apk add --no-cache ca-certificates
COPY --from=builder /nexus /usr/local/bin/nexus
EXPOSE 8080
CMD ["nexus"]
