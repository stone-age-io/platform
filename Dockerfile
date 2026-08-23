# One container. The binary runs the bus.
#
# `serve --nats` starts a real nats-server in this process from the same
# nats.conf that `nats export` writes (internal/natsd), so there is no second
# container and no compose file: the whole pitch of the product is that this is
# one moving part, and a stack that spawns a separate nats-server to do what a
# flag already does argues against itself.
#
#   docker build -t stone-age .
#   docker run -d --name stone-age \
#     -p 8090:8090 -p 4222:4222 -p 9222:9222 \
#     -v stone-age-data:/data \
#     -e STONE_AGE_BOOTSTRAP_PASSWORD='change-me-8-chars-min' \
#     -e STONE_AGE_NATS_WEBSOCKET_URLS='ws://localhost:9222' \
#     stone-age
#
# STONE_AGE_NATS_WEBSOCKET_URLS is the address a BROWSER dials, which this
# container cannot know: it is the host's name or address as the operator's users
# reach it, not anything visible from inside. Nothing else is required.

# ----------------------------------------------------------------- 1. console
FROM node:24-alpine AS ui
WORKDIR /src/ui

# Dependencies first, so a source-only change does not re-run npm ci.
COPY ui/package.json ui/package-lock.json ./
RUN npm ci

COPY ui/ ./
# vue-tsc, then vite build into ../pb_public -- which the Go build embeds.
RUN npm run build

# ---------------------------------------------------------------- 2. binaries
FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=ui /src/pb_public ./pb_public

# CGO_ENABLED=0 is free here: PocketBase's SQLite driver is modernc.org/sqlite,
# a pure-Go translation, so there is no C toolchain to satisfy and the result is
# a static binary. That is also why cross-compiling releases costs nothing.
ARG VERSION=dev
RUN CGO_ENABLED=0 go build \
      -trimpath \
      -ldflags "-s -w -X platform/internal/version.Version=${VERSION}" \
      -o /out/stone-age .

# ----------------------------------------------------------------- 3. runtime
FROM alpine:3

# Alpine rather than distroless: the entrypoint has real work to do on first
# boot (see docker-entrypoint.sh) and needs a shell. ca-certificates is for
# outbound TLS -- OAuth2 providers and SMTP.
#
# The floating `alpine:3` tag rather than a pinned minor, on purpose: it always
# resolves to the current stable, so the image picks up base security fixes
# without anyone here watching Alpine release notes. Pinning a minor on a
# one-maintainer project mostly means shipping an EOL base two years from now.
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -H -u 10001 stoneage

COPY --from=build /out/stone-age /usr/local/bin/stone-age
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

# chmod even though the file is committed 100755. COPY preserves the source
# mode, and the mode is exactly the thing a Windows checkout drops:
# core.filemode defaults to false there, so a contributor re-adding this file
# can silently clear the bit. The symptom is a container that exits immediately
# with "permission denied" and nothing else, which is a bad way to learn about
# a file permission.
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

# Everything that must survive the container: the PocketBase database, the
# generated NATS config, the account JWTs the resolver writes, and the JetStream
# store. `nats export --output` resolves the resolver directory and the JetStream
# store inside its output directory, so one volume covers all four.
RUN mkdir -p /data && chown stoneage /data
VOLUME /data
USER stoneage

# 8090 console + REST, 4222 NATS clients, 9222 NATS over WebSocket (browsers
# cannot speak the NATS TCP protocol, so the console needs this one).
EXPOSE 8090 4222 9222

# /api/ready rather than PocketBase's /api/health, which only reports that the
# HTTP server is listening -- true of every failure worth catching here. This
# answers 503 while the operator is unseeded, the schema never imported, or the
# NATS server does not trust this platform's operator.
#
# busybox wget is already in the base image and exits non-zero on a 503, so no
# curl and no shell arithmetic are needed.
#
# start-period covers first boot: the entrypoint seeds the superuser, imports
# the schema, bootstraps the operator and runs `nats export` before serving, and
# on a slow disk that is not quick. Failed probes inside the period do not count
# against retries.
#
# Note this marks the container unhealthy, it does not restart it -- plain
# `docker run` takes no action on a health status. That is the right default:
# most of what this endpoint reports (unseeded operator, unreachable bus) is not
# fixed by a restart, and an orchestrator configured to restart on unhealthy
# would loop instead of surfacing the actual problem.
HEALTHCHECK --interval=30s --timeout=5s --start-period=90s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8090/api/ready || exit 1

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
