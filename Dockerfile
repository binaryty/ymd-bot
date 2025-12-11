FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/ym-bot ./cmd/bot

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /bin/ym-bot /app/ym-bot
COPY env.example /app/.env.example
ENV TELEGRAM_TOKEN="" \
    YANDEX_TOKEN="" \
    LOG_LEVEL=info
ENTRYPOINT ["/app/ym-bot"]

