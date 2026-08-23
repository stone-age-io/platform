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
EXPECTED_CHECKS=150         # bump when you add a check; guards against silent early exits
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
DASH=$(mkuser dashboard@test.local)
VIEW=$(mkuser viewer@test.local)
EVE=$(mkuser eve@test.local)
[ -z "$ALICE" ] && die "user create failed: $RBODY"
echo "  users: alice=$ALICE bob=$BOB dashboard=$DASH viewer=$VIEW eve=$EVE"

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
  "{\"user\":\"$DASH\",\"organization\":\"$ORG\",\"role\":\"dashboard\"}"
MDASH=$(j "$RBODY" id)
req POST /collections/memberships/records "$SU" \
  "{\"user\":\"$VIEW\",\"organization\":\"$ORG\",\"role\":\"viewer\"}"
MVIEW=$(j "$RBODY" id)
[ -z "$MBOB" ] && die "membership create failed: $RBODY"
[ -z "$MVIEW" ] && die "viewer membership create failed (is 'viewer' in the role select?): $RBODY"
for u in "$ALICE" "$BOB" "$DASH" "$VIEW"; do
  req PATCH "/collections/users/records/$u" "$SU" "{\"current_organization\":\"$ORG\"}"
done
echo "  memberships: bob=$MBOB dashboard=$MDASH viewer=$MVIEW"

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
TG=$(login dashboard@test.local)   # dashboard
TV=$(login viewer@test.local)  # viewer -- read-only staff
[ -z "$TA" ] || [ -z "$TB" ] || [ -z "$TG" ] || [ -z "$TV" ] && die "user login failed: $RBODY"

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
echo "=== 3. 'dashboard' no longer passes admin checks ==="
# The old rules said role ?!= "member", which any role other than member
# satisfied -- including dashboard, a deliberately low-trust role.
req POST /collections/thing_types/records "$TG" \
  "{\"name\":\"DashType\",\"code\":\"BT1\",\"organization\":\"$ORG\"}"
expect "dashboard cannot create thing_types" "403|400|404" "$RCODE" "$RBODY"
req POST /collections/invites/records "$TG" \
  "{\"email\":\"x@test.local\",\"organization\":\"$ORG\",\"role\":\"member\"}"
expect "dashboard cannot invite users" "403|400|404" "$RCODE" "$RBODY"
req POST /collections/message_schemas/records "$TG" \
  "{\"namespace\":\"n\",\"name\":\"s\",\"version\":\"1.0.0\",\"organization\":\"$ORG\",\"schema\":{}}"
expect "dashboard cannot create message_schemas" "403|400|404" "$RCODE" "$RBODY"
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
req POST /collections/nats_users/records "$TG" "$(natsuser_payload dashboard-minted)"
expect "dashboard cannot mint a NATS identity" "403|400|404" "$RCODE" "$RBODY"

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

# Quick metadata edit from the detail page (MetadataCard) sends ONLY `metadata`.
# That is load-bearing rather than economical: the member branch of
# things.updateRule requires `nats_user:changed = false`, and a field absent from
# the body counts as unchanged. This Thing now HAS a linked identity (attached
# just above), which is exactly the live case -- so assert the partial update
# still passes with the relation populated, not merely on a bare record.
req PATCH "/collections/things/records/$THING" "$TB" '{"metadata":{"last_service":"2026-03-14"}}'
expect "member CAN patch only metadata on a thing with a linked identity" 200 "$RCODE" "$RBODY"
req PATCH "/collections/things/records/$THING" "$TG" '{"metadata":{"last_service":"2026-03-15"}}'
expect "dashboard cannot patch a thing's metadata (same payload)" "403|400|404" "$RCODE" "$RBODY"

# Rotation is a route, not a rule: it must permit a write to exactly one field
# (`regenerate`), which a rule can only approximate with an :isset deny-list.
#
# Re-read creds IMMEDIATELY before rotating. An earlier check in section 8 PATCHes
# publish_permissions on this same record, and pb-nats re-mints the JWT on any
# API update whose JWT-relevant fields changed -- so comparing against a value
# captured earlier would pass whether or not the route did anything.
req GET "/collections/nats_users/records/$BOB_NATS" "$TB"
ROT_BEFORE=$(j "$RBODY" creds_file)
req POST "/me/nats-creds/rotate" "$TB" ""
expect "member CAN rotate their own credentials" 200 "$RCODE" "$RBODY"
sleep 1
req GET "/collections/nats_users/records/$BOB_NATS" "$TB"
ROT_AFTER=$(j "$RBODY" creds_file)
if [ -n "$ROT_AFTER" ] && [ "$ROT_AFTER" != "$ROT_BEFORE" ]; then
  ok "rotation actually re-minted the credential"
else
  no "creds_file unchanged after rotation -- the route set the flag but nothing acted on it"
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
echo "=== 11. dashboard is excluded from inventory writes ==="
# things create/update admitted a member through a branch that restricted the
# FIELDS (nats_user/nebula_host unchanged) without naming a ROLE, and locations
# had no role check at all -- so dashboard, the most restricted role, satisfied both.
# Same failure as the `role ?!= "member"` deny-list: say which roles may act.
req POST /collections/things/records "$TG" \
  "{\"email\":\"dashthing@test.local\",\"password\":\"Password123!\",\"passwordConfirm\":\"Password123!\",\"emailVisibility\":true,\"name\":\"Dash Thing\",\"code\":\"BT1\",\"organization\":\"$ORG\"}"
