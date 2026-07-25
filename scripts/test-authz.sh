#!/usr/bin/env bash
#
# Regression guard for the PocketBase API rules in schema.json.
#
# API rules are the ONLY thing enforcing tenant isolation and privilege
# boundaries in this platform -- pb-nats and pb-nebula contain no tenancy logic
# at all. They are also plain strings in a JSON file with no compiler and no
# type checker, so a typo silently opens a hole. This script exercises the ones
# that matter against a real server.
#
# It builds the binary, brings up a throwaway database, runs the checks, and
# tears everything down. Nothing touches pb_data/.
#
#   ./scripts/test-authz.sh
#   PORT=18123 ./scripts/test-authz.sh     # if the default port is busy
#   KEEP=1 ./scripts/test-authz.sh         # keep the temp dir + server log
#
# Requires: go, curl, node (node is only used as a JSON parser).
#
# Reading the output: PocketBase answers 404 -- not 403 -- when an update rule
# rejects a request, deliberately, so it does not leak whether the record
# exists. A denial is therefore "403|400|404". Because a blanket deny would
# also produce those codes, every deny check below is paired with an allow
# check on the SAME record, proving the rule discriminates on body content
# rather than just refusing everything.

set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

PORT="${PORT:-18099}"
API="http://127.0.0.1:$PORT/api"
EXPECTED_CHECKS=53          # bump when you add a check; guards against silent early exits
SU_EMAIL="su@authz.test"
SU_PASS="SuperSecret123!"

EXT=""
case "$(uname -s)" in MINGW* | MSYS* | CYGWIN*) EXT=".exe" ;; esac
BIN_BASE="stone-age-authz-test"
BIN="./$BIN_BASE$EXT"

WORK="$(mktemp -d)"
SRV_PID=""
PASS=0
FAIL=0

cleanup() {
  if [ -n "$SRV_PID" ]; then
    kill "$SRV_PID" 2>/dev/null
    # Git Bash cannot reliably signal a native Windows child process, so fall
    # back to killing it by name. The binary name is unique to this script.
    if [ -n "$EXT" ] && command -v powershell.exe >/dev/null 2>&1; then
      powershell.exe -NoProfile -Command \
        "Get-Process -Name '$BIN_BASE' -ErrorAction SilentlyContinue | Stop-Process -Force" \
        >/dev/null 2>&1
    fi
    wait "$SRV_PID" 2>/dev/null
  fi
  if [ "${KEEP:-}" = "1" ]; then
    echo "kept: $WORK (server log: $WORK/server.log)"
  else
    rm -rf "$WORK" "$BIN"
  fi
}
trap cleanup EXIT

die() { echo "FATAL: $*" >&2; exit 1; }

# ---------------------------------------------------------------- test harness

# j <json> <key> -> top-level value ('' if absent)
j() {
  node -e "let s='';process.stdin.on('data',d=>s+=d).on('end',()=>{try{const v=JSON.parse(s)['$2'];console.log(v===undefined||v===null?'':v)}catch(e){console.log('')}})" <<<"$1"
}

# jn <json> <js-expr> -> expression evaluated against the parsed body as `o`
# e.g. jn "$RBODY" 'o.items[0].id'
jn() {
  node -e "let s='';process.stdin.on('data',d=>s+=d).on('end',()=>{try{const o=JSON.parse(s);const v=($2);console.log(v===undefined||v===null?'':v)}catch(e){console.log('')}})" <<<"$1"
}

# expect <label> <expected-http, |-separated> <actual-http> <body>
expect() {
  if [[ "$3" =~ ^($2)$ ]]; then
    echo "  PASS  $1 (HTTP $3)"
    PASS=$((PASS + 1))
  else
    echo "  FAIL  $1 -- expected HTTP $2, got $3"
    echo "        $(head -c 300 <<<"$4")"
    FAIL=$((FAIL + 1))
  fi
}

# ok <label> / no <label> -- for assertions that are not about a status code
ok() { echo "  PASS  $1"; PASS=$((PASS + 1)); }
no() { echo "  FAIL  $1"; FAIL=$((FAIL + 1)); }

