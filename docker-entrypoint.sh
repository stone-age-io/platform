#!/bin/sh
# First-boot seeding, then serve. Nothing else.
#
# `serve --nats` needs a nats.conf, and nothing can write one until the database
# has an operator in it. `stone-age init` does that seeding: the four first-run
# commands in their load-bearing order (see init.go).
set -eu

DATA="${STONE_AGE_DATA_DIR:-/data}"
PB_DIR="$DATA/pb_data"
NATS_DIR="$DATA/nats-config"

# nats.conf is the marker rather than the database directory, because it is
# precisely what serve needs and does not have. Using it also means an operator
# who edits the generated config keeps their edits across restarts. init checks
# the same marker; checking it here as well spares every later start a run of
# the app just to find out there is nothing to do.
if [ ! -f "$NATS_DIR/nats.conf" ]; then
  EMAIL="${STONE_AGE_BOOTSTRAP_EMAIL:-admin@example.com}"

  if [ -z "${STONE_AGE_BOOTSTRAP_PASSWORD:-}" ]; then
    echo "This is a first boot and STONE_AGE_BOOTSTRAP_PASSWORD is not set." >&2
    echo "" >&2
    echo "  docker run ... -e STONE_AGE_BOOTSTRAP_PASSWORD='at-least-8-chars' ..." >&2
    echo "" >&2
    echo "It creates the PocketBase superuser and the platform operator account," >&2
    echo "both as $EMAIL. Change it after logging in." >&2
    exit 1
  fi

  echo "==> First boot: seeding $DATA"

  # init reads the password from STONE_AGE_BOOTSTRAP_PASSWORD. --operator-org
  # is passed explicitly because leaving it out makes init prompt, and there is
  # no terminal here.
  stone-age init \
    --dir "$PB_DIR" \
    --nats-config "$NATS_DIR/nats.conf" \
    --email "$EMAIL" \
    --org "${STONE_AGE_BOOTSTRAP_ORG:-System}" \
    --operator-org "${STONE_AGE_BOOTSTRAP_OPERATOR_ORG:-Operator}"
fi

# exec, so the binary is PID 1 and gets SIGTERM directly: PocketBase owns
# signal handling and stops the embedded NATS server on the way down.
exec stone-age serve \
  --http "0.0.0.0:${STONE_AGE_HTTP_PORT:-8090}" \
  --dir "$PB_DIR" \
  --nats \
  --nats-config "$NATS_DIR/nats.conf" \
  "$@"
