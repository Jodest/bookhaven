FROM golang:1.27.3-alpine AS base
RUN apk add --no-cache git ca-certificates tzdata
WORKDIR /app

# Development stage
FROM base AS dev
RUN go install github.com/air-verse/air@v1.61.5
RUN go install github.com/swaggo/swag/cmd/swag@v1.16.4
COPY go.mod ./
RUN go mod download
COPY . .
EXPOSE 8080
CMD ["air", "-c", ".air.toml"]

# Build stage
FROM base AS builder
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/migrate ./cmd/migrate
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/worker ./cmd/worker
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/harvest ./cmd/harvest
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/similarity ./cmd/similarity
# Empty mount point for the self-hosted covers volume (copied below with the
# nonroot owner: a fresh named volume inherits the owner of its mount point).
RUN mkdir -p /out/data/covers

# Production stage
FROM gcr.io/distroless/static-debian12:nonroot AS production
WORKDIR /app
COPY --from=builder /app/bin/api /app/api
COPY --from=builder /app/bin/migrate /app/migrate
COPY --from=builder /app/bin/worker /app/worker
COPY --from=builder /app/bin/harvest /app/harvest
COPY --from=builder /app/bin/similarity /app/similarity
COPY --from=builder /app/migrations /app/migrations
# Covers volume mount point owned by nonroot (65532): the worker writes there.
COPY --from=builder --chown=65532:65532 /out/data/covers /data/covers
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/api"]
