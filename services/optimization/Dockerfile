FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY proto ./proto
COPY services/optimization/go.mod services/optimization/go.sum ./services/optimization/
WORKDIR /app/services/optimization
RUN go mod download
WORKDIR /app
COPY services/optimization ./services/optimization
WORKDIR /app/services/optimization
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /service ./cmd/api

FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /service /service
COPY services/optimization/migrations /migrations
EXPOSE 8082 9092
ENTRYPOINT ["/service"]
