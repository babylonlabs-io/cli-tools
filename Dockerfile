FROM golang:1.21-alpine as builder

# Version to build. Default is the Git HEAD.
ARG VERSION="HEAD"

# Use muslc for static libs
ARG BUILD_TAGS="muslc"


RUN apk add --no-cache --update openssh git make build-base linux-headers libc-dev \
                                pkgconfig zeromq-dev musl-dev alpine-sdk libsodium-dev \
                                libzmq-static libsodium-static gcc

# Build
WORKDIR /go/src/github.com/babylonchain/cli-tools
# Cache dependencies
COPY go.mod go.sum /go/src/github.com/babylonchain/cli-tools/
RUN go mod download
# Copy the rest of the files
COPY ./ /go/src/github.com/babylonchain/cli-tools/

RUN CGO_LDFLAGS="$CGO_LDFLAGS -lstdc++ -lm -lsodium" \
    CGO_ENABLED=1 \
    BUILD_TAGS=$BUILD_TAGS \
    LINK_STATICALLY=true \
    make build

# FINAL IMAGE
FROM alpine:3.16 AS run

RUN addgroup --gid 1138 -S cli-tools && adduser --uid 1138 -S cli-tools -G cli-tools

RUN apk add bash curl jq

COPY --from=builder /go/src/github.com/babylonchain/cli-tools/build/cli-tools /bin/cli-tools

WORKDIR /home/cli-tools
RUN chown -R cli-tools /home/cli-tools
USER cli-tools
