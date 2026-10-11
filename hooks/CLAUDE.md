# hooks/CLAUDE.md - server-side hooks and routes

Claude Code loads this file when it touches anything under `hooks/`. Before you
write a hook that enforces anything, a route that saves with `app.Save()`, or
anything that touches a credential, read `AUTHORIZATION.md` at the repo root. The
one-line rules are in the root `CLAUDE.md`; the reasoning is there.

## PocketBase hook mechanics

- **A hook that returns without `e.Next()` silently deletes every handler bound
  after it.** FIXED, and worth keeping written down because of how it presented.
  pb-tenancy's `organizations` AfterCreateSuccess handler was
  `return autoCreateOwnerMembership(...)` with no `e.Next()`, and it registered
  from `Setup`'s `OnBootstrap` callback rather than from `Setup` itself — which
  put it after every `hooks.Register*` call, so it was always last and nothing
  downstream ever noticed. Anything binding later got silence: no handler, no
  error, no log. `RegisterManagedOrgExports` on an already-bootstrapped test app
  provisioned nothing, and a `Priority: -9999` bind was the only thing that
  fired. That hook is now `hooks.RegisterOrgMembership` and it calls `e.Next()`,
  guarded by `TestOrgCreateDoesNotTerminateTheHookChain`, which binds on the
  bootstrapped app the harness hands out — precisely the case that used to be
  inert. `internal/testutil` still registers organization hooks before Bootstrap,
  now as a convention (be equivalent to main.go) rather than a requirement.
- **Tenancy is platform code, not a library.** `organizations`, `memberships` and
  `invites` were pb-tenancy until it was absorbed. Only half of it ever ran here:
  `schema.json` has owned those collections and every one of their API rules since
  the initial import, so the library's `createCollections`/`setAPIRules` half was
  dead, and its roles (`owner`/`admin`/`member`) had already been outgrown by
  `viewer` and `dashboard`. What came back in-tree is the half that did run —
  `hooks/org_membership.go` (owner membership) and `hooks/invites.go` (token,
  expiry, mail, resend, `POST /api/org/invites/accept`).
- **pb-nats trigger fields only fire from a route if pb-nats watches them on the
  MODEL hook.** `regenerate`, `revoke`, `rotate_keys`, `add_signing_key` and
  `remove_signing_key` are all handled in `pb-nats internal/sync/manager.go`. Those
  handlers used to be bound to `OnRecordUpdateRequest`, which fires **only for REST
  requests** — so a route doing `app.Save()` persisted the flag and nothing acted on
  it, leaving it set to fire later on an unrelated update. They are now on
  `OnRecordUpdate`. If a trigger-setting route ever appears to do nothing, check
  which hook the library binds before debugging the route.
- **`Record.Original()` is the state at the last database READ, not the state
  before this write.** `Record.originalData` is refreshed in exactly one place —
  `Record.PostScan` (`core/record_model.go`), which runs when the row is loaded —
  and by nothing in the save path. So the two readings agree for a REST request,
  where PocketBase loads the row before applying the patch, and diverge the
  moment one in-memory record is saved twice: the second save compares against
  the value from before the *first* one. That makes an edge-triggered hook
  (`was != now`) safe only where the trigger is always a request.
  `hooks/active_flag.go` keys on an edge for that reason and is fine;
  `hooks/org_active_flag.go` deliberately does not, because a mirror that must
  also hold for `app.Save()` cannot assume a fresh load — its edge build
  mirrored a deactivation and then silently declined to undo it, which is worse
  than the inert flag it was written to replace. Where an edge really is wanted
  outside a request (pb-nats's `hasPriorState`, the relation-tenancy guard in
  `AUTHORIZATION.md`), the test has to
  **reload the record before the update**, exactly as `TestRelationTenancy`
  documents for its own subtest: a record built in memory carries an `Original()`
  describing the blank pre-create state, so `false == false` and the hook never
  fires. Both of those suites would otherwise pass against a hook that does
  nothing.

## Features

The numbers match the feature index in the root `CLAUDE.md`.