# req <method> <path> [token] [json-body] -> sets RCODE, RBODY
req() {
  local args=(-s -w '\n%{http_code}' -X "$1" "$API$2" -H 'Content-Type: application/json')
  [ -n "${3:-}" ] && args+=(-H "Authorization: ${3}")
  [ -n "${4:-}" ] && args+=(-d "${4}")
  local out
  out=$(curl "${args[@]}")
  RCODE=$(tail -n1 <<<"$out")
  RBODY=$(sed '$d' <<<"$out")
}

# ------------------------------------------------------------------- bring up

for tool in go curl node; do
  command -v "$tool" >/dev/null 2>&1 || die "$tool is required but not on PATH"
done

if curl -s -o /dev/null -m 2 "$API/health"; then
  die "something is already listening on port $PORT. Re-run with PORT=<free port>."
fi

echo "=== building ==="
go build -o "$BIN" . || die "build failed"

echo "=== initialising a throwaway database in $WORK ==="
# Same order as a real deployment: superuser, then schema, then serve.
"$BIN" superuser upsert "$SU_EMAIL" "$SU_PASS" --dir "$WORK/pb_data" >/dev/null 2>&1 \
  || die "superuser upsert failed"
"$BIN" migrate up --dir "$WORK/pb_data" >/dev/null 2>&1 || die "migrate up failed"

"$BIN" serve --http "127.0.0.1:$PORT" --dir "$WORK/pb_data" >"$WORK/server.log" 2>&1 &
SRV_PID=$!

for _ in $(seq 1 40); do
  curl -s -o /dev/null -m 1 "$API/health" && break
  sleep 0.25
done
curl -s -o /dev/null -m 1 "$API/health" || {
  echo "--- server log ---"; cat "$WORK/server.log"; die "server never became ready"
}

# --------------------------------------------------------------------- fixtures

echo ""
echo "=== setup (as superuser; API rules are bypassed) ==="
req POST /collections/_superusers/auth-with-password "" \
  "{\"identity\":\"$SU_EMAIL\",\"password\":\"$SU_PASS\"}"
SU=$(j "$RBODY" token)
[ -z "$SU" ] && die "superuser auth failed: $RBODY"

mkuser() { # <email> -> id
  req POST /collections/users/records "$SU" \
    "{\"email\":\"$1\",\"password\":\"Password123!\",\"passwordConfirm\":\"Password123!\",\"name\":\"$1\",\"emailVisibility\":true,\"verified\":true}"
  j "$RBODY" id
}
ALICE=$(mkuser alice@test.local)
BOB=$(mkuser bob@test.local)
BADGE=$(mkuser badge@test.local)
EVE=$(mkuser eve@test.local)
[ -z "$ALICE" ] && die "user create failed: $RBODY"
echo "  users: alice=$ALICE bob=$BOB badge=$BADGE eve=$EVE"

req POST /collections/organizations/records "$SU" \
  "{\"name\":\"TestOrg\",\"owner\":\"$ALICE\",\"active\":true}"
ORG=$(j "$RBODY" id)
req POST /collections/organizations/records "$SU" \
  "{\"name\":\"OtherOrg\",\"owner\":\"$EVE\",\"active\":true}"
ORG2=$(j "$RBODY" id)
[ -z "$ORG" ] || [ -z "$ORG2" ] && die "org create failed: $RBODY"
echo "  orgs: TestOrg=$ORG OtherOrg=$ORG2"

# alice's owner membership is created for her by pb-tenancy; add the others.
req POST /collections/memberships/records "$SU" \
  "{\"user\":\"$BOB\",\"organization\":\"$ORG\",\"role\":\"member\"}"
MBOB=$(j "$RBODY" id)
req POST /collections/memberships/records "$SU" \
  "{\"user\":\"$BADGE\",\"organization\":\"$ORG\",\"role\":\"badge\"}"
MBADGE=$(j "$RBODY" id)
[ -z "$MBOB" ] && die "membership create failed: $RBODY"
for u in "$ALICE" "$BOB" "$BADGE"; do
  req PATCH "/collections/users/records/$u" "$SU" "{\"current_organization\":\"$ORG\"}"
done
echo "  memberships: bob=$MBOB badge=$MBADGE"

