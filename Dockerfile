# syntax=docker/dockerfile:1

# ──────────────────────────────── build ─────────────────────────────────────
# The version follows go.mod's `go` directive — pgx v5.10 requires 1.25.
# Letting them drift breaks the image build without breaking the local one.
ARG GO_VERSION=1.25

# --platform=$BUILDPLATFORM: compilation stays native, the target comes from
# GOOS/GOARCH. Building an arm64 image from amd64 therefore does not go
# through QEMU emulation, which costs ten times the build time.
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS build

WORKDIR /src

# Dependencies are copied on their own: as long as go.mod and go.sum do not
# move, this layer is reused as is.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH
# CGO_ENABLED=0    : no C dependency — pgx is pure Go — hence a static
#                    binary, runnable from a `scratch` image.
# -trimpath        : strips the build machine's paths, the binary becomes
#                    reproducible.
# -ldflags='-s -w' : strips the symbol table and the DWARF info, ~25 % less.
#                    The price: no symbolised stack trace any more — the
#                    sandbox is not a production service.
# -tags timetzdata : embeds the timezone database (~450 KB). Without it, on an
#                    image with no /usr/share/zoneinfo, a TZ=Africa/Dakar
#                    would be silently ignored and timestamps would come out
#                    in UTC.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    mkdir -p /out && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -tags timetzdata -ldflags='-s -w' -o /out/ ./cmd/...

# An unprivileged user: `scratch` has no /etc/passwd, and some pgx paths query
# the current user when the URL carries none.
RUN printf 'numflex:x:10001:10001::/app:/sbin/nologin\n' > /out/passwd && \
    printf 'numflex:x:10001:\n'                          > /out/group

# ─────────────────────────── runtime, all-in-one ────────────────────────────
# Optional target: PostgreSQL and the server in the same image — the
# repository's docker-compose reduced to a single container, for whoever wants
# to run the sandbox without orchestrating anything. It is NOT the default
# target: `runtime`, further down, is, and stays what `docker compose`,
# `make image` and `make push` produce. This one is asked for explicitly:
#
#   docker build --target standalone -t numflex-sandbox:standalone .
#   docker run -p 8080:8080 -v "$PWD/data:/data" numflex-sandbox:standalone PGDATA=/data
#
# The database's data directory is set like anything else — argument,
# environment variable or .env — following the precedence described in
# internal/framework/config/env.go, which scripts/standalone-entrypoint.sh
# reproduces identically rather than inventing a second convention.
#
# What it costs, and what must be accepted: ~44 MB instead of ~11, a shell
# and a package manager in the image, a start as root for the duration of
# initdb. In other words everything `runtime` refuses. It is a demonstration
# convenience, not a hardening — not to be exposed on an open network.
# Two stages for one image, because deleting a file in a layer above the one
# that carries it saves nothing: the bytes stay in the lower layer, and only
# a whiteout is added. `pgtrim` strips, then `standalone` copies the result
# as a single flat layer — that is what actually shrinks the image.
FROM postgres:16-alpine AS pgtrim

# PostgreSQL's JIT is 180 MB of LLVM for queries this sandbox never runs:
# every statement here is a point read or one sequential scan, and the
# planner only reaches for the JIT far above those costs. The bitcode and
# the module that loads it go with it, and the entrypoint starts the server
# with `-c jit=off` so nothing ever looks for them.
#
# The C headers, the static libraries and the man pages go too: no extension
# is compiled in this image, and nothing reads a man page in a container.
RUN rm -rf /usr/lib/libLLVM*.so* \
           /usr/local/lib/postgresql/bitcode \
           /usr/local/lib/postgresql/llvmjit*.so \
           /usr/local/lib/*.a \
           /usr/local/include \
           /usr/local/share/doc \
           /usr/local/share/man

FROM scratch AS standalone

# The whole trimmed filesystem, in one layer. COPY --from preserves
# ownership and permissions, so the postgres user, /var/run/postgresql and
# the data directory keep exactly what the official image gave them.
COPY --from=pgtrim / /

# What the postgres image declared and `scratch` does not inherit. PGDATA is
# deliberately empty rather than absent — see the comment further down.
ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
    LANG=en_US.utf8 \
    PGDATA=""
STOPSIGNAL SIGTERM

COPY --from=build /out/server /out/artp /usr/local/bin/
COPY migrations /app/migrations
COPY scripts/standalone-entrypoint.sh /usr/local/bin/standalone-entrypoint.sh

# The documentation, served by the server itself on the API's own port. No
# extra package, no second process, no reverse proxy: a proxy in front would
# stamp Server and Connection on all 33 contract responses, which carry
# exactly three headers today. The server registers these three root paths
# only because this folder is here — `runtime` ships none, so the same binary
# answers 404 there, which is the platform's exact surface.
COPY docs/swagger.html docs/openapi.yaml docs/openapi.json /app/docs/

# PGDATA is set empty above, not left to the postgres image's
# /var/lib/postgresql/data: an environment value wins over the file, so a
# PGDATA written in a .env would never be used. Empty counts as absent, as
# everywhere else in the sandbox's configuration; the entrypoint falls back
# on the same default when nobody decides.

# The server looks for migrations/ and .env by walking up from the current
# directory: /app/.env is therefore the file read by default, on the entrypoint
# side as on the server side.
WORKDIR /app

# One port. 5432 is not published, and the database listens on 127.0.0.1 only
# anyway: the API is the container's single door.
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/standalone-entrypoint.sh"]

LABEL org.opencontainers.image.title="numflex-sandbox (all-in-one)" \
      org.opencontainers.image.description="The NumFlex sandbox and its PostgreSQL database in a single image" \
      org.opencontainers.image.source="https://github.com/ouznoreyni/numflex-sandbox"

# ─────────────────────────────── runtime ────────────────────────────────────
# `scratch` rather than alpine: nothing but what is listed below enters the
# image — no busybox, no package manager, hence no base CVE to track and
# nothing to run for whoever would get hold of the container.
FROM scratch AS runtime

# The trust roots, without which a DATABASE_URL in `sslmode=verify-full`
# would fail. ~250 KB, the only non-negotiable weight.
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/passwd /out/group /etc/

COPY --from=build /out/server /out/artp /usr/local/bin/
# The server is the schema's sole owner: it runs the migrations at startup and
# looks for them by walking up from the current directory, hence the WORKDIR
# below.
COPY migrations /app/migrations

WORKDIR /app
USER 10001:10001
EXPOSE 8080

# No configuration variable is set here, deliberately: an `ENV PORT=8080`
# would mask the value of a mounted .env, since the environment wins over the
# file.
#
# No HEALTHCHECK either: the sandbox exposes no health route — the real
# platform has none, and the surface must stay identical — and `scratch`
# offers no shell to exercise one. Probe it from the outside, through
# POST /api/authenticate.
ENTRYPOINT ["/usr/local/bin/server"]

LABEL org.opencontainers.image.title="numflex-sandbox" \
      org.opencontainers.image.description="Local double of the ARTP NumFlex API Gateway (guide v2)" \
      org.opencontainers.image.source="https://github.com/ouznoreyni/numflex-sandbox"
