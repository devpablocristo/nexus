FROM golang:1.26-alpine AS builder

WORKDIR /workspace

COPY core/ai/go/ core/ai/go/
COPY core/errors/go/ core/errors/go/
COPY core/http/go/ core/http/go/
COPY core/observability/go/ core/observability/go/
COPY core/security/go/ core/security/go/
COPY core/concurrency/go/ core/concurrency/go/
COPY core/config/go/ core/config/go/
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
