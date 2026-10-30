# Plan: inventory events on the bus

Status: implemented (`hooks/inventory_events.go`, `hooks/inventory_exports.go`),
off by default behind `nats.inventory_events_enabled`. Three things moved
during implementation; each is marked **Changed in implementation** below. The
`demo/inventory` polling feed has not been moved over yet.

## The problem

Nothing the Control Plane knows reaches NATS. A rule, a stream processor or a
tenant's own service that wants to react to "a Thing was created", "a Thing was
deactivated" or "a Location moved" has to poll the REST API. For a platform
whose premise is that everything interesting is a subject, inventory being off
the bus is the one real asymmetry in the architecture.

The inventory feed in `demo/inventory` exists because of this gap: rule-router
polls the list API every minute and fans each record out to
`inventory.thing.<code>`. This plan retires that polling. Its rules move to
`stone-age.inventory.thing.>` and key on the payload's `code` where they used a
subject token.

It is also what the documented alerting pattern stands on. Alerting is
rule-router reading device traffic and inventory together, with KV for state
and an HTTP or NATS action as the sink. That only works if inventory is a
subject rule-router can subscribe to.

## Decisions already made

These are settled. Do not re-open them while implementing.

- **Export from the provider account, import into the org account.** The
  Control Plane keeps a single connection and publishes once; NATS does the
  routing. The alternative — a publish-only credential per org and a connection
  pool in the Control Plane — trades provisioning-time data for runtime state
  machines, which is the wrong direction for the component that most needs to
  stay boring.
- **Every organization, behind one global flag.** Not only managed ones. The
  managed gate on `managed_org_exports.go` exists because that export lets the
  provider reach *into* a tenant. This one runs the other way: it hands a
  tenant its own data. Gating it on `managed` would leave unmanaged tenants
  unable to use the alerting pattern at all. The cost, stated plainly: every
  org's inventory now crosses the provider account, and whoever the provider
  lets subscribe to `*.>` there sees all of it. The Control Plane's operators
  can already read every record, so this widens the channel, not the audience.
  `nats.inventory_events_enabled` is the switch for a provider who disagrees.
- **The whole prefix.** `stone-age.<org-code>.>` in the provider account maps
  to `stone-age.>` in the org account. Claiming the namespace now means the next
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
- **Every event carries the whole record, never a diff.** This is what makes
  "no queue" and "no replay" safe: a consumer that misses an event is corrected
  by the next one about the same record. Do not trim the payload to the changed
  fields to save bytes — the `changed` list says what moved, the rest says
  where things now stand.

## Subject design

In the provider (operator) account, the Control Plane publishes:

```
stone-age.<org-code>.inventory.<collection>.<action>
```

**Changed in implementation:** the provider-side subject is rooted at the
local subject's prefix, not at the bare org code. A bare code shares the first
token with everything else in the provider account, so an organization whose
code was `helpdesk` would have been exported every managed org's
`helpdesk.<code>.>` traffic. Under the prefix an org code can only address its
own events, and `main.go` refuses to start if `inventory_local_subject` and
`managed_export_subject` share a first token.

The importing org account sees:

```
stone-age.inventory.<collection>.<action>
```

`<collection>` is `thing` or `location`. `<action>` is `created`, `updated` or
`deleted`. Deactivation is an `updated` whose `changed` list contains `active`;
a second verb for one field would be a second thing to keep in step with
`hooks/active_flag.go`.

