#!/bin/sh
# Entrypoint of the standalone image (the Dockerfile's `standalone` target):
# PostgreSQL and the server in the same container — the repository's
# docker-compose reduced to a single image.
#
# The configuration follows EXACTLY the server's rules, described in
# internal/framework/config/env.go. Precedence, strongest first:
#
#   1. the arguments        docker run IMAGE PGDATA=/data FIDELITY=contract
#   2. the environment      docker run -e PGDATA=/data
#   3. the .env file        -v ./.env:/app/.env   or   --env-file /path
#   4. the defaults
#
# This file therefore reinterprets nothing: it reads ahead only the variables
# it needs BEFORE the server starts — the database's data directory, and its
# credentials — then passes the arguments on to the server unchanged, which
# applies the same precedence for everything else.
#
#   docker run -p 8080:8080 -v "$PWD/data:/data" IMAGE PGDATA=/data
#   docker run -p 8080:8080 -v "$PWD/data:/data" -v "$PWD/.env:/app/.env" IMAGE
#   docker run -p 8080:8080 -v "$PWD/prod.env:/cfg.env" IMAGE --env-file /cfg.env
#   docker run -p 8080:8080 -v "$PWD/data:/data" IMAGE /data      # shorthand
set -e

# ── reading a key from a .env file ──────────────────────────────────────────
# Same grammar as parseLine/unquote on the Go side: `export` optional, blank
# lines and comments ignored, enclosing quotes stripped. The FIRST assignment
# wins, as in LoadEnvFile — which never overwrites a value already set.
value_from_file() {
	[ -f "$2" ] || return 1
	awk -v key="$1" '
		{
			line = $0
			sub(/^[ \t]+/, "", line)
			if (line ~ /^#/ || line == "") next
			sub(/^export[ \t]+/, "", line)
			eq = index(line, "=")
			if (eq == 0) next
			name = substr(line, 1, eq - 1)
			sub(/[ \t]+$/, "", name)
			if (name != key) next
			val = substr(line, eq + 1)
			sub(/^[ \t]+/, "", val)
			sub(/[ \t]+$/, "", val)
			n = length(val)
			if (n >= 2) {
				p = substr(val, 1, 1)
				d = substr(val, n, 1)
				if ((p == "\"" && d == "\"") || (p == "'"'"'" && d == "'"'"'")) val = substr(val, 2, n - 2)
			}
			print val
			exit
		}
	' "$2"
}

# ── a bare path as first argument ───────────────────────────────────────────
# A shorthand, and the only argument the server would not know how to read: it
# accepts only --env-file and KEY=value. An existing file is the .env, anything
# else is the data directory. It is removed from the arguments; those that
# follow go to the server untouched.
arg_pgdata=""
arg_envfile=""
case "$1" in
/*)
	if [ -f "$1" ]; then
		arg_envfile="$1"
	else
		arg_pgdata="$1"
	fi
	shift
	;;
esac

# ── re-reading the arguments, without consuming them ────────────────────────
# The server will receive them all: we only spot the ones that concern us.
expect_path=0
for argument in "$@"; do
	if [ "$expect_path" = 1 ]; then
		arg_envfile="$argument"
		expect_path=0
		continue
	fi
	case "$argument" in
	--env-file) expect_path=1 ;;
	--env-file=*) arg_envfile="${argument#--env-file=}" ;;
	PGDATA=*) arg_pgdata="${argument#PGDATA=}" ;;
	POSTGRES_USER=*) arg_user="${argument#POSTGRES_USER=}" ;;
	POSTGRES_PASSWORD=*) arg_password="${argument#POSTGRES_PASSWORD=}" ;;
	POSTGRES_DB=*) arg_db="${argument#POSTGRES_DB=}" ;;
	esac
done

# ── resolution, in precedence order ─────────────────────────────────────────
# An empty value counts as absent, exactly as on the Go side: `-e PGDATA=`
# therefore does not mask the file. That is also why the Dockerfile empties the
# PGDATA that the postgres image sets by default — without it, the environment
# would always win and a PGDATA written in the .env would never be used.
env_file="${arg_envfile:-${ENV_FILE:-.env}}"

resolve() {
	name=$1
	default_value=$2
	value=$3 # what an argument gave
	[ -n "$value" ] && { echo "$value"; return; }
	value=$(eval "printf %s \"\${$name:-}\"")
	[ -n "$value" ] && { echo "$value"; return; }
	value=$(value_from_file "$name" "$env_file" || true)
	[ -n "$value" ] && { echo "$value"; return; }
	echo "$default_value"
}

PGDATA=$(resolve PGDATA /var/lib/postgresql/data "$arg_pgdata")
POSTGRES_USER=$(resolve POSTGRES_USER numflex "${arg_user:-}")
POSTGRES_PASSWORD=$(resolve POSTGRES_PASSWORD numflex "${arg_password:-}")
POSTGRES_DB=$(resolve POSTGRES_DB numflex "${arg_db:-}")
export PGDATA POSTGRES_USER POSTGRES_PASSWORD POSTGRES_DB

# DATABASE_URL is set, not read: the database lives in the container, and a
# value coming from the .env or from an argument would point at a database this
# image does not manage. It is the only server variable the image decides.
DATABASE_URL="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@127.0.0.1:5432/${POSTGRES_DB}?sslmode=disable"
export DATABASE_URL

echo "standalone — data: $PGDATA · database: $POSTGRES_DB · env: $env_file$([ -f "$env_file" ] || echo ' (absent)')"

# ── the database ────────────────────────────────────────────────────────────
# The postgres image's official entrypoint does the whole bootstrap: initdb on
# an empty directory, creation of the role and the database, chown, then
# execution under the postgres user. Reusing it rather than rewriting it avoids
# reinventing an initialisation that has ten years of fixes behind it.
#
# listen_addresses=127.0.0.1: the database listens inside the container only.
# The sandbox publishes 8080, never 5432 — the API is the single door.
docker-entrypoint.sh postgres -c listen_addresses=127.0.0.1 &

# Availability is observed, not assumed: an initdb on an empty volume takes a
# few seconds, and the first migration would fail against a database that does
# not accept a connection yet.
attempt=0
until pg_isready -h 127.0.0.1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" >/dev/null 2>&1; do
	attempt=$((attempt + 1))
	if [ "$attempt" -gt 120 ]; then
		echo "postgres did not accept a connection within 60 s" >&2
		exit 1
	fi
	sleep 0.5
done

# ── the server ──────────────────────────────────────────────────────────────
# Clean shutdown of both processes: without it, a `docker stop` would kill
# Postgres with SIGKILL after the grace period, leaving the data directory to
# recover on the next start.
shutdown() {
	kill -TERM "$server_pid" 2>/dev/null || true
	wait "$server_pid" 2>/dev/null || true
	su-exec postgres pg_ctl -D "$PGDATA" -m fast stop >/dev/null 2>&1 || true
	exit 0
}
trap shutdown TERM INT

# The server runs the migrations then the seed, and looks for migrations/ by
# walking up from the current directory — hence the image's WORKDIR /app.
su-exec postgres /usr/local/bin/server "$@" &
server_pid=$!
wait "$server_pid"