# NATS identities. Creating the org auto-provisions its account (and a default
# role); fall back to creating a role if this deployment does not.
req GET "/collections/nats_accounts/records?filter=(organization='$ORG')" "$SU"
ACCT=$(jn "$RBODY" 'o.items[0].id')
[ -z "$ACCT" ] && die "org did not get a NATS account: $RBODY"
req GET "/collections/nats_roles/records?filter=(organization='$ORG')" "$SU"
NROLE=$(jn "$RBODY" 'o.items[0].id')
if [ -z "$NROLE" ]; then
  req POST /collections/nats_roles/records "$SU" \
    "{\"name\":\"test-role\",\"organization\":\"$ORG\",\"publish_permissions\":[\"test.>\"],\"subscribe_permissions\":[\"test.>\"]}"
  NROLE=$(j "$RBODY" id)
fi
[ -z "$NROLE" ] && die "could not obtain a NATS role: $RBODY"

mknats() { # <nats_username> -> id
  req POST /collections/nats_users/records "$SU" \
    "{\"email\":\"$1@nats.test\",\"password\":\"Password123!\",\"passwordConfirm\":\"Password123!\",\"nats_username\":\"$1\",\"account_id\":\"$ACCT\",\"role_id\":\"$NROLE\",\"organization\":\"$ORG\",\"active\":true}"
  j "$RBODY" id
}
# BOB_NATS is linked to bob's membership -- this is the identity his browser
# connects with, and the one row he is allowed to see. DEV_NATS belongs to a
# device and must stay invisible to him.
BOB_NATS=$(mknats bob-ui)
DEV_NATS=$(mknats device-1)
[ -z "$BOB_NATS" ] || [ -z "$DEV_NATS" ] && die "nats_user create failed: $RBODY"
req PATCH "/collections/memberships/records/$MBOB" "$SU" "{\"nats_user\":\"$BOB_NATS\"}"
echo "  nats identities: bob-ui=$BOB_NATS device-1=$DEV_NATS (account=$ACCT role=$NROLE)"

# A Thing, for the identity-link checks.
req POST /collections/things/records "$SU" \
  "{\"email\":\"thing1@test.local\",\"password\":\"Password123!\",\"passwordConfirm\":\"Password123!\",\"emailVisibility\":true,\"name\":\"Thing One\",\"code\":\"TH1\",\"organization\":\"$ORG\"}"
THING=$(j "$RBODY" id)
[ -z "$THING" ] && die "thing create failed: $RBODY"
echo "  thing: $THING"

login() {
  req POST /collections/users/auth-with-password "" \
    "{\"identity\":\"$1\",\"password\":\"Password123!\"}"
  j "$RBODY" token
}
TA=$(login alice@test.local)   # owner
TB=$(login bob@test.local)     # member
TG=$(login badge@test.local)   # badge
[ -z "$TA" ] || [ -z "$TB" ] || [ -z "$TG" ] && die "user login failed: $RBODY"

# ----------------------------------------------------------------- the checks

echo ""
echo "=== 1. is_operator freeze (THE critical hole) ==="
# A platform operator reads every tenant's data, including audit_logs, which
# used to carry the NATS operator seed. users.updateRule guarded only
# current_organization, so this single PATCH was unauthenticated-to-root.
req PATCH "/collections/users/records/$BOB" "$TB" '{"is_operator":true}'
expect "member cannot self-grant is_operator" "403|400|404" "$RCODE" "$RBODY"
req GET "/collections/users/records/$BOB" "$TB"
if [ "$(j "$RBODY" is_operator)" = "true" ]; then
  no "is_operator actually got set!"
else
  ok "is_operator still false after the attempt"
fi
req PATCH "/collections/users/records/$BOB" "$TB" '{"name":"Bob Renamed"}'
expect "normal profile update still works" 200 "$RCODE" "$RBODY"

