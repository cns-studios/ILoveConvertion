FROM golang:1.22-bookworm AS builder

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        libvips-dev \
        pkg-config \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build
COPY go.* ./
RUN go mod download 2>/dev/null || true
COPY . .

RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/worker ./cmd/worker

FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        tzdata \
        libvips42 \
        ffmpeg \
        ghostscript \
        qpdf \
        pngquant \
    && rm -rf /var/lib/apt/lists/*

RUN groupadd -r appgroup && useradd -r -g appgroup appuser

WORKDIR /app
COPY --from=builder /out/worker .

RUN mkdir -p /app/storage/inputs /app/storage/outputs /tmp/processing && \
    chown -R appuser:appgroup /app /tmp/processing

USER appuser

CMD ["./worker"]