`thing_type` is left out of the first cut. Types change rarely, and nothing a
rule needs today turns on a type edit. It is one more allowlist and one more
set of tests; add it the day a consumer asks.

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
  "location_path": "/HQ/hq-floor-1/",
  "active": false,
  "changed": ["active"],
  "ts": "2026-10-21T14:03:11Z"
}
```

Rules:

- Never any field from `nats_users`, `nebula_hosts`, or any `creds_file`,
  `config_yaml`, `private_key`, `password` or `tokenKey` column. The allowlist
  enforces this; a test asserts it.
- `metadata` is included (it is tenant-authored inventory data, and it is what a
  consumer most often needs).
- **`changed`** is on `updated` events only: the allowlisted field names whose
  values differ from the record before the write. rule-router has no previous
  value to compare against, so without it "a Thing was deactivated" cannot be
  written as a rule — `active: false` is on every update to an inactive Thing,
  and renaming a dead device would page someone. With it, the rule is one
  `contains` condition. Only allowlisted names appear; a change to a field
  outside the allowlist yields an event whose `changed` is empty, and the hook
  skips publishing it.
- **`location_path`** on a Thing is its location's `locations.path` (ADR 0004:
  codes from the root down, `/` between and at both ends). Alerting by area
  ("anything under HQ") is the main reason to route on location, and the path
  makes that one rule-router `contains` condition on `/HQ/` instead of a KV
  lookup per message — the end slashes are what keep `/BD-3/` from matching
  `/BD-30/`. A Location's own payload carries its `path` for the same reason.
- The event is self-describing enough to act on without a REST round trip. That
  is the point; a notification-only event would leave the consumer polling,
  which is what this exists to remove.

## Implementation

### 1. The provider-account connection

The Control Plane connects to NATS today on the System Account only, for
`$SYS.REQ.CLAIMS.UPDATE`. App traffic cannot go there, so this needs a second
connection authenticated to the provider account.

It uses an ordinary credential: a `platform-events` `nats_users` record in the
operator org, whose `creds_file` the Control Plane reads at startup. It is
visible in the console, rotates through the existing credential routes, and its
permissions are the provider's to set, per the decision above.

Create it with an idempotent ensure at `serve` startup, not in `bootstrap`.
Deployments that bootstrapped before this existed would otherwise never get one.
Look it up by name in the operator org's account; create it only if absent.

Considered and rejected: the Control Plane signing itself a short-lived user
JWT with the account signing key it already holds. Nothing stored, but it is a
renewal loop that must also recover when the signing key rotates under a live
connection — a runtime state machine to save one database row.

### 2. `hooks/inventory_events.go`

Binds `OnRecordAfterCreateSuccess`, `OnRecordAfterUpdateSuccess` and
`OnRecordAfterDeleteSuccess` on the things and locations collections. The
`Success` hooks fire after the transaction commits, so a rolled-back
`POST /api/org/things` publishes nothing.

- **Call `e.Next()`.** A hook that returns without it silently drops every
  handler bound after it — the trap in `hooks/CLAUDE.md`.
- **Never fail the write.** Unlike `org_provisioning.go`, which deliberately
  returns its error because a half-provisioned org is invisible damage, a failed
  event publish is advisory. Log it and carry on. A Thing that saved but whose
  event did not publish is a missed notification; a Thing that failed to save
  because NATS was briefly unreachable is an outage.
- **Return early when the flag is off,** before doing any work, so the disabled
  case costs one boolean read.
- **No queue.** `nats_publish_queue` exists because an account claim must
  eventually reach the server. An inventory event delivered an hour late is
  worse than one never delivered — a consumer would act on a stale fact. Core
  NATS, fire and forget, log on failure. A tenant needing guarantees binds a
  JetStream stream on their side, which is the decision above.
- **Compute `changed` from `e.Record.Original()`.** The trap: `Original()` is
  the state at the last database READ, not before this write. After a plain
  API request that is the pre-update record. For a record a route or hook
  loaded, changed, saved, changed and saved again, it is not — the second
  event's `changed` would include the first write's fields too. Over-reporting
  is the safe direction, but cover it with a test so it is a known behaviour
  rather than a surprise.

### 3. Writes the platform makes on its own

Two kinds of write change inventory without a request for each record. They
behave differently, and the difference decides what the hook must do.

- **A location move rewrites the subtree in raw SQL, so it fires no hooks.**
  `hooks/location_path.go` updates every descendant's `path` with one `UPDATE`
  that swaps the old prefix, in the move's transaction. Only the moved location
  gets an event. Every descendant location, and every Thing under the subtree,
  now has a different path and gets no event at all. Under the
  whole-record-not-a-diff decision that is not acceptable: the next event is
  what corrects a consumer, and a Thing that never changes again would carry
  a stale `location_path` forever.

  So when a location event's `changed` includes `path`, the hook also reads
  the subtree after the write (`substr(path, 1, length(:new)) = :new`, never
  `LIKE`, for the reason in ADR 0004) and publishes an `updated` for each
  descendant location and each Thing under it, with `"changed": ["path"]` and
  `["location_path"]` respectively. Reads and publishes only — nothing is
  saved, so nothing re-enters a hook. A large move is a burst; say so in the
  tenant docs, and that a rule on path changes sees all of it.
- **Deleting a location goes through PocketBase's relation cleanup, which does
  fire hooks.** PocketBase unsets the deleted id on every referencing record
  with `SaveNoValidate` inside the delete's transaction. That is how each
  child location becomes a root (`TestLocationPath` pins it), and Things
  pointing at the location go through the same path, so each gets an
  `updated` with `"changed": ["location"]` and needs no special handling here.
  Each re-rooted child's own subtree rewrite is a raw `UPDATE` again, so the
  republish above covers it. Pin the Thing half with a test anyway: it is
  PocketBase's behaviour, not ours.

### 4. `hooks/inventory_exports.go`

Mirror image of `managed_org_exports.go`, with the two accounts swapped. Reuse
its `field` / `reconcile` helpers rather than writing new ones.

On every org with a NATS account, while the flag is on, create and reconcile:

| Record | Collection | `account_id` | `name` | `subject` | `local_subject` | `account` |
|---|---|---|---|---|---|---|
| Export | `nats_account_exports` | provider account | `inventory-<org.Id>` | `stone-age.<org-code>.>` | — | — |
| Import | `nats_account_imports` | org's account | `inventory-events` | `stone-age.<org-code>.>` | `stone-age.>` | provider public key |

Both `type: "stream"`. The import carries the tenant's `organization`, so its
owner can see where `stone-age.inventory.>` comes from. **Changed in
implementation:** the export carries the *operator* org, not the tenant's.
It sits on the operator's account, and relation tenancy refuses a record whose
organization differs from its account's. `scripts/test-authz.sh` section 28
checks that a tenant can read its import and not the export.

- **One export per org, not a single `*.>` export.** A wildcard export cannot be
  revoked for one tenant. Per-org means removing an org removes exactly its own
  wiring.
- **Name records by `org.Id`, key subjects on `org.code`.** Exactly the reasoning
  in `managed_org_exports.go`. A code is frozen once set, so the only
  transition is an org that had none getting its first; the ensure refuses an
  org with no code (`run migrate up`), exactly as the managed one does.
- **Create on org create; remove on org delete.** **Changed in
  implementation:** the pair is removed *before* the delete, in its
  transaction, not after it. Deleting an organization blanks rather than
  cascades, so PocketBase unsets `organization` on each referencing record in
  turn, and for a moment the import and its account disagree. Relation tenancy
  refuses that and fails the whole delete. The managed helpdesk export has the
  same defect today; a managed org cannot be deleted.
- **Sweep at startup.** The flag is a config value, so turning it on or off
  takes effect on restart. At `serve` startup, ensure the pair on every org when
  the flag is on, and remove every pair when it is off. That is also how
  existing organizations get wired on the first boot after upgrade.
- **Reconcile, do not just create.** Every value here except the two lookup keys
  can drift, and saving an export or import record makes pb-nats republish the
  account JWT — so a no-op must not write. On a startup sweep across every org,
  this is the difference between a quiet restart and republishing every account.

### 5. Config

```yaml
nats:
  inventory_events_enabled: false        # off until someone wants it
  inventory_local_subject: "stone-age.>" # what the org account sees
