# syntax=docker/dockerfile:1.6

FROM golang:1.22-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Сборка статического бинарника
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /out/server ./cmd/server

# Рантайм-слой (минимальный)
FROM alpine:3.20
RUN adduser -D -u 10001 app
WORKDIR /app

COPY --from=builder /out/server /app/server

ENV PORT=3282
EXPOSE 3282
USER app

# Опционально: healthcheck (если есть /health)
HEALTHCHECK --interval=30s --timeout=3s --retries=3 CMD wget -qO- http://127.0.0.1:${PORT}/health || exit 1

ENTRYPOINT ["/app/server"]
