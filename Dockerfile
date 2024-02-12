FROM ghcr.io/celestiaorg/celestia-app:latest AS app

FROM docker.io/golang:1.21-alpine3.18 AS builder
ARG arch=x86_64

# ENV CGO_ENABLED=0
WORKDIR /go/src/app/
COPY . /go/src/app/

RUN apk update && apk add --no-cache \
    llvm \
    clang \
    llvm-static \
    llvm-dev \
    make \
    libbpf \
    libbpf-dev \
    musl-dev

RUN mkdir -p /build/ && \
    make build && \
    cp ./bin/* /build/

#----------------------------#

FROM docker.io/alpine:3.18.4 AS production

RUN apk update && apk add iproute2 curl

# Copy the worker binary
WORKDIR /app/
COPY --from=builder /build .

# Copy all testplan files/resources to the container
COPY ./testplan .

# Copy the target app binary & extra files
COPY --from=app /bin/celestia-appd /bin/celestia-appd
COPY --from=app /opt/entrypoint.sh /opt/entrypoint.sh

EXPOSE 9090 26657 26656 1317 26658 26660 26659 30000

CMD ["/app/testwave"]