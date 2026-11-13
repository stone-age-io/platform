# hooks/CLAUDE.md - server-side hooks and routes

Claude Code loads this file when it touches anything under `hooks/`. Before you
write a hook that enforces anything, a route that saves with `app.Save()`, or
anything that touches a credential, read `AUTHORIZATION.md` at the repo root.

## PocketBase hook mechanics

- **A hook that returns without `e.Next()` silently drops every handler bound
  after it**: no error, no log. `TestOrgCreateDoesNotTerminateTheHookChain`
  guards the organization hooks. `internal/testutil` registers them before
  Bootstrap, as `main.go` does.
- **Tenancy is in-tree**: `org_membership.go` (the owner membership) and
  `invites.go` (token, expiry, mail, resend, `POST /api/org/invites/accept`).
  `schema.json` owns the three collections and their rules.
- **pb-nats trigger fields (`regenerate`, `revoke`, `rotate_keys`,
  `add_signing_key`, `remove_signing_key`) fire from `app.Save()` only because
  pb-nats binds them to the MODEL hook (`OnRecordUpdate`).** If a route that sets
  a trigger appears to do nothing, check which hook the library binds first.
- **`Record.Original()` is the state at the last database READ, not before this
  write.** It agrees with "before" for a REST request, which loads the row first,
  and diverges when one in-memory record is saved twice. So:
  - an edge-triggered hook (`was != now`) is safe only where the trigger is
    always a request. `active_flag.go` keys on an edge; `org_active_flag.go`
    is level-triggered because it must also hold for `app.Save()`;
  - a test of an edge-keyed hook outside a request must **reload the record
    before the update**, as `TestRelationTenancy` does. A record built in memory
    has a blank `Original()`, so the hook never fires and the test passes
    against a hook that does nothing.
- **A record request hook does not see the body the client sent.**
  `RequestInfo().Body` holds each field's PREPARED value, so a number field's
  `null` and `""` are already 0, and `max_payload-` is the plain field name. To
  tell blank from 0, bind the body again with `e.BindBody` (every body is
  rereadable). `nats_limits.go` (`rawRequestBody`) does this.

## Features

The numbers match the feature index in the root `CLAUDE.md`.

7. **Audit logging** - operator-only: `audit_logs` has no `organization` column
    to scope a tenant read on. Every event records `changed_fields`; old and new
    values only for `auditSnapshotCollections` (`main.go`), an allowlist for the
    reason in `AUTHORIZATION.md`'s credential bullet.

7b. **Tenant activity feed** (`activity.go`) - who changed what, org-scoped,
    readable by every role, with no record values.
    - **An entry is visible to exactly those who could read the record it
      describes.** So it covers only the five collections whose reads are
      org-scoped with no role branch (`things`, `locations`, `thing_types`,
      `location_types`, `thing_type_operations`). Adding to
      `activityCollections` is an authorization change, pinned by
      `activity_internal_test.go`.
    - **Bound to the REQUEST hooks**, the only layer that knows the actor
      (nothing in PocketBase puts auth into `RecordEvent.Context`). The id is
      captured after `e.Next()`; everything else is the pre-write snapshot.
    - `actor` is plain text and labels are snapshots, so a user delete or a
      rename does not rewrite history. Write rules are all `nil`.
    - The console side (gating "Open record", timestamps): `ui/CLAUDE.md`.

11. **Operator org and managed orgs** - bootstrap creates the operator's org
    (`is_operator_org`) beside the `$SYS` org; its NATS account is the hub for
    shared services. Flagging an org `managed` provisions a `helpdesk.>` export
    from its account (`nats.managed_export_subject`) and a hub-side import
    remapped to `helpdesk.{code}.>`, so provenance is in the subject.
    - **Both records are read-only in the console**, because every org save
      reconciles them. The console recognises them by name in
      `ui/src/utils/managedExports.ts`, which `managed_org_exports_test.go` reads.
    - Why reconcile rather than create-if-missing, and why the import record is
      keyed by `org.Id` while the subject uses the code: `managed_org_exports.go`.

