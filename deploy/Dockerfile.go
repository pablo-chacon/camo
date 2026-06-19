# Shared multi-stage Dockerfile for all CAMO Go components.
# Used by docker-compose.yml via build.dockerfile.
#
# Build arg COMPONENT selects which binary to build.
# Produces a minimal runtime image — no build tools in the final layer.

ARG GO_VERSION=1.21

# Build stage
FROM golang:${GO_VERSION}-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy go.work and all module source
COPY go.work go.work.sum* ./
COPY camo-gossip/   ./camo-gossip/
COPY camo-simpool/  ./camo-simpool/
COPY camo-apncore/  ./camo-apncore/
COPY camo-wireguard/ ./camo-wireguard/
COPY camo-circuit/  ./camo-circuit/

ARG COMPONENT
RUN cd camo-${COMPONENT} && \
    go build -trimpath -ldflags="-s -w" -o /camo-${COMPONENT} .

# Runtime stage — minimal Alpine
FROM alpine:3.18

RUN apk add --no-cache ca-certificates iproute2

ARG COMPONENT
ENV CAMO_COMPONENT=${COMPONENT}

COPY --from=builder /camo-${COMPONENT} /usr/local/bin/camo-${COMPONENT}

# Create runtime directories
RUN mkdir -p /run/camo /var/lib/camo /etc/camo

ENTRYPOINT ["sh", "-c", "exec /usr/local/bin/camo-${CAMO_COMPONENT}"]