expect "dashboard cannot create a thing" "403|400|404" "$RCODE" "$RBODY"
req POST /collections/things/records "$TB" \
  "{\"email\":\"memberthing@test.local\",\"password\":\"Password123!\",\"passwordConfirm\":\"Password123!\",\"emailVisibility\":true,\"name\":\"Member Thing\",\"code\":\"MT1\",\"organization\":\"$ORG\"}"
expect "member CAN create a thing (same payload, so the deny was authz)" 200 "$RCODE" "$RBODY"

req PATCH "/collections/things/records/$THING" "$TG" '{"name":"Renamed By Dashboard"}'
expect "dashboard cannot edit a thing" "403|400|404" "$RCODE" "$RBODY"
req PATCH "/collections/things/records/$THING" "$TB" '{"name":"Renamed By Member"}'
expect "member CAN edit a thing (same field, so the deny was authz)" 200 "$RCODE" "$RBODY"

req POST /collections/locations/records "$TG" \
  "{\"name\":\"Dashboard Location\",\"code\":\"BL1\",\"organization\":\"$ORG\"}"
expect "dashboard cannot create a location" "403|400|404" "$RCODE" "$RBODY"
req POST /collections/locations/records "$TB" \
  "{\"name\":\"Member Location\",\"code\":\"ML1\",\"organization\":\"$ORG\"}"
expect "member CAN create a location (same payload, so the deny was authz)" 200 "$RCODE" "$RBODY"
MLOC=$(j "$RBODY" id)
req PATCH "/collections/locations/records/$MLOC" "$TG" '{"name":"Renamed By Dashboard"}'
expect "dashboard cannot edit a location" "403|400|404" "$RCODE" "$RBODY"

# Same quick metadata edit on the locations side (MetadataCard is shared).
req PATCH "/collections/locations/records/$MLOC" "$TB" '{"metadata":{"last_inspection":"2026-03-14"}}'
expect "member CAN patch only metadata on a location" 200 "$RCODE" "$RBODY"
req PATCH "/collections/locations/records/$MLOC" "$TG" '{"metadata":{"last_inspection":"2026-03-15"}}'
expect "dashboard cannot patch a location's metadata (same payload)" "403|400|404" "$RCODE" "$RBODY"

echo ""
echo "=== 11b. viewer reads the inventory and writes none of it ==="
# `viewer` is a tenant's read-only staff. It exists because the allowlists make it
# nearly free: a role value that appears in no write branch is denied everywhere
# by construction, so this section is asserting a property of the EXISTING rules
# rather than of any rule written for viewer. If one of these denies ever starts
# failing, some branch has grown a deny-list.
#
# What viewer is NOT: a read boundary. The read rules on things/locations/types
# are org-scoped with no role check, so viewer reads exactly what member reads --
# the first two checks pin that down deliberately, because the console hides
# write controls from a viewer and it would be easy to later mistake that for the
# server hiding data. It does not. UI navigation is not enforcement.
req GET "/collections/things/records?perPage=1" "$TV"
if [ "$RCODE" = "200" ] && [ "$(jn "$RBODY" 'o.totalItems > 0')" = "true" ]; then
  ok "viewer CAN list things (reads are org-scoped, not role-scoped)"
else
  no "viewer could not list things -- HTTP $RCODE, body $(head -c 200 <<<"$RBODY")"
fi
req GET "/collections/locations/records/$MLOC" "$TV"
expect "viewer CAN read a location record" 200 "$RCODE" "$RBODY"
req GET "/collections/thing_types/records?perPage=1" "$TV"
expect "viewer CAN read thing_types (the console needs them to render a Thing)" 200 "$RCODE" "$RBODY"

# Inventory writes: the line between viewer and member. Each deny is paired with
# the same call as member on the same record, so a blanket deny cannot pass.
req POST /collections/things/records "$TV" \
  "{\"email\":\"viewerthing@test.local\",\"password\":\"Password123!\",\"passwordConfirm\":\"Password123!\",\"emailVisibility\":true,\"name\":\"Viewer Thing\",\"code\":\"VT1\",\"organization\":\"$ORG\"}"
expect "viewer cannot create a thing" "403|400|404" "$RCODE" "$RBODY"
req POST /collections/things/records "$TB" \
  "{\"email\":\"pairthing@test.local\",\"password\":\"Password123!\",\"passwordConfirm\":\"Password123!\",\"emailVisibility\":true,\"name\":\"Pair Thing\",\"code\":\"PT1\",\"organization\":\"$ORG\"}"
expect "member CAN create a thing (same payload, so the deny was authz)" 200 "$RCODE" "$RBODY"
PAIRTHING=$(j "$RBODY" id)

req PATCH "/collections/things/records/$PAIRTHING" "$TV" '{"name":"Renamed By Viewer"}'
expect "viewer cannot edit a thing" "403|400|404" "$RCODE" "$RBODY"
req PATCH "/collections/things/records/$PAIRTHING" "$TB" '{"name":"Renamed By Member"}'
expect "member CAN edit that thing (same field, same record)" 200 "$RCODE" "$RBODY"

