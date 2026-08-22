#!/bin/sh
# First-boot seeding, then serve. Nothing else.
#
# This script exists for one reason: `serve --nats` needs a nats.conf, and
# `nats export` cannot write one until the database has an operator in it. That
# is the chicken-and-egg the getting-started docs walk you through by hand, and
# it is the only part of running this container that is not just "start the
# binary".
#
# The four seeding commands are the same four from the README, in the same
# order, which is load-bearing: `bootstrap` writes is_operator / is_system_org /
# is_operator_org, and those fields only exist once the schema is imported.
# PocketBase silently drops writes to fields that do not exist, so running
# bootstrap before migrate yields a platform with no operator and no error.
set -eu

DATA="${STONE_AGE_DATA_DIR:-/data}"
PB_DIR="$DATA/pb_data"
NATS_DIR="$DATA/nats-config"

# nats.conf is the marker rather than the database directory, because it is
# precisely what the next line needs and does not have. Using it also means an
# operator who edits the generated config keeps their edits across restarts --
# `nats export` would overwrite them.
if [ ! -f "$NATS_DIR/nats.conf" ]; then
  EMAIL="${STONE_AGE_BOOTSTRAP_EMAIL:-admin@example.com}"
  ORG="${STONE_AGE_BOOTSTRAP_ORG:-System}"
  OPERATOR_ORG="${STONE_AGE_BOOTSTRAP_OPERATOR_ORG:-Operator}"

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

  # PocketBase superuser, which also seeds the NATS \$SYS operator records that
  # `bootstrap` links up and `nats export` reads.
  stone-age superuser upsert "$EMAIL" "$STONE_AGE_BOOTSTRAP_PASSWORD" --dir "$PB_DIR"

  # Import schema.json. The API rules it carries are the platform's entire
  # authorization layer, so this is not optional in any deployment.
  stone-age migrate up --dir "$PB_DIR"

  # Platform operator, the $SYS organization, and the operator's own
  # organization. --operator-org is passed explicitly because leaving it empty
  # makes the command prompt, and there is no terminal here.
  stone-age bootstrap --dir "$PB_DIR" \
    --email "$EMAIL" \
    --org "$ORG" \
    --operator-org "$OPERATOR_ORG"

  # operator.jwt, operator.conf, nats.conf, the resolver directory and the
  # JetStream store, all resolved inside NATS_DIR so one volume holds them.
  stone-age nats export --dir "$PB_DIR" --output "$NATS_DIR"

  echo "==> Seeded. Log in as $EMAIL"
fi

# exec, so the binary is PID 1 and gets SIGTERM directly: PocketBase owns
# signal handling and stops the embedded NATS server on the way down.
exec stone-age serve \
  --http "0.0.0.0:${STONE_AGE_HTTP_PORT:-8090}" \
  --dir "$PB_DIR" \
  --nats \
  --nats-config "$NATS_DIR/nats.conf" \
  "$@"
