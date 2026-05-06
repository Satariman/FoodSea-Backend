FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY proto ./proto
COPY services/core/go.mod services/core/go.sum ./services/core/
WORKDIR /app/services/core
RUN go mod download
WORKDIR /app
COPY services/core ./services/core
WORKDIR /app/services/core
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /service ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /seed ./cmd/seed

FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /service /service
COPY --from=builder /seed /seed
COPY services/core/migrations /migrations
EXPOSE 8081 9091
ENTRYPOINT ["/service"]
