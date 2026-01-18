# =============================================================================
# staff-service - Dockerfile
# =============================================================================

FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -a -installsuffix cgo -o staff-service ./cmd

FROM alpine:3.19 AS production

RUN apk --no-cache add ca-certificates wget tzdata

# Install golang-migrate for database migrations
RUN wget -q https://github.com/golang-migrate/migrate/releases/download/v4.17.0/migrate.linux-amd64.tar.gz \
    && tar -xzf migrate.linux-amd64.tar.gz \
    && mv migrate /usr/local/bin/migrate \
    && rm migrate.linux-amd64.tar.gz \
    && chmod +x /usr/local/bin/migrate

RUN addgroup -g 1000 -S appgroup && adduser -D -u 1000 -S appuser -G appgroup

WORKDIR /app

COPY --from=builder /build/staff-service .
COPY --from=builder /build/migrations ./migrations/

RUN chown -R appuser:appgroup /app

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./staff-service"]