echo ""
echo "=== 2. membership self-escalation and tenant hopping ==="
req PATCH "/collections/memberships/records/$MBOB" "$TB" '{"role":"owner"}'
expect "member cannot promote self to owner" "403|400|404" "$RCODE" "$RBODY"
req PATCH "/collections/memberships/records/$MBOB" "$TB" '{"role":"admin"}'
expect "member cannot promote self to admin" "403|400|404" "$RCODE" "$RBODY"
# Re-links the SAME identity rather than clearing it: this exercises the same
# rule branch, but sections 7 and 9 need the link to survive.
req PATCH "/collections/memberships/records/$MBOB" "$TB" "{\"nats_user\":\"$BOB_NATS\"}"
expect "self nats_user link still works (Settings page)" 200 "$RCODE" "$RBODY"
req PATCH "/collections/memberships/records/$MBOB" "$TB" "{\"organization\":\"$ORG2\"}"
expect "cannot move own membership into another tenant" "403|400|404" "$RCODE" "$RBODY"
req PATCH "/collections/memberships/records/$MBOB" "$TA" '{"role":"admin"}'
expect "owner CAN change a member's role (admin UX intact)" 200 "$RCODE" "$RBODY"
req PATCH "/collections/memberships/records/$MBOB" "$SU" '{"role":"member"}' # reset

echo ""
echo "=== 3. 'badge' no longer passes admin checks ==="
# The old rules said role ?!= "member", which any role other than member
# satisfied -- including badge, a deliberately low-trust role.
req POST /collections/thing_types/records "$TG" \
  "{\"name\":\"BadgeType\",\"code\":\"BT1\",\"organization\":\"$ORG\"}"
expect "badge cannot create thing_types" "403|400|404" "$RCODE" "$RBODY"
req POST /collections/invites/records "$TG" \
  "{\"email\":\"x@test.local\",\"organization\":\"$ORG\",\"role\":\"member\"}"
expect "badge cannot invite users" "403|400|404" "$RCODE" "$RBODY"
req POST /collections/message_schemas/records "$TG" \
  "{\"namespace\":\"n\",\"name\":\"s\",\"version\":\"1.0.0\",\"organization\":\"$ORG\",\"schema\":{}}"
expect "badge cannot create message_schemas" "403|400|404" "$RCODE" "$RBODY"
req POST /collections/thing_types/records "$TA" \
  "{\"name\":\"OwnerType\",\"code\":\"OT1\",\"organization\":\"$ORG\"}"
expect "owner CAN still create thing_types" 200 "$RCODE" "$RBODY"

echo ""
echo "=== 4. anonymous signup requires a pending invite ==="
req POST /collections/users/records "" \
  '{"email":"attacker@evil.test","password":"Password123!","passwordConfirm":"Password123!"}'
expect "uninvited anonymous signup blocked" "403|400|404" "$RCODE" "$RBODY"
req POST /collections/invites/records "$TA" \
  "{\"email\":\"carol@test.local\",\"organization\":\"$ORG\",\"role\":\"member\"}"
expect "owner can create an invite" 200 "$RCODE" "$RBODY"
req POST /collections/users/records "" \
  '{"email":"carol@test.local","password":"Password123!","passwordConfirm":"Password123!","name":"Carol","emailVisibility":true}'
expect "INVITED anonymous signup still works" 200 "$RCODE" "$RBODY"

echo ""
echo "=== 5. cross-tenant org switch ==="
req PATCH "/collections/users/records/$BOB" "$TB" "{\"current_organization\":\"$ORG2\"}"
expect "cannot switch into an org you do not belong to" "403|400|404" "$RCODE" "$RBODY"

echo ""
echo "=== 6. operator seed no longer reaches the API or audit_logs ==="
# The four secret fields are hidden:true. Superusers are intentionally exempt
# from field hiding in direct API responses, so the checks below assert the
# superuser CAN see them -- and that the seed still never lands in audit_logs,
# which is the table ordinary operators can read.
req POST /collections/nats_system_operator/records "$SU" \
  '{"name":"TESTOP","public_key":"OPUB","private_key":"PRIV-LEAK-CANARY","seed":"SOSEEDLEAKCANARY","signing_public_key":"SPUB","signing_private_key":"SPRIV-LEAK-CANARY","signing_seed":"SSEEDLEAKCANARY"}'
OPID=$(j "$RBODY" id)
if grep -q "LEAK-CANARY\|LEAKCANARY" <<<"$RBODY"; then
  ok "superuser sees the seed on create (by design; admin panel needs it)"
else
  no "create response unexpectedly omitted the seed for a superuser"
fi
req GET "/collections/nats_system_operator/records/$OPID" "$SU"
if grep -q "LEAK-CANARY\|LEAKCANARY" <<<"$RBODY"; then
  ok "superuser GET sees the seed (by design); non-superusers have no read rule at all"
