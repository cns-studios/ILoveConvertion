# ── Build Stage ──
FROM golang:1.22-bookworm AS builder

WORKDIR /build

COPY . .

RUN go mod tidy

RUN mkdir -p /out && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/api ./cmd/api

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app
COPY --from=builder /out/api .

RUN mkdir -p /app/storage/inputs /app/storage/outputs && \
    chown -R appuser:appgroup /app

USER appuser

EXPOSE 3000

CMD ["./api"]