```

`inventory_local_subject` stays configurable because of branding: a provider
running the platform under their own name may not want `stone-age.` in every
tenant's subjects. Validate it ends in `.>` at startup, the way
`nats.managed_export_subject` already is in `main.go`.

## Tests

- Subject construction and the field allowlist, as pure functions.
- A test that asserts no credential-bearing field can appear in a payload, by
  name, for every collection in the allowlist. This is the one that matters.
- `changed`: lists exactly the allowlisted fields that moved; empty (and nothing
  published) for a write that touches only non-allowlisted fields; the
  double-save case through `app.Save()` documented as over-reporting.
- Moving a location: one event for the location itself, plus one per
  descendant location and per Thing under the subtree, each carrying the new
  path.
- Deleting a location: its Things each get an `updated` with
  `"changed": ["location"]` from PocketBase's relation cleanup.
- Export/import pair created on org create, removed on delete, reconciled rather
  than duplicated on re-save, and the startup sweep writing nothing when
  everything is already in place. Model it on
  `hooks/managed_org_exports_test.go`.
- The `platform-events` user is created once and found, not duplicated, on the
  next start.
- `scripts/test-authz.sh`: the new export and import records are org-scoped
  like every other record. Add the checks and bump `EXPECTED_CHECKS`.
- A publish failure does not fail the record write.

## Not doing

- No JetStream across the account boundary.
- No per-org connections from the Control Plane.
- No self-signed, self-renewing credential for the Control Plane.
- No `thing_type` events until somebody asks for one.
- No backfill or replay. A consumer that needs current state reads REST or
  `stone pull` once and then follows the subject.
- No diffs. Every event is the whole allowlisted record plus `changed`.
- No permission provisioning on either side.
- No per-record subject token until somebody asks for one.
