# =========================
# 1. Builder stage
# =========================
FROM golang:1.25-alpine AS builder

# Минимальные зависимости для сборки
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# Копируем только go.mod и go.sum для кеширования слоя зависимостей
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Копируем весь код
COPY . .

RUN go mod tidy

# Компилируем с максимальной оптимизацией
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -extldflags '-static'" \
    -trimpath \
    -o app \
    ./cmd/api/main.go

# =========================
# 2. Runtime stage (scratch для минимального размера)
# =========================
FROM scratch AS runtime

# Копируем CA сертификаты для HTTPS запросов
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Копируем timezone data (если нужна работа с временными зонами)
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Копируем бинарник
COPY --from=builder /build/app /app

# Если есть миграции/конфиги (раскомментируй при необходимости):
# COPY --from=builder /build/migrations /migrations
# COPY --from=builder /build/configs /configs

# Документируем порт (не открывает, только для информации)
EXPOSE 8080

# Точка входа
ENTRYPOINT ["/app"]