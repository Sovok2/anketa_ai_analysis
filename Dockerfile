# syntax=docker/dockerfile:1.6

FROM golang:1.25-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Сборка статического бинарника
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /out/server ./cmd

# Рантайм-слой (минимальный)
FROM alpine:3.20
RUN adduser -D -u 10001 app
WORKDIR /app

COPY --from=builder /out/server /app/server

ENV PORT=3282
EXPOSE 3282
USER app

ENTRYPOINT ["/app/server"]
