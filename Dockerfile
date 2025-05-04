FROM golang:1.23.1-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

# Загружаем зависимости
RUN go mod download

COPY . .


RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/server ./cmd/main.go


FROM alpine:latest

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем только скомпилированный бинарник из этапа сборщика
COPY --from=builder /app/server /app/server

# Указываем порт, который будет слушать наше приложение.
# ЗАМЕНИ 8080 на порт, который использует твое приложение (из конфигурации).
EXPOSE 8080

# Команда для запуска приложения при старте контейнера
CMD ["/app/server"]