# MetadataCard's quick edit is the narrowest inventory write there is, so it is
# the one most likely to be let through by a branch that only froze the
# identity relations. It is a write, and viewer does not hold it.
req PATCH "/collections/things/records/$PAIRTHING" "$TV" '{"metadata":{"asset_tag":"viewer"}}'
expect "viewer cannot patch a thing's metadata" "403|400|404" "$RCODE" "$RBODY"
req PATCH "/collections/things/records/$PAIRTHING" "$TB" '{"metadata":{"asset_tag":"member"}}'
expect "member CAN patch that thing's metadata (same field, same record)" 200 "$RCODE" "$RBODY"

req POST /collections/locations/records "$TV" \
  "{\"name\":\"Viewer Location\",\"code\":\"VL1\",\"organization\":\"$ORG\"}"
expect "viewer cannot create a location" "403|400|404" "$RCODE" "$RBODY"
req PATCH "/collections/locations/records/$MLOC" "$TV" '{"name":"Renamed By Viewer"}'
expect "viewer cannot edit a location" "403|400|404" "$RCODE" "$RBODY"
req PATCH "/collections/locations/records/$MLOC" "$TB" '{"name":"Renamed By Member Again"}'
expect "member CAN edit that location (same field, same record)" 200 "$RCODE" "$RBODY"

req DELETE "/collections/things/records/$PAIRTHING" "$TV"
expect "viewer cannot delete a thing" "403|400|404" "$RCODE" "$RBODY"
req DELETE "/collections/things/records/$PAIRTHING" "$TA"
expect "owner CAN delete that thing (same record, so the deny was authz)" "200|204" "$RCODE" "$RBODY"

# The route bypasses the API rules entirely (app.Save()), so its own role check
# is the only thing standing here -- viewer has to be denied twice over.
req POST "/org/things" "$TV" '{"name":"Viewer Route Thing","code":"viewer-route-1"}'
expect "viewer cannot create a thing through POST /api/org/things" "403|400" "$RCODE" "$RBODY"

# And viewer holds none of the admin surface either -- it sits below member, not
# beside it.
req POST /collections/thing_types/records "$TV" \
  "{\"name\":\"ViewerType\",\"code\":\"VTY1\",\"organization\":\"$ORG\"}"
expect "viewer cannot create thing_types" "403|400|404" "$RCODE" "$RBODY"
req POST /collections/invites/records "$TV" \
  "{\"email\":\"v@test.local\",\"organization\":\"$ORG\",\"role\":\"member\"}"
expect "viewer cannot invite users" "403|400|404" "$RCODE" "$RBODY"
req PATCH "/collections/memberships/records/$MVIEW" "$TV" '{"role":"member"}'
expect "viewer cannot promote itself to member" "403|400|404" "$RCODE" "$RBODY"

echo ""
echo "=== 12. a leaf node reads no NATS collection at all ==="
# leaf-sync config used to read nats_users + nats_accounts through the CRUD API,
# which meant granting a leaf-node identity a read branch on each. Those branches
# are gone: GET /api/leaf/bootstrap serves the same values with the app's own
# privileges, so the edge's blast radius is a fixed list of named fields rather
# than "whatever those rules happen to match".
req POST /collections/leaf_nodes/records "$SU" \
  "{\"email\":\"leaf1@test.local\",\"password\":\"Password123!\",\"passwordConfirm\":\"Password123!\",\"emailVisibility\":true,\"name\":\"Leaf One\",\"code\":\"LEAF1\",\"domain\":\"edge-leaf1\",\"organization\":\"$ORG\",\"synced_collections\":[\"things\",\"locations\"]}"
LEAF=$(j "$RBODY" id)
[ -z "$LEAF" ] && die "leaf node create failed: $RBODY"
sleep 2   # the provisioning hook mints its NATS user asynchronously
req GET "/collections/leaf_nodes/records/$LEAF" "$SU"
LEAF_NATS=$(j "$RBODY" nats_user)
[ -z "$LEAF_NATS" ] && die "leaf node did not get a nats_user: $RBODY"

req POST /collections/leaf_nodes/auth-with-password "" \
  '{"identity":"leaf1@test.local","password":"Password123!"}'
TL=$(j "$RBODY" token)
[ -z "$TL" ] && die "leaf node login failed: $RBODY"

req GET "/collections/nats_users/records" "$TL"
LEAF_SEES=$(jn "$RBODY" 'o.items ? o.items.length : "err"')
if [ "$LEAF_SEES" = "0" ] || [[ "$RCODE" =~ ^(403|404)$ ]]; then
  ok "leaf node sees no nats_users rows (got ${LEAF_SEES:-$RCODE})"
else
  no "leaf node still reads nats_users: $LEAF_SEES row(s) -- $(head -c 200 <<<"$RBODY")"
fi
req GET "/collections/nats_users/records/$LEAF_NATS" "$TL"
expect "leaf node cannot view even its own nats_user" "403|404" "$RCODE" "$RBODY"
req GET "/collections/nats_accounts/records" "$TL"
LEAF_ACCTS=$(jn "$RBODY" 'o.items ? o.items.length : "err"')
if [ "$LEAF_ACCTS" = "0" ] || [[ "$RCODE" =~ ^(403|404)$ ]]; then
  ok "leaf node sees no nats_accounts rows (got ${LEAF_ACCTS:-$RCODE})"
else
  no "leaf node still reads nats_accounts: $LEAF_ACCTS row(s)"
fi

