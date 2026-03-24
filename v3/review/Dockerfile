FROM golang:1.26-alpine AS builder

WORKDIR /workspace

COPY core/ai/go/ core/ai/go/
COPY core/backend/go/ core/backend/go/
COPY core/databases/postgres/go/ core/databases/postgres/go/
COPY nexus/v3/review/ nexus/v3/review/

WORKDIR /workspace/nexus/v3/review
RUN go mod download
RUN CGO_ENABLED=0 go build -o /nexus-review ./cmd/api

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /nexus-review /usr/local/bin/nexus-review
EXPOSE 8080
CMD ["nexus-review"]