7. **Audit Logging** - Comprehensive audit trail with searchable viewer
    (operator-only: `audit_logs` has no `organization` column, so there is
    nothing to scope a tenant read on). Every event records `changed_fields`;
    full before/after values only for the collections in
    `auditSnapshotCollections` (`main.go`). See the credential bullet in
    `AUTHORIZATION.md` for why that list is short and why it is an
    allowlist.

7b. **Tenant activity feed** (`activity`) - who changed what in the console,
    org-scoped and readable by every role. NOT `audit_logs`, which stays
    operator-only: this carries no record values at all, only actor, action,
    resource and snapshotted labels.
    - **The invariant: an entry is visible to exactly those who could read the
      record it describes.** A flat collection mirroring other collections
      inherits none of their scoping — the lesson `audit_logs` taught. So the
      feed covers only the five tenant collections whose own reads are
      org-scoped with no role branch (`things`, `locations`, `thing_types`,
      `location_types`, `thing_type_operations`) and NOT `memberships`,
      `invites`, `nats_roles` or `nebula_networks`, whose reads stop at
      owner/admin. Adding to `activityCollections` is an authorization change,
      pinned by `hooks/activity_internal_test.go`.
    - **Bound to the REQUEST hooks, because they are the only layer that knows
      the actor.** `core.RecordEvent` carries a `Context`, but nothing in
      PocketBase ever puts auth into it — the only `context.WithValue` calls in
      the framework are its own tests, which is why pb-audit passes `nil` for
      request info on its success hooks.
    - **The id is the one field captured AFTER `e.Next()`**: a new record has no
      id until PocketBase assigns it during the save. Everything else is the
      pre-write snapshot, so an entry describes what the request asked for
      rather than what the request plus every server-side hook did.
    - `actor` is plain text, not a relation: a non-cascade relation is *blanked*
      on user delete, one write per row. Labels are snapshots for the same
      reason — a log that changes when a record is renamed is not a log.
    - Write rules are all `nil`, so the log cannot be forged or edited through
      the API. Deliberately no changed-field list, no retention cron and no
      transaction dance — `hooks/activity.go` says why.
    - **The console's "Open record" button is gated on the capability the
      TARGET route needs, not the one that opened the feed.** Every role in an
      org reads the feed, but the three type collections have no detail view —
      their only route is the edit form behind `manageDefinitions` — so an
      ungated link would bounce a `viewer` to `/`. `RESOURCE_ROUTES` in
      `ui/src/views/activity/ActivityView.vue` maps each snapshotted noun to a
      route plus its capability, and `TestActivityNounsAllHaveAConsoleRoute`
      reads that file so adding a sixth collection to `activityCollections`
      cannot silently ship entries with no way to reach the record. Nothing
      in the dialog is a live join: the label and the noun are both snapshots,
      which the dialog says out loud so an accurate feed does not read as a
      stale one.
    - **Both timestamps on every covered entity's detail surface.** A feed that
      reports an update is only useful beside a record that admits when it
      changed, so `RecordTimestamps.vue` (relative headline, exact underneath —
      the same shape the feed uses) is on the Thing and Location detail views
      and on the three type FORM views, which are those collections' only
      detail surface. No "never edited" marker: PocketBase's autodate fields
      each call `types.NowDateTime()` separately, so `created === updated` on
      an untouched record is a coin flip.

11. **Operator Org & Managed Orgs** - Bootstrap creates the operator's own org
    (`is_operator_org`) beside the `$SYS` org (`is_system_org`); its NATS account
    is the hub for shared operator services. Flagging a customer org `managed`
    provisions a `helpdesk.>` stream export from its account (configurable:
    `nats.managed_export_subject`) plus a hub-side import remapped to
    `helpdesk.{organizations.code}.>`, so event provenance is subject-based and
    cannot be forged by the publisher.
    - **Both records are read-only in the console.** Reconciliation rewrites them
      on every org save, so an Edit button would offer a change that is silently
      undone. Recognised by name in `ui/src/utils/managedExports.ts`, which
      `hooks/managed_org_exports_test.go` reads so the two cannot drift.
    - Why `reconcile` rather than create-if-missing, and why the import *record*
      is still keyed by the immutable `org.Id` while the *subject* token is the
      code: `hooks/managed_org_exports.go`.