# Pair every "cannot" with a "can": the bootstrap route must still hand it
# everything `leaf-sync config` needs, or the deny above is just a broken edge.
req GET "/leaf/bootstrap" "$TL"
expect "leaf node CAN reach /api/leaf/bootstrap" 200 "$RCODE" "$RBODY"
BS_MISSING=$(jn "$RBODY" \
  '["domain","creds","account_jwt","account_pub","operator_jwt","sys_account_jwt","sys_account_pub"].filter(k=>!o[k]).join(",")')
if [ -z "$BS_MISSING" ]; then
  ok "bootstrap response carries domain, creds, account/operator/sys JWTs and pubkeys"
else
  no "bootstrap response missing: $BS_MISSING"
fi
# The $SYS pair must be the actual system account, not a second copy of the org's.
# Without it the generated nats-leaf.conf cannot resolve the system account the
# operator JWT names, and nats-server refuses to start -- which is exactly the bug
# this check exists to prevent coming back.
BS_SYS_DISTINCT=$(jn "$RBODY" 'o.sys_account_pub !== o.account_pub ? "yes" : "no"')
if [ "$BS_SYS_DISTINCT" = "yes" ]; then
  ok "bootstrap sys_account_pub is the system account, not a copy of the org account"
else
  no "bootstrap sys_account_pub equals account_pub -- the system account lookup is wrong"
fi
req GET "/collections/things/records" "$TL"
expect "leaf node CAN still list the collections it mirrors" 200 "$RCODE" "$RBODY"

# The route is leaf-only: an org owner is not an edge box.
req GET "/leaf/bootstrap" "$TA"
expect "org owner cannot reach the leaf bootstrap route" "401|403|404" "$RCODE" "$RBODY"
req GET "/leaf/bootstrap" ""
expect "anonymous cannot reach the leaf bootstrap route" "401|403|404" "$RCODE" "$RBODY"

echo ""
echo "=== 12b. /api/client-config is console-only, and every console role gets it ==="
# The deployment's browser-facing NATS WebSocket URLs. Not secret -- the
# credential is, and that is row-scoped in nats_users -- but there is no
# pre-login need for it either, since connect() already requires a session and a
# linked nats_user. So it is bound to RequireAuth("users"): no reason to hand an
# unauthenticated scanner the address of the bus.
#
# `dashboard` is the probe on the allow side deliberately. It is the
# zero-authority role AND the appliance login that most needs this value, so an
# over-tight guard here would strand exactly the deployment the setting exists
# for. Device identities are on the deny side: a Thing or leaf node gets its bus
# address from its own config file, not from the console's.
req GET "/client-config" ""
expect "anonymous cannot read the client config" "401|403|404" "$RCODE" "$RBODY"
req GET "/client-config" "$TL"
expect "a leaf node cannot read the console's client config" "401|403|404" "$RCODE" "$RBODY"
req GET "/client-config" "$TG"
expect "dashboard CAN read the client config" 200 "$RCODE" "$RBODY"
CC_URLS=$(jn "$RBODY" 'Array.isArray(o.natsWebsocketUrls)?"array":typeof o.natsWebsocketUrls')
if [ "$CC_URLS" = "array" ]; then
  ok "client config returns natsWebsocketUrls as an array when unset (not null)"
else
  no "client config natsWebsocketUrls should be an array, got: $CC_URLS"
fi

echo ""
echo "=== 13. account + CA writes are operator-only; key ops are a route ==="
# Both updateRules used to carry an owner/admin branch commented "can only change
# rotate_keys", built from `:changed = false` on the limit fields. Deny-list: every
# field NOT named stayed writable -- on nats_accounts that included `jwt` and
# `revocations`, on nebula_ca the CA `certificate`. Now operator-only, with the
# three legitimate key operations behind POST /api/org/nats-account/keys.
req PATCH "/collections/nats_accounts/records/$ACCT" "$TA" '{"description":"owner edit"}'
expect "owner cannot update their org's NATS account" "403|400|404" "$RCODE" "$RBODY"
req PATCH "/collections/nats_accounts/records/$ACCT" "$TA" '{"revocations":{"*":1}}'
expect "owner cannot write the account's revocations list" "403|400|404" "$RCODE" "$RBODY"
req PATCH "/collections/nats_accounts/records/$ACCT" "$TO" '{"description":"operator edit"}'
expect "platform operator CAN update it (same field, so the deny was authz)" 200 "$RCODE" "$RBODY"

# nebula_ca has no rotate_keys field at all, so there is no tenant key operation to
# preserve -- the whole owner/admin branch is gone.
req GET "/collections/nebula_ca/records?filter=(organization='$ORG')" "$SU"
CA=$(jn "$RBODY" 'o.items[0].id')
if [ -n "$CA" ]; then
  req PATCH "/collections/nebula_ca/records/$CA" "$TA" '{"name":"owner renamed CA"}'
  expect "owner cannot update their org's Nebula CA" "403|400|404" "$RCODE" "$RBODY"
  req PATCH "/collections/nebula_ca/records/$CA" "$TO" '{"name":"operator renamed CA"}'
  expect "platform operator CAN update the CA (same field)" 200 "$RCODE" "$RBODY"
else
  # Keep the count stable whether or not this deployment auto-provisions a CA.
  ok "nebula_ca not provisioned for this org; update-rule check skipped"
  ok "nebula_ca not provisioned for this org; operator check skipped"
fi

