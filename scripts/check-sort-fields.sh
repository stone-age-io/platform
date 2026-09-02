#!/usr/bin/env bash
#
# Regression guard for the `sortable:` fields on list-view columns.
#
# A sort term is a string in a .vue file. Nothing type-checks it against the
# collection it is aimed at, and getting it wrong does not look like a sorting
# bug: PocketBase rejects an unknown sort field inside `searchProvider.ParseAndExec`
# and returns a bare 400, which reaches the browser as only "Something went wrong
# while processing your request." -- no field name, and it fails for superusers
# too. That exact failure has already taken two screens down in this console
# (Members and Invitations, sorted by a `created` that neither collection had).
#
# So every `sortable:` value in ui/src/views is asked of a real server here.
#
#   ./scripts/check-sort-fields.sh
#   PORT=18124 ./scripts/check-sort-fields.sh   # if the default port is busy
#   KEEP=1 ./scripts/check-sort-fields.sh       # keep the temp dir + server log
#
# Requires: go, curl, node (node parses the .vue files).
#
# The probes are UNAUTHENTICATED on purpose, and that is not a gap. PocketBase
# validates the sort expression after it builds the list rule but before it
# applies one, so a guest gets the same 400 for a bad field and the same 200
# (with zero rows) for a good one that a member would. No fixtures, no login, and
# the check still fails for the only reason it is looking for.

set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

PORT="${PORT:-18098}"
API="http://127.0.0.1:$PORT/api"

EXT=""
case "$(uname -s)" in MINGW* | MSYS* | CYGWIN*) EXT=".exe" ;; esac
BIN="./stone-age-sortcheck$EXT"

WORK="$(mktemp -d)"
SRV_PID=""
PASS=0
FAIL=0

cleanup() {
  if [ -n "$SRV_PID" ]; then
    kill "$SRV_PID" 2>/dev/null
    # Wait for it to actually exit: on Windows the SQLite files stay locked until
    # the process is gone, and rm -rf would race it and leave the temp dir behind.
    wait "$SRV_PID" 2>/dev/null
  fi
  if [ -n "${KEEP:-}" ]; then
    echo "kept: $WORK"
  else
    rm -rf "$WORK" "$BIN"
  fi
}
trap cleanup EXIT

die() { echo "ERROR: $*" >&2; exit 1; }

for tool in go curl node; do
  command -v "$tool" >/dev/null 2>&1 || die "$tool is required but not on PATH"
done

if curl -s -o /dev/null -m 2 "$API/health"; then
  die "something is already listening on port $PORT. Re-run with PORT=<free port>."
fi

echo "=== building ==="
go build -o "$BIN" . || die "build failed"

echo "=== initialising a throwaway database in $WORK ==="
"$BIN" migrate up --dir "$WORK/pb_data" >"$WORK/migrate.log" 2>&1 || {
  cat "$WORK/migrate.log"; die "migrate up failed"
}

"$BIN" serve --http "127.0.0.1:$PORT" --dir "$WORK/pb_data" >"$WORK/server.log" 2>&1 &
SRV_PID=$!

for _ in $(seq 1 40); do
  curl -s -o /dev/null -m 1 "$API/health" && break
  sleep 0.25
done
curl -s -o /dev/null -m 1 "$API/health" || {
  echo "--- server log ---"; cat "$WORK/server.log"; die "server never became ready"
}

# ---------------------------------------------------------------- the terms
#
# A view names its collection once, in its usePagination call, and its sort
# terms in the `sortable:` entries of its column list. Pair them up.

node > "$WORK/terms.txt" <<'ENDNODE'
const fs = require('fs'), path = require('path')
const root = 'ui/src/views'
const walk = d => fs.readdirSync(d, { withFileTypes: true }).flatMap(e =>
  e.isDirectory() ? walk(path.join(d, e.name)) : [path.join(d, e.name)])

for (const file of walk(root).filter(f => f.endsWith('.vue'))) {
  const src = fs.readFileSync(file, 'utf8')
  const terms = [...src.matchAll(/sortable: '([^']+)'/g)].map(m => m[1].replace(/^-/, ''))
  if (!terms.length) continue
  const coll = src.match(/usePagination<[^>]*>\('([a-z_]+)'/)
  if (!coll) {
    console.log(`?\t?\t${file}\tno usePagination call to name a collection`)
    continue
  }
  for (const t of new Set(terms)) console.log(`${coll[1]}\t${t}\t${file}`)
}
ENDNODE

[ -s "$WORK/terms.txt" ] || die "found no sortable columns to check -- did the extraction break?"

echo "=== checking $(wc -l < "$WORK/terms.txt" | tr -d ' ') sort terms ==="

while IFS=$'\t' read -r coll field file note; do
  if [ "$coll" = "?" ]; then
    echo "  FAIL  $file: $note"
    FAIL=$((FAIL + 1))
    continue
  fi
  code=$(curl -s -o /dev/null -w "%{http_code}" \
    "$API/collections/$coll/records?perPage=1&sort=$field")
  if [ "$code" = "200" ]; then
    PASS=$((PASS + 1))
  else
    echo "  FAIL  $coll.$field ($code) -- $file"
    FAIL=$((FAIL + 1))
  fi
done < "$WORK/terms.txt"

echo
if [ "$FAIL" -eq 0 ]; then
  echo "OK: $PASS sort terms all resolve"
  exit 0
fi
echo "$FAIL of $((PASS + FAIL)) sort terms are not fields of their collection"
exit 1
