FROM golang:1.22.7-alpine as builder

# Version to build. Default is the Git HEAD.
ARG VERSION="HEAD"

# Use muslc for static libs
ARG BUILD_TAGS="muslc"

# hadolint ignore=DL3018
RUN apk add --no-cache --update openssh git make build-base linux-headers libc-dev \
                                pkgconfig zeromq-dev musl-dev alpine-sdk libsodium-dev \
                                libzmq-static libsodium-static gcc \
                                && rm -rf /var/cache/apk/*

# Build
WORKDIR /go/src/github.com/babylonlabs-io/cli-tools
# Cache dependencies
COPY go.mod go.sum /go/src/github.com/babylonlabs-io/cli-tools/
RUN go mod download
# Copy the rest of the files
COPY ./ /go/src/github.com/babylonlabs-io/cli-tools/

RUN CGO_LDFLAGS="$CGO_LDFLAGS -lstdc++ -lm -lsodium" \
    CGO_ENABLED=1 \
    BUILD_TAGS=$BUILD_TAGS \
    LINK_STATICALLY=true \
    make build

# FINAL IMAGE
FROM alpine:3.20 AS run

# hadolint ignore=DL3018
RUN addgroup --gid 1138 -S cli-tools && adduser --uid 1138 -S cli-tools -G cli-tools \
    && apk --no-cache add bash curl jq && rm -rf /var/cache/apk/*

COPY --from=builder /go/src/github.com/babylonlabs-io/cli-tools/build/cli-tools /bin/cli-tools

WORKDIR /home/cli-tools
RUN chown -R cli-tools /home/cli-tools
USER cli-tools
