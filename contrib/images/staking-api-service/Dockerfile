FROM golang:1.21-alpine AS builder

ARG VERSION="HEAD"

RUN apk add --no-cache  \
    make \
    git \
    build-base \
    linux-headers \
    libc-dev \
    pkgconfig \
    alpine-sdk \
    libsodium-dev \
    libsodium-static \
    openssh

# Build
WORKDIR /go/src/github.com/babylonchain/cli-tools
# Cache dependencies
COPY go.mod go.sum /go/src/github.com/babylonchain/cli-tools/

# Copy the rest of the files
COPY ./ /go/src/github.com/babylonchain/cli-tools/


RUN LDFLAGS='-extldflags "-static" -v' \
    CGO_ENABLED=1 \
    CGO_LDFLAGS="$CGO_LDFLAGS -lstdc++ -lm -lsodium" \
    LINK_STATICALLY=true \
    BUILD_TAGS="muslc" \
    make build

# Final minimal image with binary only
FROM alpine:3.16 as run

RUN addgroup --gid 1138 -S cli-tools && adduser --uid 1138 -S cli-tools -G cli-tools
RUN apk add bash curl jq

# Label should match your github repo
LABEL org.opencontainers.image.source="https://github.com/babylonchain/cli-tools:${VERSION}"


# Copy over binaries from the build-env
COPY --from=builder /go/src/github.com/babylonchain/cli-tools/build/cli-tools /bin/cli-tools
COPY --from=builder /go/src/github.com/babylonchain/cli-tools/contrib/images/cli-tools/entrypoint.sh /bin/entrypoint.sh

WORKDIR /home/cli-tools
RUN chown -R cli-tools /home/cli-tools
USER cli-tools

ENTRYPOINT ["/bin/entrypoint.sh"]

