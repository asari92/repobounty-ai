FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o main ./cmd/api

FROM alpine:3.21
RUN apk --no-cache add ca-certificates wget git && \
    adduser -D -u 1001 appuser && \
    mkdir -p /data /data/mirrors && \
    chown -R appuser:appuser /data
WORKDIR /home/appuser
COPY --from=builder /app/main .
USER appuser

EXPOSE 8080
CMD ["./main"]
