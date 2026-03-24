FROM golang:1.26-alpine AS builder

WORKDIR /workspace

COPY core/backend/go/ core/backend/go/
COPY core/databases/postgres/go/ core/databases/postgres/go/
COPY core/governance/go/ core/governance/go/
COPY nexus/v3/companion/ nexus/v3/companion/

WORKDIR /workspace/nexus/v3/companion
RUN go mod download
RUN CGO_ENABLED=0 go build -o /nexus-companion ./cmd/api

FROM alpine:3.21
RUN apk add --no-cache ca-certificates wget
COPY --from=builder /nexus-companion /usr/local/bin/nexus-companion
EXPOSE 8080
CMD ["nexus-companion"]