else
  no "superuser GET unexpectedly omitted the seed"
fi
sleep 1 # audit writes are asynchronous
req GET "/collections/audit_logs/records?perPage=200&filter=collection_name='nats_system_operator'" "$SU"
if [ "$(j "$RBODY" totalItems)" = "0" ]; then
  ok "EventFilter kept nats_system_operator out of audit_logs (0 rows)"
else
  no "$(j "$RBODY" totalItems) audit row(s) exist for nats_system_operator"
fi
req GET "/collections/audit_logs/records?perPage=500" "$SU"
if grep -q "LEAK-CANARY\|LEAKCANARY" <<<"$RBODY"; then
  no "canary seed found somewhere in audit_logs"
else
  ok "no canary seed anywhere in audit_logs"
fi

echo ""
echo "=== 7. credential broadcast: scoped by ROW, not by hidden fields ==="
# nats_users.listRule used to be "any member of the org sees every identity in
# it", and creds_file embeds the NATS seed -- so any member could download every
# device credential. The fix narrows which ROWS are visible rather than hiding
# the field, because the field has to stay readable to the identity that owns it
# (the browser's own NATS connection, and leaf-sync's bootstrap).
req GET "/collections/nats_users/records" "$TB"
expect "member can query nats_users (rule parses; back-relation resolves)" 200 "$RCODE" "$RBODY"
if [ "$(j "$RBODY" totalItems)" = "1" ]; then
  ok "member sees exactly ONE nats identity (not the whole org)"
else
  no "member sees $(j "$RBODY" totalItems) nats identities, expected 1"
fi
if [ "$(jn "$RBODY" 'o.items[0].id')" = "$BOB_NATS" ]; then
  ok "the one row a member sees is their own linked identity"
else
  no "member's single visible row is $(jn "$RBODY" 'o.items[0].id'), expected $BOB_NATS"
fi
# Load bearing: without this the browser cannot open its NATS connection at all.
req GET "/collections/nats_users/records/$BOB_NATS" "$TB"
CREDS_BEFORE=$(j "$RBODY" creds_file)
if [ -n "$CREDS_BEFORE" ]; then
  ok "member CAN still read their own creds_file (browser NATS connection intact)"
else
  no "member cannot read their own creds_file -- the UI NATS connection is broken"
fi
req GET "/collections/nats_users/records/$DEV_NATS" "$TB"
expect "member cannot read another identity's credential" "403|400|404" "$RCODE" "$RBODY"
req GET "/collections/nats_users/records" "$TA"
if [ "$(j "$RBODY" totalItems)" -ge 2 ] 2>/dev/null; then
  ok "owner still sees every identity in the org (admin UX intact)"
else
  no "owner sees $(j "$RBODY" totalItems) identities, expected >= 2"
fi

echo ""
echo "=== 8. infrastructure writes are owner/admin only ==="
# The escalation this closes: publish_permissions on a nats_users record is
# copied verbatim into the JWT pb-nats signs, so a member who could write this
# collection could self-grant publish ">" and be handed a signed JWT for it.
# Each deny is paired with the SAME request as owner, so a 400 from validation
# cannot be mistaken for a 400 from the rule.
req PATCH "/collections/nats_users/records/$BOB_NATS" "$TB" '{"publish_permissions":[">"]}'
expect "member cannot grant themselves publish '>' on their own identity" "403|400|404" "$RCODE" "$RBODY"
req PATCH "/collections/nats_users/records/$BOB_NATS" "$TA" '{"publish_permissions":[">"]}'
expect "owner CAN set publish permissions (same request, so the deny was authz)" 200 "$RCODE" "$RBODY"

natsuser_payload() { # <username>
  echo "{\"email\":\"$1@nats.test\",\"password\":\"Password123!\",\"passwordConfirm\":\"Password123!\",\"nats_username\":\"$1\",\"account_id\":\"$ACCT\",\"role_id\":\"$NROLE\",\"organization\":\"$ORG\",\"active\":true}"
}
req POST /collections/nats_users/records "$TB" "$(natsuser_payload member-minted)"
expect "member cannot mint a NATS identity" "403|400|404" "$RCODE" "$RBODY"
req POST /collections/nats_users/records "$TA" "$(natsuser_payload owner-minted)"
expect "owner CAN mint a NATS identity (same payload)" 200 "$RCODE" "$RBODY"
req POST /collections/nats_users/records "$TG" "$(natsuser_payload badge-minted)"
expect "badge cannot mint a NATS identity" "403|400|404" "$RCODE" "$RBODY"

