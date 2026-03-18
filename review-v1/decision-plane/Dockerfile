FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY v2/pkgs/go-pkg/ v2/pkgs/go-pkg/
COPY review-v1/decision-plane/ review-v1/decision-plane/

WORKDIR /app/review-v1/decision-plane
RUN go mod download
RUN CGO_ENABLED=0 go build -o /nexus-decision-plane ./cmd/api

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /nexus-decision-plane /usr/local/bin/nexus-decision-plane
EXPOSE 8080
CMD ["nexus-decision-plane"]
