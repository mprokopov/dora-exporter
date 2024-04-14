FROM golang:1.19.2 as builder
WORKDIR /usr/local/go/src/dora-exporter
COPY go.mod ./
COPY go.sum ./
COPY Makefile ./
COPY pkg/ pkg/
COPY cmd/ cmd/
RUN make deps
RUN make dist

FROM alpine:3.16.2
MAINTAINER Maksym Prokopov <mprokopov@gmaio.com>
COPY --from=builder /usr/local/go/src/dora-exporter/dora-exporter .
COPY configs/config.yml.dist config.yml

EXPOSE 8090

ENTRYPOINT ["./dora-exporter"]