12. **Edge sites** - an edge site is a **Thing**. The agent signs in as it and
    calls `GET /api/me/leaf-config` for everything a leaf needs, in ten named
    fields, so a gateway has **no read grant on `nats_*` or `nebula_*`**. Extend
    the route rather than add a read branch.
    - **Nothing marks a Thing as a gateway.** Do not write code that assumes a
      marker.
    - **The JetStream domain is the Thing's code**, computed, never stored.
    - **`nats export` and the route both read `nats.jetstream_domain`** (the
      route serves it as `hub_domain`); keep it that way. The route answers 503
      naming the key when `leaf_url` or the domain is empty.
    - **The agent caches `hub_domain`**, so changing it reaches a running agent
      only when it re-runs `agent -leaf-config`.
    - **Site liveness is asked of NATS, never stored**: `$SYS.REQ.ACCOUNT.PING.CONNZ`
      for leaves, `$SRV.PING.stone-agent` for agents. Discovery needs `$SRV.>` on
      subscribe, and behind a leaf the uplink credential needs it too
      (`TestServiceDiscoveryCrossesALeafOnlyIfTheUplinkAllowsIt`).
    - **Tenant roles carry no `$SYS` publish deny.** The account boundary already
      blocks the operator-wide endpoints, and a DENY beats an ALLOW, so a deny
      kills account monitoring with a bare timeout
      (`internal/demoseed/contract.go`, `internal/health/leaf_visibility_test.go`).
    - Why the route gates on nothing: `leaf_config_routes.go`.

14. **Readiness and metrics** - `/api/ready` and `/metrics`; checks in
    `readiness.go` and `internal/health`, collectors in `metrics.go` and
    `internal/metrics`. What they report: platform-docs `health-metrics.md`.
    - **A check must be answerable first-hand.** The Control Plane has no
      credential in any organization's account; do not mint it one. **No
      per-org labels.**
    - **No check that cannot fail, no metric that cannot vary** — but verify a
      claimed limitation before cutting on it.
    - **NATS server metrics are not ours**: scrape the server's monitoring port
      with `prometheus-nats-exporter`.
    - **Say so in the HELP text** of any metric mistakable for health, such as
      `stone_age_records`, which counts configured devices.
    - Nebula expiry has two windows, 30 days for a host and 90 for a CA:
      `cert_expiry.go`, `TestCAGetsALongerWarningWindowThanAHost`, and
      `ui/src/utils/expiry.ts`.

15. **Decommissioning a device** - `things.active`, owner/admin only, enforced
    in four places: the `authRule`, a `tokenKey` refresh in `active_flag.go`, and
    `active` mirrored onto the linked `nats_user` and `nebula_host`.
    - **Deactivate, then delete**: revoking a Nebula certificate needs it in the
      database to fingerprint, and a delete reaches neither identity. So
      `guardThingDelete` refuses a Thing delete while its NATS user or Nebula
      host is active. It checks the identities, not the Thing, because the
      mirror is best-effort.
    - **It mirrors `active`, not `revoke`.** pb-nats checks `revoke` first, and
      revoke hands back a working replacement.
    - The Nebula half applies on redeploy; Nebula has no CRL.
    - **While the Thing is deactivated, `guardLinkedIdentity` refuses** any save
      that leaves its NATS user or Nebula host active, and any minting trigger
      (`regenerate`, `revoke`, `renew`), whatever the libraries do on their own.
      `TestDeactivatedThingIdentitiesStayDown`. It is an invariant hook, at
      negative priority, level-keyed. Any new minting trigger goes on its list.

15b. **Rotating an organization's Nebula CA** - `POST /api/org/nebula-ca/rotate`,
    owner/admin, `prepare` / `commit` / `finish`. A route because a rule cannot
    allow one field and forbid the rest. Three steps because the wait between
    them is what keeps the mesh together. `nebula_routes.go`,
    `nebula_rotation_test.go`, `scripts/test-authz.sh` section 13b.