# Reads must survive: the console's account and CA detail views need them.
req GET "/collections/nats_accounts/records" "$TB"
expect "member can still read the org's NATS account" 200 "$RCODE" "$RBODY"

# The route is the replacement, so it has to actually work for owner/admin --
# and the trigger must reach pb-nats, not just persist a flag. `keymat` is every
# non-secret field a re-key should move.
keymat() { jn "$1" 'JSON.stringify([o.public_key,o.signing_public_key,o.signing_keys])'; }
req GET "/collections/nats_accounts/records/$ACCT" "$SU"
KEYS_BEFORE=$(keymat "$RBODY")
req POST "/org/nats-account/keys" "$TA" '{"action":"rotate"}'
expect "owner CAN rotate account keys through the route" 200 "$RCODE" "$RBODY"
sleep 1
req GET "/collections/nats_accounts/records/$ACCT" "$SU"
KEYS_AFTER=$(keymat "$RBODY")
if [ "$KEYS_AFTER" != "$KEYS_BEFORE" ]; then
  ok "rotation actually re-keyed the account (pb-nats acted on the trigger)"
else
  no "account key material unchanged after rotate: $KEYS_BEFORE"
fi

req POST "/org/nats-account/keys" "$TA" '{"action":"add_signing"}'
expect "owner CAN add a signing key through the route" 200 "$RCODE" "$RBODY"
sleep 1
req GET "/collections/nats_accounts/records/$ACCT" "$SU"
if [ "$(keymat "$RBODY")" != "$KEYS_AFTER" ]; then
  ok "add_signing appended a key (a distinct operation from rotate)"
else
  no "account key material unchanged after add_signing: $KEYS_AFTER"
fi

# ...and refuse everyone else, plus anything outside the action allowlist.
req POST "/org/nats-account/keys" "$TB" '{"action":"rotate"}'
expect "member cannot rotate account keys" "401|403|404" "$RCODE" "$RBODY"
req POST "/org/nats-account/keys" "$TG" '{"action":"rotate"}'
expect "dashboard cannot rotate account keys" "401|403|404" "$RCODE" "$RBODY"
req POST "/org/nats-account/keys" "" '{"action":"rotate"}'
expect "anonymous cannot rotate account keys" "401|403|404" "$RCODE" "$RBODY"
req POST "/org/nats-account/keys" "$TA" '{"action":"set_limits","max_payload":99999999}'
expect "an unknown action is rejected, not silently ignored" 400 "$RCODE" "$RBODY"
req POST "/org/nats-account/keys" "$TA" '{"action":"remove_signing"}'
expect "remove_signing without a public_key is rejected" 400 "$RCODE" "$RBODY"

# The route takes no record id, so it can only ever reach the caller's own org.
# eve owns OtherOrg; her rotate must not touch TestOrg's account.
req PATCH "/collections/users/records/$EVE" "$SU" "{\"current_organization\":\"$ORG2\"}"
TE=$(login eve@test.local)
req GET "/collections/nats_accounts/records/$ACCT" "$SU"
OTHER_BEFORE=$(j "$RBODY" signing_public_key)
req POST "/org/nats-account/keys" "$TE" '{"action":"rotate"}'
EVE_CODE="$RCODE"
sleep 1
req GET "/collections/nats_accounts/records/$ACCT" "$SU"
if [ "$(j "$RBODY" signing_public_key)" = "$OTHER_BEFORE" ]; then
  ok "another org's owner cannot re-key this org's account (HTTP $EVE_CODE, account untouched)"
else
  no "cross-tenant rotation reached TestOrg's account"
fi

echo ""
echo "=== 14. regression: ordinary tenant reads still work ==="
req GET "/collections/organizations/records" "$TB"
expect "member can list their orgs" 200 "$RCODE" "$RBODY"
req GET "/collections/memberships/records" "$TB"
expect "member can list own memberships (org switcher)" 200 "$RCODE" "$RBODY"
req GET "/collections/users/records" "$TA"
expect "owner can list org users" 200 "$RCODE" "$RBODY"
req GET "/collections/things/records" "$TB"
expect "member can list things" 200 "$RCODE" "$RBODY"

echo ""
echo "=== 15. deactivating a device actually takes it off the network ==="
# Runs last on purpose: it revokes DEV_NATS, which re-signs the org account JWT,
# and section 13 compares that account's key material.
#
# `active` is only worth having if it does more than turn a badge red. The
# cautionary example is in-tree: nats_users.active is read into pb-nats's model
# and consulted by nothing in JWT generation, so clearing it leaves the device
# publishing. These checks assert the three effects hooks/active_flag.go must
# have, because the authRule alone gives none of them -- PocketBase evaluates
# authRule at the auth endpoint only, so an already-issued token (7 day lifetime)
# would otherwise outlive the deactivation by a week.
req POST /collections/things/auth-with-password "" \
  '{"identity":"thing1@test.local","password":"Password123!"}'
expect "an active thing CAN authenticate" 200 "$RCODE" "$RBODY"
TT=$(j "$RBODY" token)
[ -z "$TT" ] && die "thing login failed: $RBODY"