role_payload() { echo "{\"name\":\"$1\",\"organization\":\"$ORG\",\"publish_permissions\":[\">\"]}"; }
req POST /collections/nats_roles/records "$TB" "$(role_payload member-role)"
expect "member cannot create a NATS role (2nd path to the same escalation)" "403|400|404" "$RCODE" "$RBODY"
req POST /collections/nats_roles/records "$TA" "$(role_payload owner-role)"
expect "owner CAN create a NATS role (same payload)" 200 "$RCODE" "$RBODY"

req GET "/collections/nebula_ca/records?filter=(organization='$ORG')" "$SU"
CA=$(jn "$RBODY" 'o.items[0].id')
[ -z "$CA" ] && die "org did not get a Nebula CA: $RBODY"
net_payload() { echo "{\"name\":\"$1\",\"cidr_range\":\"10.42.0.0/16\",\"ca_id\":\"$CA\",\"organization\":\"$ORG\"}"; }
req POST /collections/nebula_networks/records "$TB" "$(net_payload member-net)"
expect "member cannot create a Nebula network" "403|400|404" "$RCODE" "$RBODY"
req POST /collections/nebula_networks/records "$TA" "$(net_payload owner-net)"
expect "owner CAN create a Nebula network (same payload)" 200 "$RCODE" "$RBODY"
NET=$(j "$RBODY" id)

host_payload() { # <hostname> <ip>
  echo "{\"email\":\"$1@hosts.test\",\"password\":\"Password123!\",\"passwordConfirm\":\"Password123!\",\"emailVisibility\":true,\"hostname\":\"$1\",\"overlay_ip\":\"$2\",\"network_id\":\"$NET\",\"organization\":\"$ORG\"}"
}
req POST /collections/nebula_hosts/records "$TB" "$(host_payload member-host 10.42.0.11)"
expect "member cannot mint a Nebula host identity" "403|400|404" "$RCODE" "$RBODY"
req POST /collections/nebula_hosts/records "$TA" "$(host_payload owner-host 10.42.0.12)"
expect "owner CAN mint a Nebula host (same payload)" 200 "$RCODE" "$RBODY"
HOST=$(j "$RBODY" id)
req GET "/collections/nebula_hosts/records/$HOST" "$TB"
expect "member cannot read a host's config_yaml (it embeds the private key)" "403|400|404" "$RCODE" "$RBODY"

echo ""
echo "=== 9. Thing identity links, and self-service rotation ==="
# Members keep inventory CRUD, but not the identity relations: a Thing can read
# the credential of its own linked identity, so a member able to re-point
# things.nats_user at a privileged identity and then authenticate as the Thing
# would have a credential-theft path.
req PATCH "/collections/things/records/$THING" "$TB" '{"name":"Renamed By Member"}'
expect "member CAN still edit thing inventory fields" 200 "$RCODE" "$RBODY"
req PATCH "/collections/things/records/$THING" "$TB" "{\"nats_user\":\"$DEV_NATS\"}"
expect "member cannot attach an identity to a thing" "403|400|404" "$RCODE" "$RBODY"
req PATCH "/collections/things/records/$THING" "$TA" "{\"nats_user\":\"$DEV_NATS\"}"
expect "owner CAN attach an identity to a thing (same field)" 200 "$RCODE" "$RBODY"

# The Thing detail page renders its credential buttons behind
# `v-if="thing.expand?.nats_user"`. PocketBase drops an expand the caller may not
# view rather than failing the request, so the page degrades on its own for a
# member -- no UI change needed. That is worth asserting rather than assuming.
req GET "/collections/things/records/$THING?expand=nats_user" "$TB"
if [ "$RCODE" = "200" ] && [ -z "$(jn "$RBODY" 'o.expand && o.expand.nats_user && o.expand.nats_user.id')" ]; then
  ok "member loads the thing but gets no nats_user expand (credential UI self-hides)"
