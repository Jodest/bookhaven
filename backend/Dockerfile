FROM golang:1.23-alpine AS base
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

# Production stage
FROM gcr.io/distroless/static-debian12:nonroot AS production
COPY --from=builder /app/bin/api /api
COPY --from=builder /app/migrations /migrations
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/api"]
