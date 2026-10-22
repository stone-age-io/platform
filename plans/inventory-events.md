# Plan: inventory events on the bus

Status: not scheduled. Written down so the design is settled before anyone needs it.

## The problem

Nothing the Control Plane knows reaches NATS. A rule, a stream processor or a
tenant's own service that wants to react to "a Thing was created", "a Thing was
deactivated" or "a Location moved" has to poll the REST API. For a platform
whose premise is that everything interesting is a subject, inventory being off
the bus is the one real asymmetry in the architecture.

## Decisions already made

These are settled. Do not re-open them while implementing.

- **Export from the provider account, import into the org account.** The
  Control Plane keeps a single connection and publishes once; NATS does the
  routing. The alternative — a publish-only credential per org and a connection
  pool in the Control Plane — trades provisioning-time data for runtime state
  machines, which is the wrong direction for the component that most needs to
  stay boring.
- **Managed organizations only.** Same gate as `managed_org_exports.go`. An
  unmanaged org gets nothing and needs no cleanup.
- **The whole prefix.** `<org-code>.>` in the provider account maps to
  `stone-age.>` in the org account. Claiming the namespace now means the next
  platform-to-tenant channel (config pushes, announcements, a revocation nudge)
  needs no second provisioning dance. It also gives tenants one rule: anything
  under `stone-age.` came from the Control Plane, everything else is theirs.
- **Core NATS at the boundary.** No JetStream across the account line. A tenant
  that wants durability binds its own stream to `stone-age.>` inside its own
  account, where it is an ordinary subject and the retention decision is theirs.
- **No permission provisioning.** Who on the provider side may subscribe to
  `*.>`, and what a tenant's own roles may do with `stone-age.>`, is the
  provider's configuration. Provisioning creates the export/import pair and
  nothing else.

## Subject design

In the provider (operator) account, the Control Plane publishes:

```
<org-code>.inventory.<collection>.<action>
```

The importing org account sees:

```
stone-age.inventory.<collection>.<action>
```

`<collection>` is `thing`, `location` or `thing_type`. `<action>` is `created`,
`updated` or `deleted`. Deactivation is an `updated` whose payload carries
`active: false`; a second verb for one field would be a second thing to keep in
step with `hooks/active_flag.go`.

The record's code is in the payload, not the subject. Filtering by collection
and action is what consumers actually want, and a per-record token can be added
later without breaking a `stone-age.inventory.>` subscription. Adding one now
would be speculative.

## Payload

A JSON object with a **field allowlist per collection**, in the shape
`auditSnapshotCollections` already uses in `main.go`. An allowlist, never a
record dump: the day someone adds a credential-bearing field to a collection,
a denylist leaks it and an allowlist does not.

```json
{
  "code": "cam-lobby",
  "id": "a1b2c3...",
  "name": "Lobby Camera",
  "type": "camera",
  "location": "hq-floor-1",
  "active": true,
  "ts": "2026-10-21T14:03:11Z"
}
```

Rules:

- Never any field from `nats_users`, `nebula_hosts`, or any `creds_file`,
  `config_yaml`, `private_key`, `password` or `tokenKey` column. The allowlist
  enforces this; a test asserts it.
- `metadata` is included (it is tenant-authored inventory data, and it is what a
  consumer most often needs).
- The event is self-describing enough to act on without a REST round trip. That
  is the point; a notification-only event would leave the consumer polling,
  which is what this exists to remove.

## Implementation

### 1. The provider-account connection

The Control Plane connects to NATS today on the System Account only, for
`$SYS.REQ.CLAIMS.UPDATE`. App traffic cannot go there, so this needs a second
connection authenticated to the provider account.

Preferred: the Control Plane holds the account signing key already, so it signs
itself a short-lived user JWT for the provider account at startup and renews it.
No stored credential, nothing to rotate by hand, nothing new in the database.

