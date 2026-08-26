FROM golang:1.24-alpine AS builder
# The Alpine variant of the Go image ships without make, unlike the Debian one.
RUN apk add --no-cache make
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY Makefile ./
COPY pkg/ pkg/
COPY cmd/ cmd/
RUN make dist

FROM alpine:3.21
LABEL org.opencontainers.image.authors="Maksym Prokopov <mprokopov@gmail.com>"
LABEL org.opencontainers.image.source="https://github.com/mprokopov/dora-exporter"

RUN apk add --no-cache wget \
    && addgroup -S dora && adduser -S -G dora dora

WORKDIR /app
COPY --from=builder /src/dora-exporter .
COPY configs/config.yml.dist config.yml

USER dora

EXPOSE 8090

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8090/metrics >/dev/null 2>&1 || exit 1

ENTRYPOINT ["./dora-exporter"]