# Who may flip it. Same roles as delete: taking a device out of service revokes
# its credential, so it is a management action, not inventory editing.
req PATCH "/collections/things/records/$THING" "$TG" '{"active":false}'
expect "dashboard cannot deactivate a thing" "403|400|404" "$RCODE" "$RBODY"
req PATCH "/collections/things/records/$THING" "$TB" '{"active":false}'
expect "member cannot deactivate a thing" "403|400|404" "$RCODE" "$RBODY"
req PATCH "/collections/things/records/$THING" "$TA" '{"active":false}'
expect "owner CAN deactivate a thing (same field, so the deny was authz)" 200 "$RCODE" "$RBODY"
sleep 1

# Effect 1: no new logins. This is the authRule.
req POST /collections/things/auth-with-password "" \
  '{"identity":"thing1@test.local","password":"Password123!"}'
expect "a deactivated thing cannot authenticate" "400|401|403" "$RCODE" "$RBODY"

# Effect 2: the token it already held is dead. This is RefreshTokenKey(), and it
# is the whole reason the hook exists -- without it the device keeps its API
# access until the token expires on its own.
req GET "/collections/things/records/$THING" "$TT"
expect "the thing's pre-existing auth token is rejected after deactivation" "401|403|404" "$RCODE" "$RBODY"

# Effect 3: the NATS credential is revoked. pb-nats sets active=false as part of
# handling the revoke trigger, so this also proves the trigger reached it.
req GET "/collections/nats_users/records/$DEV_NATS" "$SU"
if [ "$(j "$RBODY" active)" = "false" ]; then
  ok "deactivation revoked the thing's linked NATS identity"
else
  no "linked nats_user still active -- the NATS cascade did not fire"
fi

# Reactivation must issue a FRESH credential: the revocation cutoff in the
# account JWT is permanent, so re-enabling without re-minting would leave a
# device that looks enabled and cannot connect. Baseline is read on the line
# immediately before the call that should change it.
req GET "/collections/nats_users/records/$DEV_NATS" "$SU"
REACT_BEFORE=$(j "$RBODY" creds_file)
req PATCH "/collections/things/records/$THING" "$TA" '{"active":true}'
expect "owner CAN reactivate a thing" 200 "$RCODE" "$RBODY"
sleep 1
req POST /collections/things/auth-with-password "" \
  '{"identity":"thing1@test.local","password":"Password123!"}'
expect "a reactivated thing can authenticate again" 200 "$RCODE" "$RBODY"
req GET "/collections/nats_users/records/$DEV_NATS" "$SU"
if [ "$(j "$RBODY" active)" = "true" ] && [ -n "$(j "$RBODY" creds_file)" ] \
   && [ "$(j "$RBODY" creds_file)" != "$REACT_BEFORE" ]; then
  ok "reactivation re-minted the NATS credential (old .creds stays revoked)"
else
  no "nats_user not re-issued on reactivation -- device would look enabled and fail to connect"
fi

# things.manageRule. Without it, `password` on update requires `oldPassword`
# (forms/record_upsert.go), which nobody holds for a device -- so a Thing's
# PocketBase credential was mint-once with no recovery, while leaf_nodes could be
# reset. The member deny below is the pair that proves this is a role boundary
# and not just PocketBase refusing every password write.
req PATCH "/collections/things/records/$THING" "$TB" \
  '{"password":"NewPassword456!","passwordConfirm":"NewPassword456!"}'
expect "member cannot reset a thing's password" "403|400|404" "$RCODE" "$RBODY"
req PATCH "/collections/things/records/$THING" "$TA" \
  '{"password":"NewPassword456!","passwordConfirm":"NewPassword456!"}'
expect "owner CAN reset a thing's password (same payload, so the deny was authz)" 200 "$RCODE" "$RBODY"
req POST /collections/things/auth-with-password "" \
  '{"identity":"thing1@test.local","password":"NewPassword456!"}'
expect "the reset password actually works" 200 "$RCODE" "$RBODY"

echo ""
echo "=== 16. POST /api/org/things: two authority levels in one endpoint ==="
# The console used to create the nats_users record, the nebula_hosts record, and
# the Thing in three separate client calls. A failure on the last one orphaned a
# signed NATS credential, and the browser never sent `active`, so every Thing it
# made was locked out by things.authRule. The route does all of it in one
# transaction -- and because it writes with app.Save(), it bypasses the API rules
# entirely, so the role checks it makes are the ONLY thing standing here.
req POST "/org/things" "" '{"name":"Anon Thing","code":"anon-1"}'
expect "anonymous cannot create a thing through the route" 401 "$RCODE" "$RBODY"

req POST "/org/things" "$TG" '{"name":"Dash Route Thing","code":"dash-route-1"}'
expect "dashboard cannot create a thing through the route" "403|400" "$RCODE" "$RBODY"

req POST "/org/things" "$TB" '{"name":"Member Route Thing","code":"member-route-1"}'
expect "member CAN create a thing through the route (no identity)" 200 "$RCODE" "$RBODY"
RT_EMAIL=$(j "$RBODY" email)
RT_PASS=$(j "$RBODY" password)

# The bug this route fixes: a bool column has no schema default, so the Thing the
# old client created read active = false and could not authenticate at all.
req POST /collections/things/auth-with-password "" \
  "{\"identity\":\"$RT_EMAIL\",\"password\":\"$RT_PASS\"}"
expect "a thing created through the route can authenticate (active was set)" 200 "$RCODE" "$RBODY"

# The identity half is owner/admin only -- the same boundary things.createRule
# draws by freezing nats_user/nebula_host for the member branch.
req POST "/org/things" "$TB" \
  "{\"name\":\"Member Auto\",\"code\":\"member-auto-1\",\"nats\":{\"mode\":\"auto\",\"role_id\":\"$NROLE\"}}"
