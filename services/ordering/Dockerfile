FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY proto ./proto
COPY services/ordering/go.mod services/ordering/go.sum ./services/ordering/
WORKDIR /app/services/ordering
RUN go mod download
WORKDIR /app
COPY services/ordering ./services/ordering
WORKDIR /app/services/ordering
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /service ./cmd/api

FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /service /service
COPY services/ordering/migrations /migrations
EXPOSE 8083 9093
ENTRYPOINT ["/service"]