12. **Edge sites** - An edge site is a **Thing**. The agent
    ([stone-age-io/agent](https://github.com/stone-age-io/agent)) authenticates
    against `things` and calls `GET /api/me/leaf-config` for everything a NATS
    leaf server needs, in ten named fields. So a gateway needs **no read grant on
    any `nats_*` or `nebula_*` collection**, and `nats_system_operator` stays
    superuser-only. Don't add a read branch to those collections to make an edge
    feature work — extend the route.
    - **Nothing in the schema marks which Things are gateways.** `thing_types`
      has no such field; "gateway" is a naming convention a tenant picks. Do not
      write code that assumes the marker exists — `CLAUDE.md` once claimed it did,
      and a console feature was built on the strength of it and then removed.
    - **The JetStream domain is the Thing's code**, computed and never stored. A
      stored column could disagree with the code, and the disagreement surfaced
      as a site that simply stopped appearing.
    - **`hub_domain` is cached by the agent, so changing it is not a live change.**
      The route serves it, but the agent stores it in its platform session file and
      re-fetches only when that is empty. A deployment that moves its hub's
      JetStream domain will not reach running agents until each re-runs
      `agent -leaf-config` — which is defensible, since such a move invalidates
      every leaf's generated `nats-leaf.conf` anyway, but it is not something a
      console action can push.
    - **Site liveness is asked of NATS, never stored** — no heartbeat, no
      `leaf_status` bucket. A heartbeat travels over the very link whose failure
      it reports. It is a dashboard widget recipe against
      `$SYS.REQ.ACCOUNT.PING.CONNZ`, deliberately not a screen.
    - **The agent is a NATS micro service, `stone-agent`**, so which agents are
      up, and their versions, is asked with `$SRV.PING.stone-agent` -- the same
      ask-never-store shape as CONNZ, inside the tenant account. Its role needs
      `$SRV.>` on subscribe (the seeded `device` and `gateway` roles carry it).
      **Behind a leaf, the uplink credential needs it as well**: the leaf
      connection filters `$SRV` like any other subject, so the agent's own grant
      is not enough. `TestServiceDiscoveryCrossesALeafOnlyIfTheUplinkAllowsIt`.
    - **Tenant roles carry no `$SYS` publish deny, and adding one is a
      regression.** The account boundary already blocks the operator-wide
      endpoints, and in NATS a publish DENY beats a publish ALLOW — so a deny
      silently kills account monitoring and fails as a bare timeout.
      `internal/demoseed/contract.go`, pinned by
      `internal/health/leaf_visibility_test.go`.
    - Why the route gates on nothing, and the wire contract:
      `hooks/leaf_config_routes.go`. The generated `nats-leaf.conf` (two
      mandatory directives no string assertion can check) and the opt-in embedded
      `nats-server` live in the agent repo.

14. **Readiness & metrics** - `GET /api/ready` (unauthenticated, 503/200) and
    `GET /metrics` (Prometheus, open by default) on the Control Plane; `/ready` +
    `/metrics` on the agent. Checks in `internal/health`, exposition in
    `internal/metrics`, platform-specific parts in `hooks/observability.go` +
    `hooks/readiness.go` + `hooks/metrics.go`.
    - **A check must be answerable first-hand by the process running it.** This is
      the NATS account boundary restated: the Control Plane holds the operator and
      `$SYS` and has **no credential inside any organization's account**. Do not
      "improve" a metric by minting it one — that turns a credential issuer into a
      data-plane participant in every tenant's bus. **No per-org labels**, same
      reason.
    - **Four states, and only `fail` is unready.** `warn` is
      running-but-misconfigured; `skipped` means the check could not look, and it
      ranks **below** `ok`. An islanded edge warns rather than failing.
    - **Don't add a check that cannot fail, and don't publish a metric that cannot
      vary — but verify the claim first.** A comment asserting a limitation is not
      evidence of one; `stone_age_nats_cluster_routes` was nearly cut on one that
      was wrong.
    - **`stone_age_records{collection="things"}` counts devices CONFIGURED**, not
      online, and an alert on it can never fire. Say so in the HELP text of any
      metric mistakable for a health signal.
    - Why `nats_trust` earns the feature, why Nebula expiry is the one credential
      this process may check (and its **two** windows — see also
      `ui/src/utils/expiry.ts`), why the prober caches, and why `/metrics` is open
      by default: the files above, plus `hooks/cert_expiry.go`.

15. **Decommissioning a device** - `things.active`, owner/admin only. Enforced in
    **four** places at once because any one alone is a half-measure: the
    `authRule` blocks new logins, `hooks/active_flag.go` refreshes `tokenKey` so
    tokens already issued die immediately, and the same hook mirrors `active` onto
    the linked `nats_user` and `nebula_host`. A device's real capability is its
    credentials, not its PocketBase session.
    - **Deactivate, never delete.** Revoking a Nebula certificate requires the
      certificate to still be in the database to fingerprint it.
    - **It mirrors `active`, not `revoke`.** In pb-nats `revoke` is the
      "credentials leaked" button — it hands back a *working* replacement — and it
      is checked first, so setting both in one save silently takes the revoke path.
    - The Nebula half takes effect on redeploy, not instantly; Nebula has no CRL.
      Requires pb-nebula v0.2.0, and from v0.3.0 the blocklist is CA-scoped rather
      than network-scoped. Full reasoning: `hooks/active_flag.go`.
    - **The mirror runs one way, so the identities are guarded the other.**
      While a Thing is deactivated, `guardLinkedIdentity` refuses any save that
      leaves its NATS user or Nebula host active, and any minting trigger on them
      (`regenerate`, `revoke`, `renew`). Before it, Re-enable on the NATS user
      page or the Nebula form's Active toggle put the device back on the bus or
      the mesh under a "Deactivated" badge. pb-nats mints for an INACTIVE
      identity on `regenerate` and `revoke`, which
      `TestDeactivatedThingIdentitiesStayDown` shows by failing case by case with
      the guard removed. pb-nebula did the same on `renew` until v0.3.3, which
      now refuses it on any inactive host by itself -- so with the guard removed
      the `renew` case still passes, on the library's refusal. `renew` stays on
      the guard's list regardless: the invariant should not rest on a library
      version. It is an invariant, so a hook (the invariants-vs-permissions
      line in `AUTHORIZATION.md`), bound at negative priority to run before the libraries
      act, and LEVEL-keyed for the `Record.Original()` reason. The cascade passes
      it because it runs after the Thing's own save. Reactivating the Thing is the
      way back. Any new trigger that mints belongs on the list.

15b. **Rotating an organization's Nebula CA** - `POST /api/org/nebula-ca/rotate`,
    owner/admin, `{"step":"prepare"|"commit"|"finish"}`. A route rather than a
    rule branch for the reason `hooks/nats_account_routes.go` is: a rule cannot say
    "this one field and nothing else". **Three steps because the wait between them
    is the feature** — Nebula verification is mutual and config distribution is
    pull-based, so a single write carrying both new trust and new certificates
    splits the mesh. The tenant owns the lever because the wait belongs to whoever
    operates the devices.
    - Requires **pb-nebula v0.3.2**. Before it the rotation checks were bound to
      the record-API request hooks alone and this route calls `app.Save()`, so
      validation was skipped while the executor ran regardless.
    - `hooks/nebula_routes.go`, `hooks/nebula_rotation_test.go`, and
      `scripts/test-authz.sh` section 13b.

15c. **The `/32` certificate defect** - pb-nebula signed host certificates at
    `/32` until v0.3.0. Nebula puts a certificate's network straight onto the tun
    device and installs a link route for it, so the mask **is** the host's route to
    the overlay: a `/32` verifies, renders, handshakes — and moves no packet.
    Nothing errors.
    - **Nothing re-signs automatically, deliberately.** Re-signing moves a
      fingerprint, and a fingerprint is what `pki.blocklist` revokes, so a sweep
      would rewrite every peer config in the mesh on the strength of a dependency
      bump. `GET /api/org/nebula/cert-audit` names the affected hosts (active only);
      the fix is `renew`, one host at a time, then redeploy that host's config.
    - `hooks/nebula_routes.go`, `ui/src/utils/nebula.ts`.

15d. **Suspending an organization** - `organizations.active`, operator-only,
    and it means exactly one thing: **the organization's NATS account is
    withdrawn.** `hooks/org_active_flag.go` mirrors the flag onto
    `nats_accounts.active`, which is pb-nats's own account-scope suspend switch —
    the true→false edge schedules a DELETE of the account claim, so every
    device, edge agent and browser in the tenant disconnects at once, and
    `ReconcileAccounts` skips inactive accounts so it stays withdrawn across
    restarts. The flag enforced **nothing** before this: schema field, form
    toggle and two red/green badges, and not one line of Go, TypeScript or rule
    text reading it back. Same shape as the `nats_users.active` checkbox that
    was removed, and the same lesson — a flag nobody enforces is worse than no
    flag, because somebody trusts it during an incident.
    - **Reversible, which the device-scope flag is not.** This deletes the
      ACCOUNT claim: no revocation cutoff, no `nats_users` row touched, nothing
      re-minted, so every `creds_file` stays valid and simply has nothing to
      connect to. Contrast `hooks/active_flag.go`, where a device's revocation
      cutoff is permanent and coming back must issue a NEW credential. That
      asymmetry is why a tenant-wide suspend is safe behind a checkbox and a
      tenant-wide revoke would not be.
    - **The scope is deliberate and narrow.** It does NOT touch Nebula (a
      blocklist lands only on redeploy and undoing it rewrites every peer
      config — wrong shape for something reversible), does NOT lock the console
      (that means `organization.active = true` on eight read rules plus a hook
      for the org switcher, since a rule cannot traverse
      `@request.body.current_organization`), and does NOT kill Things' auth
      tokens. A suspended tenant still signs in and reads what it owns. Say
      both halves in any UI that surfaces this, or a reader misreads every
      other screen.
    - **Level-triggered, not edge-triggered** — see the `Record.Original()`
      bullet under **PocketBase hook mechanics** above for why, and for the failure that
      taught it. The refusal hook beside it IS edge-keyed, on purpose: a level
      check there would refuse every save of an infrastructure org that is
      already inactive, including the save that would fix it.
    - **Both infrastructure orgs are exempt.** The refusal blocks the flip;
      the mirror also skips them, which is not redundant — a database predating
      `hooks/org_active_flag.go` can already hold an operator org with `active` clear, and
      without the skip the first unrelated save after an upgrade would withdraw
      the hub account every managed tenant imports through.
    - The account inherits the org's flag at provisioning time rather than
      hardcoding `true`, because the mirror binds to update only and has nothing
      to compare against on a create.

16. **Organization code — the ecosystem's namespace root** (ADR 0002 in
    `platform-docs`). The rule is **ids for storage, codes for addressing**.
    `organizations.code` is the one *globally* unique identifier in the ecosystem;
    everything below it (`things`, `locations`, and the two type collections) is
    unique only within its org, enforced by `UNIQUE (organization, code COLLATE
    NOCASE) WHERE code != ''` partial indexes (case-folded since ADR 0003, see
    16b). Relation columns stay PocketBase ids.
    - **`code` is optional but immutable.** Mutability was the disqualifier, not
      optionality. `@request.body.code:changed = false` freezes it on
      `organizations` and the four inventory collections — but that is a rule and
      hook guard, so it binds the API and **not** a superuser editing in the
      PocketBase dashboard. Once codes are on stickers and in signed account JWTs,
      an edit is a site visit plus a re-signed export.
    - Derivation **refuses on collision rather than auto-suffixing**: an invented
      `acme-2` would be printed on labels and baked into signed JWTs before anyone
      noticed it was the wrong tenant. A leading digit is valid (`816tech`).
      `hooks/org_code.go`, and `orgCodeFor` in `bootstrap.go` for the two
      infrastructure orgs.
    - The review question this yields, and the reason it is worth keeping: not
      *"is this identifier unique"* but **"whose database does this identifier
      belong to."** `orgSlugFor` (`hooks/thing_routes.go`) was a second,
      independent instance of the same bug.

16b. **Thing and Location codes: generated, prefixed, case-folded** (ADR 0003
    in `platform-docs`, `hooks/codes.go`).
    - **A blank code is generated on create**, `PFX-XXX-XXX` under the record's
      type prefix or `XXX-XXX` without one: 30 symbols (no `0 O 1 I 2 Z`), each
      chunk at least one letter and one digit, `crypto/rand`. Random on purpose:
      a sparse space makes a typo land on "not found", where a sequential code's
      typo lands on the neighbouring device. Installer codes (`DOOR-1`) are kept
      exactly as typed. Bound to `OnRecordCreate` for the reason
      `RegisterOrgCode` is: `code` is frozen the moment it exists.
    - **`POST /api/org/things` generates its own** when the body's code is blank,
      because the synthetic email and NATS username are built from the code
      before anything is saved. It calls the same `NewUniqueCode`.
    - **There is no way to get a code before its record exists**, and no
      TypeScript generator. A client wanting a generated code leaves it blank. A
      `GET /api/codes/suggest` route existed briefly and was removed: blank-at-save
      covers the form, and a batch of labels prints from the list once the
      records exist. Do not add a client-side copy of the generator either; a code
      is frozen and printed, so a drifted copy would be wrong permanently.
    - **Uniqueness ignores case; storage does not.** Subjects use the stored
      spelling exactly. The index only stops `cam-1` existing beside `CAM-1`.
      Human lookups that should ignore case use `code:lower = {:c}` with a
      lowercased param (the Scanner's `{value:lower}` token does this).
    - **`prefix` on both type collections**, `^[A-Z]{1,4}$`, editable (it only
      affects future codes). Thing and Location prefixes are **separate sets**:
      the per-collection partial index covers its own half, and a create/update
      hook refuses a prefix the OTHER collection holds, because an index cannot
      see across tables.
    - **`type` is frozen once set** on `things` and `locations`:
      `(type = "" || @request.body.type:changed = false)`. A blank type may be set
      once. A wrong type is fixed by delete and recreate. The forms disable the
      type picker once one is set, since a changed type is a 404.
    - **The default subject is `{thing_type_code}.{thing}`**, with no location
      (`ui/src/utils/subjectResolver.ts`). `{location}` is still a supported
      variable for a prefix that opts in.

16c. **`locations.path` and the inventory feed** (ADR 0004 in `platform-docs`,
    `hooks/location_path.go`, `demo/inventory`).
    - **`path` is the codes from the root down**, `/` between and at both ends:
      `/KC/BD-3/RM-204/`. The end slashes make `/BD-3/` never match `BD-30`.
      "Everything under BD-3" is `location.path ~ '/BD-3/'` in a PocketBase
      filter and `location_path=~".*/BD-3/.*"` in PromQL.
    - **The hook is the only writer.** Every create and update recomputes it from
      the parent's stored path, and a client-sent value is overwritten. There is
      deliberately NO `@request.body.path:changed = false` term: a form loaded
      before an ancestor moved would echo a stale path and 404 on a field it
      never meant to touch.
    - **A move rewrites the subtree** in the same transaction, one UPDATE that
      swaps the old prefix. Compare with `substr(path, 1, length(:old)) = :old`,
      never `LIKE`: codes may contain `_`. The old path is read from the table,
      not `e.Record.Original()`, which is blank for a record built with
      `core.NewRecord` and saved twice.
    - **Refused:** a parent in another organization, the location itself, one of
      its descendants (the new path would start with its own), and a parent with
      no path yet.
    - **A deleted parent needs no delete hook.** PocketBase unsets the relation on
      each child with `SaveNoValidate` inside the delete's transaction, which
      fires the update hook, so each child becomes a root and takes its subtree
      with it. `TestLocationPath` pins this, since it is PocketBase's behaviour.
    - **A location with no code** (pre-ADR 0003) uses its id as its segment, so
      two code-less siblings never share a path.
    - **Register after `RegisterCodes`**, so a generated code is in place first.
    - **The feed is demo tooling, not platform code**: nats-auth-manager signs in
      as a seeded `inventory-feed@<org>.example` viewer, rule-router polls the
      standard list API every minute and fans each record out to
      `inventory.thing.<code>`, and Telegraf writes `stone_thing_info` /
      `stone_location_info`. No platform route.

## Files

- `hooks/leaf_config_routes.go` - `GET /api/me/leaf-config` (bound to `things`, no record id): everything an agent needs to stand up a NATS leaf server, including the `$SYS` account JWT the leaf's MEMORY resolver cannot fetch. The JetStream domain is computed from the Thing's code rather than stored
- `hooks/thing_routes.go` - `POST /api/org/things`: Thing + optional NATS/Nebula identity in one transaction; member-level for inventory, owner/admin for the identity half
- `hooks/location_path.go` - `locations.path`: computed on every save, subtree rewrite on a move, cycle refusal (ADR 0004)
- `hooks/codes.go` - generated Thing/Location codes and the separate type-prefix sets (ADR 0003). The one code generator; the UI has none
- `hooks/nebula_routes.go` - `POST /api/org/nebula-ca/rotate` (owner/admin, three steps) and `GET /api/org/nebula/cert-audit`. Both are routes for the same reason `nats_account_routes.go` is: a PocketBase rule cannot say "this one field and nothing else", and the audit needs a Nebula certificate parsed, which the browser cannot do
- `hooks/activity.go` - the tenant activity feed. The collection list IS the safety argument (see feature 7b); bound to the request hooks because they are the only layer carrying the actor, and best-effort because an observation must never cost the user their write
- `hooks/relation_tenancy.go` - the one hook-based enforcement in the platform: no relation may point into another organization's records. Derived from the schema rather than listing the nineteen relations, bound to the MODEL hooks, and applied to superusers too. See the invariants-vs-permissions bullet in `AUTHORIZATION.md`
- `hooks/org_provisioning.go` - every organization's NATS account and Nebula CA. Create-if-missing and bound to update as well as create, so re-saving the organization RETRIES a failed provision; failures are returned rather than logged, and returned AFTER `e.Next()` so a NATS outage cannot cost the owner their membership row
- `hooks/org_active_flag.go` - what `organizations.active` means: the tenant's NATS account is withdrawn, and nothing else. Level-triggered so a reported failure can be retried by re-saving (`Record.Original()` is the last READ, not the last write), and it refuses the flip on the operator and system orgs. The device-scope sibling is `hooks/active_flag.go`, which is edge-keyed and whose revocation is permanent — read both before changing either
- `hooks/schema_fields.go` - the fields that exist only once `schema.json` has been imported, shared by `bootstrap` (fatal) and the `schema` readiness check (a Fail). One list, because PocketBase discards a write to a missing field in silence and two copies drift
- `hooks/client_config_routes.go` - `GET /api/client-config`: deployment facts the SPA cannot be compiled with (browser-facing NATS WebSocket URLs). Authed (`users`) — there is no pre-login need, so no reason to publish the bus address
- `internal/health/` - readiness engine: check registry, background prober, and the unauthenticated NATS reachability (`DialInfo`) + operator-trust (`CheckCreds`) probes
- `internal/metrics/` - Prometheus exposition, plus the optional Bearer/Basic scrape token. The agent repo carries a copy of this and of `internal/health`: duplicated rather than extracted into a shared module, because two small copies that drift are cheaper to live with than a third repo to version, and the two processes check different things
- `hooks/observability.go` + `hooks/readiness.go` + `hooks/metrics.go` - the Control Plane's `/api/ready` + `/metrics` routes, its checks, and its collectors
- `hooks/cert_expiry.go` - the one scan behind both the `nebula_cert_expiry` check and the `stone_age_certificate*` metrics, so the two cannot disagree. **Two windows, not one**: hosts at 30 days (they renew themselves), the CA at 90 (it cannot be renewed at all, only rotated, and rotation needs months). `TestCAGetsALongerWarningWindowThanAHost` stops them collapsing back together
- `internal/health/leaf_visibility_test.go` - what a tenant can learn about its own leaf nodes, asserted against a real hub with a real leaf attached. Nothing in the platform ships a view on it — site connectivity is a dashboard widget recipe — but the recipe only works if these three nats-server behaviours hold: CONNZ names leaves by `server_name`, the **account** (not any permission we write) blocks the operator-wide `SERVER.PING.*` endpoints, and a publish DENY beats a publish ALLOW. A fourth is the agent's: `$SRV` discovery asked at the hub reaches a service behind a leaf only if the leaf's uplink credential allows it