expect "member cannot auto-provision an identity through the route" 403 "$RCODE" "$RBODY"
req POST "/org/things" "$TB" \
  "{\"name\":\"Member Link\",\"code\":\"member-link-1\",\"nats\":{\"mode\":\"link\",\"user_id\":\"$DEV_NATS\"}}"
expect "member cannot link an identity through the route" 403 "$RCODE" "$RBODY"

req POST "/org/things" "$TA" \
  "{\"name\":\"Owner Auto\",\"code\":\"owner-auto-1\",\"nats\":{\"mode\":\"auto\",\"role_id\":\"$NROLE\"}}"
expect "owner CAN auto-provision an identity through the route (same payload)" 200 "$RCODE" "$RBODY"
AUTO_THING=$(j "$RBODY" id)
req GET "/collections/things/records/$AUTO_THING" "$SU"
if [ -n "$(j "$RBODY" nats_user)" ]; then
  ok "auto-provisioning minted the identity and linked it to the thing"
else
  no "auto-provisioning returned 200 but left the thing unlinked"
fi

# Cross-tenant: the route resolves the org from the caller's own record, but the
# LINK target comes from the body, so it has to be verified separately.
req GET "/collections/nats_accounts/records?filter=(organization='$ORG2')" "$SU"
OTHER_ACCT=$(jn "$RBODY" 'o.items[0].id')
req GET "/collections/nats_roles/records?filter=(organization='$ORG2')" "$SU"
OTHER_ROLE=$(jn "$RBODY" 'o.items[0].id')
if [ -z "$OTHER_ROLE" ]; then
  req POST /collections/nats_roles/records "$SU" \
    "{\"name\":\"other-test-role\",\"organization\":\"$ORG2\",\"publish_permissions\":[\"test.>\"],\"subscribe_permissions\":[\"test.>\"]}"
  OTHER_ROLE=$(j "$RBODY" id)
fi
req POST /collections/nats_users/records "$SU" \
  "{\"email\":\"other-org-id@nats.test\",\"password\":\"Password123!\",\"passwordConfirm\":\"Password123!\",\"nats_username\":\"other-org-id\",\"account_id\":\"$OTHER_ACCT\",\"role_id\":\"$OTHER_ROLE\",\"organization\":\"$ORG2\",\"active\":true}"
OTHER_NATS=$(j "$RBODY" id)
[ -z "$OTHER_NATS" ] && die "could not create an OtherOrg NATS identity: $RBODY"
req POST "/org/things" "$TA" \
  "{\"name\":\"Cross Tenant\",\"code\":\"cross-1\",\"nats\":{\"mode\":\"link\",\"user_id\":\"$OTHER_NATS\"}}"
expect "owner cannot link another org's NATS identity" 400 "$RCODE" "$RBODY"

# Atomicity. An invalid `type` relation fails the Thing save AFTER the nats_users
# record was created inside the transaction. PocketBase defers the
# *AfterCreateSuccess hooks to commit (core/db.go), so a rollback means pb-nats
# never signed or published either -- but the record itself must also be gone.
req POST "/org/things" "$TA" \
  "{\"name\":\"Rollback Thing\",\"code\":\"rollback-1\",\"type\":\"nonexistenttype00\",\"nats\":{\"mode\":\"auto\",\"role_id\":\"$NROLE\"}}"
expect "a thing create that fails late is rejected" "400|404" "$RCODE" "$RBODY"
req GET "/collections/nats_users/records?filter=(nats_username='rollback-1')" "$SU"
if [ "$(jn "$RBODY" 'o.items.length')" = "0" ]; then
  ok "the rolled-back create left no orphaned NATS identity"
else
  no "rollback leaked a nats_users record -- the transaction is not covering provisioning"
fi


echo ""
echo "=== 17. removing a membership ends the reader's access ==="
# Runs last on purpose: it deletes bob's membership, which every earlier section
# depends on.
#
# Every read rule on the inventory collections is
# `organization = @request.auth.current_organization` with no membership check --
# deliberately, since reads are org-scoped rather than role-scoped. That makes
# users.current_organization the whole read boundary for a console user, and
# nothing was clearing it when the membership behind it went away. The rule then
# still matches, so a removed member keeps reading the tenant's inventory for as
# long as their token lasts, and re-login does not fix it: authRule on `users` is
# empty, so they can log back in and land in the org they were removed from.
#
# The fix is hooks/membership_lifecycle.go. The doctrine this platform wrote down
# for devices -- a flag is not a control unless something acts on it -- applies to
# humans too, and current_organization is that flag.

# Baseline on the line above the action, not borrowed from section 14: a check
# that an operation had an effect is worthless if anything in between could have
# caused it.
req GET "/collections/things/records" "$TB"
BEFORE=$(jn "$RBODY" 'o.items.length')
if [ "$RCODE" = "200" ] && [ "${BEFORE:-0}" -gt 0 ]; then
  ok "member reads $BEFORE thing(s) immediately before losing their membership"
else
  no "member could not read the inventory before the membership delete (HTTP $RCODE, ${BEFORE:-0} items) -- the rest of this section proves nothing"
fi

req DELETE "/collections/memberships/records/$MBOB" "$TA"
expect "owner CAN remove a member" "200|204" "$RCODE" "$RBODY"