Fallback if that proves awkward: `bootstrap` mints a `platform-events`
`nats_users` record in the operator org and the Control Plane reads its
`creds_file`. Its permissions are then the provider's to set, per the decision
above.

This is the one genuinely open question in this plan. Resolve it first; the rest
does not depend on which way it goes.

### 2. `hooks/inventory_events.go`

Binds `OnRecordAfterCreateSuccess`, `OnRecordAfterUpdateSuccess` and
`OnRecordAfterDeleteSuccess` on the things, locations and thing-type
collections.

- **Call `e.Next()`.** A hook that returns without it silently drops every
  handler bound after it — the trap in `hooks/CLAUDE.md`.
- **Never fail the write.** Unlike `org_provisioning.go`, which deliberately
  returns its error because a half-provisioned org is invisible damage, a failed
  event publish is advisory. Log it and carry on. A Thing that saved but whose
  event did not publish is a missed notification; a Thing that failed to save
  because NATS was briefly unreachable is an outage.
- **Skip unmanaged orgs** before doing any work, so the common case costs one
  boolean read.
- **No queue.** `nats_publish_queue` exists because an account claim must
  eventually reach the server. An inventory event delivered an hour late is
  worse than one never delivered — a consumer would act on a stale fact. Core
  NATS, fire and forget, log on failure. A tenant needing guarantees binds a
  JetStream stream on their side, which is the decision above.

### 3. `hooks/inventory_exports.go`

Mirror image of `managed_org_exports.go`, with the two accounts swapped. Reuse
its `field` / `reconcile` helpers rather than writing new ones.

On a managed org, create and reconcile:

| Record | Collection | `account_id` | `name` | `subject` | `local_subject` | `account` |
|---|---|---|---|---|---|---|
| Export | `nats_account_exports` | provider account | `inventory-<org.Id>` | `<org-code>.>` | — | — |
| Import | `nats_account_imports` | org's account | `inventory-events` | `<org-code>.>` | `stone-age.>` | provider public key |

Both `type: "stream"`. Both carry `organization` so the tenant-scoped list rules
see them.

- **One export per org, not a single `*.>` export.** A wildcard export cannot be
  revoked for one tenant. Per-org means un-managing an org removes exactly its
  own wiring.
- **Name records by `org.Id`, key subjects on `org.code`.** Exactly the reasoning
  in `managed_org_exports.go`: record identity on the immutable id, the
  addressable value on the subject, so a change is a reconcile rather than a
  delete-and-recreate.
- **Remove the pair when `managed` goes false, and on org delete.** The org-side
  import cascades with its account; the provider-side export does not, so remove
  it explicitly.
- **Reconcile, do not just create.** Every value here except the two lookup keys
  can drift, and saving an export or import record makes pb-nats republish the
  account JWT — so a no-op must not write.

### 4. Config

```yaml
nats:
  inventory_events_enabled: false        # off until someone wants it
  inventory_local_subject: "stone-age.>" # what the org account sees
```

Validate the local subject ends in `.>` at startup, the way
`nats.managed_export_subject` already is in `main.go`.

## Tests

- Subject construction and the field allowlist, as pure functions.
- A test that asserts no credential-bearing field can appear in a payload, by
  name, for every collection in the allowlist. This is the one that matters.
- Export/import pair created on `managed = true`, removed on `managed = false`
  and on delete, reconciled rather than duplicated on re-save. Model it on
  `hooks/managed_org_exports_test.go`.
- `scripts/test-authz.sh`: the new export and import records are org-scoped
  like every other record. Add the checks and bump `EXPECTED_CHECKS`.
- A publish failure does not fail the record write.

## Not doing

- No JetStream across the account boundary.
- No per-org connections from the Control Plane.
- No events for unmanaged organizations.
- No backfill or replay. A consumer that needs current state reads REST or
  `stone pull` once and then follows the subject.
- No permission provisioning on either side.
- No per-record subject token until somebody asks for one.
