FROM golang:1.23.1-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

COPY templates/ templates/

RUN go mod download

COPY . .


RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/server ./cmd/main.go


FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/server /app/server

EXPOSE 8080

# Команда для запуска приложения при старте контейнера
CMD ["/app/server"]