# The token is deliberately NOT invalidated. PocketBase loads the auth record
# from the database on every request, so @request.auth.current_organization is
# read live and clearing it takes effect immediately -- unlike a device's
# authRule, which is only evaluated at the auth endpoint and therefore needs
# RefreshTokenKey(). Bob may also still be a legitimate member of other orgs;
# logging him out of those would be a second bug.
req GET "/collections/things/records" "$TB"
AFTER=$(jn "$RBODY" 'o.items.length')
if [ "${AFTER:-1}" = "0" ]; then
  ok "the removed member's inventory read returns nothing (was $BEFORE)"
else
  no "the removed member still reads $AFTER thing(s) -- current_organization was not cleared"
fi

req GET "/collections/things/records" "$TA"
if [ "$RCODE" = "200" ] && [ "$(jn "$RBODY" 'o.items.length')" != "0" ]; then
  ok "the owner still reads the inventory (so the deny above was scoping, not breakage)"
else
  no "the owner lost inventory reads too -- clearing context broke more than it fixed"
fi

req GET "/collections/users/records/$BOB" "$SU"
if [ -z "$(j "$RBODY" current_organization)" ]; then
  ok "current_organization was cleared on the removed member"
else
  no "current_organization still points at $(j "$RBODY" current_organization)"
fi

echo ""
echo "=== 18. code is a per-organization handle, and the database says so ==="
# `code` was documented as unique and used as one -- as a NATS KV key at the
# edge, as a digital-twin key prefix, as the argument to `stone thing get` --
# while nothing enforced it. Two Things sharing a code in one organization did
# not fail: leaf-sync falls back to keying by record id, so the KV key silently
# changes shape and every consumer looking up `thing.S01` finds nothing.
#
# UNIQUE (organization, code) is the enforcement. Scoped per organization, not
# global, because two tenants both calling a site "HQ" is the tenancy model
# working -- so the cross-tenant case below is as much the point as the
# duplicate one.
req POST /collections/locations/records "$SU" \
  "{\"name\":\"Unique Code A\",\"code\":\"UNIQ1\",\"organization\":\"$ORG\"}"
expect "a first location may take a code" 200 "$RCODE" "$RBODY"
req POST /collections/locations/records "$SU" \
  "{\"name\":\"Unique Code B\",\"code\":\"UNIQ1\",\"organization\":\"$ORG\"}"
expect "a second location in the SAME org cannot reuse it" 400 "$RCODE" "$RBODY"
req POST /collections/locations/records "$SU" \
  "{\"name\":\"Unique Code Other Org\",\"code\":\"UNIQ1\",\"organization\":\"$ORG2\"}"
expect "another ORGANIZATION may reuse it (the scoping is the point)" 200 "$RCODE" "$RBODY"

# The index is partial (code != ''), because code is optional on all five
# collections and SQLite treats the empty string as a value. Without the
# WHERE clause an org could hold exactly one blank-coded record, which is a
# restriction nobody asked for.
req POST /collections/locations/records "$SU" \
  "{\"name\":\"No Code One\",\"organization\":\"$ORG\"}"
expect "a location may have no code" 200 "$RCODE" "$RBODY"
req POST /collections/locations/records "$SU" \
  "{\"name\":\"No Code Two\",\"organization\":\"$ORG\"}"
expect "and so may a second one (the index is partial)" 200 "$RCODE" "$RBODY"

# A leaf node's code is frozen after creation, the same way its organization
# is. It is the JetStream domain suffix and the KV key prefix the edge has
# already written under, so changing it centrally orphans everything at the
# site without any error to show for it. Paired with a permitted edit on the
# same record, so a blanket deny cannot pass.
req PATCH "/collections/leaf_nodes/records/$LEAF" "$TA" '{"code":"RENAMED"}'
expect "owner cannot change a leaf node's code" "403|400|404" "$RCODE" "$RBODY"
req PATCH "/collections/leaf_nodes/records/$LEAF" "$TA" '{"name":"Leaf One Renamed"}'
expect "owner CAN rename it (same record, so the deny was the frozen field)" 200 "$RCODE" "$RBODY"

echo ""
echo "=== 19. the observability endpoints are unauthenticated by design ==="
# /api/ready and /metrics are reachable without a session on purpose: the
# callers are container orchestrators, load balancers and uptime checkers, none
# of which hold one. That is an authorization decision rather than an oversight,
# so it is pinned here -- if either endpoint ever starts requiring auth, every
# probe in the field breaks silently and this is the check that says so.
#
# The whole readiness body is public, including each check's detail and
# suggested fix. An operator who wants it closed puts it behind their proxy,
# the same lever that closes /metrics (or sets metrics.token).

req GET /ready ""
expect "anyone CAN probe readiness without a session" "200|503" "$RCODE" "$RBODY"
if [ -n "$(jn "$RBODY" 'o.checks && o.checks.length ? o.checks[0].name : ""')" ]; then
  ok "the readiness body names each check, so a probe can say what is wrong"
else
  no "the readiness body carried no check names"
fi

# Open by default and documented as such: the series carry no customer
# identifiers, and an endpoint needing a credential is one that does not get
# scraped. metrics.token closes it; this asserts the default, not a rule.
RAWCODE=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$PORT/metrics")
expect "metrics are scrapeable without a session (metrics.token is unset here)" 200 "$RAWCODE" ""
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
