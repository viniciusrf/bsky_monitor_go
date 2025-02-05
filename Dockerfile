FROM golang:1.23.4 AS builder

WORKDIR /app

COPY go.mod go.sum ./
COPY vendor/ ./vendor/
COPY src/ ./src/
RUN CGO_ENABLED=0 go build -o monitor ./src

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/monitor .

ENV BSKY_USER="default_user"
ENV BSKY_PASS="default_pass"
ENV BSKY_ACCOUNT="default_account"
RUN chmod +x /app/monitor

VOLUME /app/files

CMD ["./monitor", "monitor_media"]