FROM golang:1.25-alpine AS builder

LABEL org.opencontainers.image.source="https://github.com/dahhou-ilyas/host-container"

ENV CGO_ENABLED=0 \
    GOOS=linux

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -ldflags="-s -w" -o /app .

# ── Final image ──────────────────────────────────────────────────────────────
FROM alpine:3.21 AS final

LABEL org.opencontainers.image.title="docker-wrapper" \
      org.opencontainers.image.description="Docker container management API" \
      org.opencontainers.image.source="https://github.com/dahhou-ilyas/host-container"

RUN addgroup -S -g 1001 appgroup && \
    adduser  -S -u 1001 -G appgroup appuser

# Install gosu (arch-aware) for proper supplementary group support
RUN set -eux; \
    apk add --no-cache curl wget; \
    case "$(uname -m)" in \
        x86_64)  ARCH=amd64 ;; \
        aarch64) ARCH=arm64 ;; \
        armv7l)  ARCH=armhf ;; \
        *) echo "unsupported arch: $(uname -m)" && exit 1 ;; \
    esac; \
    wget -q -O /usr/local/bin/gosu \
        "https://github.com/tianon/gosu/releases/download/1.17/gosu-${ARCH}"; \
    chmod +x /usr/local/bin/gosu; \
    gosu nobody true

COPY --from=builder /app /bin/app
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -f http://localhost:8080/health || exit 1

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["/bin/app"]
