
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o main .


FROM alpine:latest
WORKDIR /root/

COPY --from=builder /app/main .

COPY --from=builder /app/db/migrations ./db/migrations

CMD ["./main"]