else
  no "member expand=nats_user returned HTTP $RCODE with expand $(jn "$RBODY" 'o.expand && o.expand.nats_user && o.expand.nats_user.id')"
fi
req GET "/collections/things/records/$THING?expand=nats_user" "$TA"
if [ "$(jn "$RBODY" 'o.expand && o.expand.nats_user && o.expand.nats_user.id')" = "$DEV_NATS" ]; then
  ok "owner DOES get the nats_user expand (same request, so the member's was authz)"
else
  no "owner expand=nats_user missing; expected $DEV_NATS"
fi

# Rotation is a route, not a rule: it must permit a write to exactly one field
# (`regenerate`), which a rule can only approximate with an :isset deny-list.
req POST "/me/nats-creds/rotate" "$TB" ""
expect "member CAN rotate their own credentials" 200 "$RCODE" "$RBODY"
sleep 1
req GET "/collections/nats_users/records/$BOB_NATS" "$TB"
CREDS_AFTER=$(j "$RBODY" creds_file)
if [ -n "$CREDS_AFTER" ] && [ "$CREDS_AFTER" != "$CREDS_BEFORE" ]; then
  ok "rotation actually re-minted the credential"
else
  no "creds_file did not change after rotation"
fi

echo ""
echo "=== 10. deletion and org profile are management actions ==="
# things was the only collection left with a member-level delete; every other
# one (locations, thing_types, leaf_nodes, ...) already required owner/admin.
req POST /collections/things/records "$SU" \
  "{\"email\":\"thing2@test.local\",\"password\":\"Password123!\",\"passwordConfirm\":\"Password123!\",\"emailVisibility\":true,\"name\":\"Thing Two\",\"code\":\"TH2\",\"organization\":\"$ORG\"}"
THING2=$(j "$RBODY" id)
[ -z "$THING2" ] && die "second thing create failed: $RBODY"
req DELETE "/collections/things/records/$THING2" "$TB"
expect "member cannot delete a thing" "403|400|404" "$RCODE" "$RBODY"
req DELETE "/collections/things/records/$THING2" "$TA"
expect "owner CAN delete a thing (same record, so the deny was authz)" "200|204" "$RCODE" "$RBODY"

# An organization record carries the tenancy flags and drives NATS/Nebula
# provisioning, so its profile is operator-only -- an org owner has no update path.
req PATCH "/collections/organizations/records/$ORG" "$TA" '{"name":"Renamed By Owner"}'
expect "org owner cannot update their own organization" "403|400|404" "$RCODE" "$RBODY"
req POST /collections/users/records "$SU" \
  '{"email":"oper@test.local","password":"Password123!","passwordConfirm":"Password123!","name":"Operator","emailVisibility":true,"verified":true,"is_operator":true}'
OPER=$(j "$RBODY" id)
[ -z "$OPER" ] && die "operator user create failed: $RBODY"
TO=$(login oper@test.local)
req PATCH "/collections/organizations/records/$ORG" "$TO" '{"name":"Renamed By Operator"}'
expect "platform operator CAN update it (same field, so the deny was authz)" 200 "$RCODE" "$RBODY"

echo ""
echo "=== 11. regression: ordinary tenant reads still work ==="
req GET "/collections/organizations/records" "$TB"
expect "member can list their orgs" 200 "$RCODE" "$RBODY"
req GET "/collections/memberships/records" "$TB"
expect "member can list own memberships (org switcher)" 200 "$RCODE" "$RBODY"
req GET "/collections/users/records" "$TA"
expect "owner can list org users" 200 "$RCODE" "$RBODY"
req GET "/collections/things/records" "$TB"
expect "member can list things" 200 "$RCODE" "$RBODY"

# ----------------------------------------------------------------------- result

TOTAL=$((PASS + FAIL))
echo ""
if [ "$TOTAL" -ne "$EXPECTED_CHECKS" ]; then
  echo "  FAIL  ran $TOTAL checks, expected $EXPECTED_CHECKS -- a check exited early"
  FAIL=$((FAIL + 1))
fi
echo "==================== $PASS passed, $FAIL failed ===================="
[ "$FAIL" -gt 0 ] && exit 1
exit 0