15d. **Suspending an organization** - `organizations.active`, operator-only.
    `org_active_flag.go` mirrors it onto `nats_accounts.active`, which withdraws
    the account claim, so the tenant disconnects from the hub and stays off
    across restarts.
    - **The hub only.** A site's leaf keeps its preloaded account JWT, so devices
      on a leaf keep working locally and the site is islanded.
    - **Reversible**: no revocation cutoff and nothing re-minted. Contrast the
      device flag, whose cutoff is permanent.
    - **Deliberately narrow**: not Nebula, not the console, not Thing tokens. A
      suspended tenant still signs in and reads what it owns; say so in any UI
      that shows it.
    - The mirror is level-triggered; the refusal hook beside it is edge-keyed, so
      it does not block the save that fixes an already-inactive infrastructure
      org. Both infrastructure orgs are exempt from the mirror and the flip.
    - A new account inherits the org's flag at provisioning time.
    - **An org is born active** unless the create request sends `active`. A
      request hook, because only the request can tell an omitted field from an
      explicit `false`.

16. **Organization code** (ADR 0002 in platform-docs) - **ids for storage, codes
    for addressing.** `organizations.code` is the one globally unique identifier;
    every other code is unique per org (`UNIQUE (organization, code COLLATE
    NOCASE) WHERE code != ''`).
    - **Optional but immutable** through the API (`@request.body.code:changed =
      false`). A superuser in the admin panel is not bound by it.
    - **Refuse on collision, never auto-suffix.** A leading digit is valid.
      `org_code.go`, and `orgCodeFor` in `bootstrap.go`.
    - The review question: not "is this identifier unique" but **"whose database
      does this identifier belong to"**.

16b. **Thing and Location codes** (ADR 0003, `codes.go`).
    - **A blank code is generated on create**: `PFX-XXX-XXX` under the type's
      prefix, or `XXX-XXX`, from 30 symbols with `crypto/rand`. Random so a typo
      lands on "not found", not a neighbour. Typed codes are kept as typed.
    - `POST /api/org/things` generates its own with the same `NewUniqueCode`,
      because the email and NATS username are built from it.
    - **No client-side generator**, and no way to get a code before its record
      exists.
    - **Uniqueness ignores case; storage does not.** Case-free lookups use
      `code:lower = {:c}` with a lowercased param.
    - **`prefix`** (`^[A-Z]{1,4}$`, editable): Thing and Location prefixes are
      separate sets, enforced across the two tables by a hook.
    - **`type` is frozen once set** on things and locations.
    - **A Thing Type's `subject_prefix` is frozen while any Thing uses the
      type**: an edit would move every subject of those Things. A hook, since
      a rule cannot count the records that point at this one.
    - **The default subject is `{thing_type_code}.{thing}`**
      (`ui/src/utils/subjectResolver.ts`).

16c. **`locations.path`** (ADR 0004, `location_path.go`).
    - The codes from the root down with `/` at both ends: `/KC/BD-3/RM-204/`.
    - **The hook is the only writer**, and there is deliberately no
      `path:changed = false` term, so a stale form does not 404.
    - **A move rewrites the subtree** in one UPDATE, compared with `substr`,
      never `LIKE` (codes may contain `_`), reading the old path from the table.
    - Refused: a parent in another org, the location itself, a descendant, a
      parent with no path.
    - A deleted parent needs no hook: PocketBase unsets each child's relation
      inside the delete, which fires the update hook (`TestLocationPath`).
    - A code-less location uses its id as its segment. Register after
      `RegisterCodes`.

## A few files, in more detail

- `nats_limits.go` - an unset NATS role limit is stored as -1, and a request
  that sends 0 for a role or account limit is refused. Two layers, because only
  a request can tell omitted from 0. Long form: `AUTHORIZATION.md`.
- `org_provisioning.go` - create-if-missing on create and update, so re-saving
  an organization retries a failed provision. Failures are returned after
  `e.Next()`, so a NATS outage cannot cost the owner their membership.
- `internal/metrics/` and `internal/health/` have copies in the agent repo, on
  purpose: two small copies are cheaper than a shared module.
- `internal/health/leaf_visibility_test.go` - the nats-server behaviours site
  liveness depends on, checked against a real hub and leaf.
