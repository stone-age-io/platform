# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html) — with the pre-1.0
caveat that a minor version may break something. Pin what you deploy.

History before `0.1.0` is not reconstructed here. Roughly three hundred commits
of pre-release development preceded it; `git log` is the record for that period,
and this file starts where the versioned releases do.

## [Unreleased]

### Security

- **A blank organization no longer matches a blank organization context.** Every
  inventory read rule scoped on `organization = @request.auth.current_organization`.
  Both sides are TEXT columns whose zero value is the empty string, and in
  PocketBase an empty string equals an empty string — so a record with a blank
  `organization` was readable by any authenticated caller whose own context was
  also blank. No role check was bypassed and no rule was mis-written; two
  sentinels compared equal.

  Both halves were reachable through ordinary use. `organizations.deleteRule`
  permitted `owner = @request.auth.id`, and 16 of the 18 relations pointing at
  `organizations` are non-cascade *and* non-required — PocketBase blanks those
  rather than deleting the rows — so an owner deleting their organization
  orphaned every thing, location, type, leaf node, `nats_account` and
  `nebula_ca` at `organization = ''`. On the other side,
  `hooks/membership_lifecycle.go` blanks `current_organization` deliberately
  when a membership is removed, and it is also the default for a freshly
  registered invitee before acceptance. Chained, a deleted tenant's whole
  inventory, its NATS account record and its Nebula CA certificate became
  readable by a user sitting in the blank-context state in any other tenant.

  Signed credentials were never exposed: `nats_users` and `nebula_hosts` require
  a correlated membership with an owner/admin role, and `memberships.organization`
  is required and cascade so it can never be blank. That is the row-scoping
  design holding.

  The eight affected read rules now require a non-blank context **and** a
  correlated membership in the organization being claimed — the same clause
  every *write* rule already carried, which is why only the reads were exposed.
  The `leaf_nodes` branches needed the identical treatment on
  `@request.auth.organization`, which an organization delete blanks for the same
  reason: a leaf node whose organization was deleted would otherwise have
  matched every orphaned record on the platform. Reads remain org-scoped rather
  than role-scoped, which is deliberate and unchanged.

- **`current_organization` is no longer settable at registration.**
  `users.updateRule` froze the field to organizations the caller holds a
  membership in; `users.createRule` did not mention it, so an invited registrant
  could name any organization id at signup and then read that tenant's
  inventory, because the read rules scope on exactly that field. The anonymous
  create branch cannot check membership even in principle, so it now refuses the
  field; `accept-invite` fills it in from the invite once the account exists.

- **Deleting an organization is a platform-operator action.** It was available to
  the organization's owner, which is what manufactured the orphaned records
  above. Every console route that touches this collection was already
  operator-gated, so nothing regresses.


### Fixed

- **`leaf-sync` no longer goes deaf when the local NATS server restarts.**
  `nats.Connect` was called with no reconnect options, so nats.go's defaults
  applied: 60 attempts at 2s, after which the connection is **closed
  permanently**. `Run` loops until its context is cancelled, so about two
  minutes after the local bus went away the agent became a zombie — the ticker
  kept firing, every KV write failed, the heartbeat failed, the twin relay
  failed, and nothing ever reconnected or exited for a supervisor to act on.

  This landed on the default topology (a separately supervised `nats-server`)
  and on the documented setup flow, where `leaf-sync config` writes
  `nats-leaf.conf` and the next step restarts the server that reads it. An
  islanded site is precisely when the agent has to keep trying, so the
  connection now retries indefinitely, with disconnect, reconnect and
  closed handlers so the state is visible in the log rather than silent.

  The **initial** dial still fails fast, deliberately and unchanged: without
  `--nats` the bus is a separate process that should already be running, and a
  hard error at startup is how an operator learns the creds path or URL is
  wrong. The options now live in a named function so
  `TestLocalConnectRetriesForever` can assert the invariant that was missing —
  it fails against the previous behaviour with `MaxReconnect = 60`.

- **The twin relay now retries a failed hub write instead of dropping it.** A
  failed write was logged and discarded, on the stated grounds that the key
  would be "re-offered by the next watcher restart's replay". It was not: the
  watcher runs on the *local* bucket, which does not die when the hub or the WAN
  does, and the supervisor only restarts the pump when the *watcher* fails. So a
  reported value that changed during an outage, failed its hub write, and then
  never changed again was absent from the hub **permanently and silently** — in
  the one direction the platform takes responsibility for delivering.

  Failed keys are now held and retried on a ticker. The value is re-read from
  the local bucket at retry time rather than remembered from the failed
  attempt, so a retry can never write a stale reading over a newer one; a key
  deleted locally during the outage is relayed as a delete. Holding and retrying
  is deliberately preferred over returning an error and letting the supervisor
  replay: one key the hub will never accept would otherwise tear down the
  watcher on every replay and block every other key behind it, forever.

  The existing partition test only ever covered convergence via a relay
  *restart*, which is the path that always worked. `twin_retry_test.go` drives a
  live relay against a failing destination, and waits on an observed write
  failure rather than a timer — an earlier version cleared the fault on a poll
  and passed against a deliberately broken build without the retry path running
  at all.

- **A short fetch no longer purges the edge mirror.** `syncCollection` paged
  without a sort order, so a record inserted or deleted between two page
  requests could shift the window and make the walk skip a record — and the
  deletion pass then read that record's absence as an upstream delete and
  removed it from the edge. The existing empty-fetch guard only caught a *total*
  failure; a partial walk is the dangerous case, because one record short of a
  400-record collection silently deletes one live config row.

  Two changes: `pbclient.List` now requests `sort=id`, because a paginated walk
  without a stable total order is wrong for any caller and the hazard belongs to
  pagination rather than to one use of it; and the walk compares what it
  collected against the `totalItems` it was promised, skipping the purge when it
  came up short. A record legitimately deleted mid-walk also trips this, which
  is a false positive worth having — the purge waits one cycle, which is the
  safe direction to be wrong in. Records the short walk *did* return are still
  upserted; only the deletion pass is skipped.

  Multi-page reconcile had no test at all: the existing fake always reported
  `TotalPages: 1` and its comment pointed at pbclient, which only ever parsed a
  single page envelope.

- **Decommissioning a device now closes the Nebula door too.**
  `hooks/active_flag.go` refreshed the PocketBase token key and mirrored
  `active` onto the linked `nats_user`, and touched `things.nebula_host` not at
  all. A decommissioned device therefore kept valid overlay-network membership
  until its certificate expired: the console door and the NATS door closed, the
  mesh door stayed open. The flag is now mirrored onto `nebula_hosts` as well,
  which is what pb-nebula writes into every other host's `pki.blocklist`.

  The two cascades are independent. The previous code returned early when
  `nats_user` was empty, which would have skipped the Nebula half entirely for
  any device holding only a certificate.

  Two properties of Nebula revocation are worth knowing rather than being
  surprised by. It has **no CRL**, so revocation is a fingerprint carried in
  every *peer's* config and takes effect when that config is redeployed and the
  process reloads — the platform's job ends when the material it hands out
  refuses the certificate, the same boundary as minting a NATS credential and
  not policing what connects with it. And fingerprinting a certificate requires
  the certificate to still be in the database, so **deactivate to revoke; do
  not delete**. Deleting a host leaves its certificate trusted until expiry.

  This needs a pb-nebula newer than v0.1.0. Against v0.1.0 the flag is mirrored
  correctly and no blocklist is produced, so the platform half is inert but
  harmless until the dependency is bumped.

  `CLAUDE.md` also described this hook as setting `revoke` on the linked NATS
  user. It never did, and the hook's own comment explains at length why it must
  not: pb-nats treats `revoke` as "these credentials leaked", rotating the key
  pair and handing back a *working* replacement, and it checks that flag before
  the active edge and returns early — so setting both in one save silently
  re-credentials the device you just disabled.

- **The test harness now agrees with the binary about what a write does.**
  `internal/testutil` bound 5 of the 13 hooks `main.go` registers, while its own
  comment insisted the ordering was "equivalent to main.go". The two missing
  record hooks were not neutral: `RegisterActiveFlag` forces `active = true` on
  every `things`/`leaf_nodes` create (PocketBase bools have no schema default and
  the authRule is `active = true`), so a test could create an inactive device and
  assert on it happily while the real binary overwrote the flag.

  That is precisely what had happened. `demo-seed` asked for decommissioned
  Things at create time, the harness allowed it, and `./stone-age demo-seed` on a
  real install produced **zero** inactive devices — with the fixture-count tests
  passing throughout. Binding the hook made the existing test fail immediately
  with "no inactive things — the decommissioned state is unrepresented".

  The six route registrars and `RegisterObservability` are still not bound, now
  with the reason written down: they bind only `OnServe`, and this harness never
  serves. Every hook that changes what a *write* does is bound; nothing that only
  answers HTTP is.

- **`demo-seed` produces decommissioned devices again, and revokes them
  properly.** Two bugs, one call site. Deactivation moved from create-time (where
  the hook overwrote it) to an update, which is the only edge
  `hooks/active_flag.go` triggers on. And the seeder no longer reaches into the
  `nats_users` record to set `revoke` alongside `active = false`: pb-nats checks
  `revoke` first and returns early, so the suspend branch never ran — and
  `revoke` means "these credentials leaked", rotating the key pair and writing
  back a fresh **working** creds file. Every "decommissioned" demo Thing was
  therefore holding a live NATS credential, which is the exact failure
  `active_flag.go` warns about at length.

  The record is re-read before the flip, which is load-bearing: a record created
  in memory and saved carries an empty `Original()` snapshot, so `active` reads
  false on both sides, the hook sees no edge, and nothing cascades.

  `TestDecommissionedThingsHaveTheirCredentialRevoked` now asserts the *effect*
  rather than the flag — the user's public key appearing in the owning account's
  revocation list, plus the linked Nebula host being inactive. It previously
  checked only the field the seeder had just written itself, so it could not tell
  "pb-nats suspended the user" from "pb-nats did nothing", and it called
  `t.Skip` when there were no inactive things — so during the bug it did not run
  at all.

- **`ensure` no longer treats a database error as "record not found".** A
  transient failure created a duplicate of a record that already existed; for
  `things` that means a second row with the same code, which the
  `UNIQUE (organization, code)` index then rejects on a later run — a seeder
  failing for a reason with no visible connection to the outage that caused it.

### Changed

- **`observability.addr` defaults to `127.0.0.1:9100`** instead of empty. A
  stock edge box previously served neither `/ready` nor `/metrics`, so the one
  place per-site health is actually visible was off unless someone opted in —
  which is why the `nats_local` check that would have caught the reconnect bug
  above had no consumer. Binding was already non-fatal by design, so if the
  port is taken (node_exporter's default is the same one) leaf-sync logs a
  warning and carries on syncing. Set it to `""` to serve neither.

- `scripts/test-authz.sh` covers 172 behaviours, up from 155. The new checks
  exercise the blank-context read path on both the user and leaf-node branches,
  cross-tenant *reads* (previously almost untested — the suite used the
  second-tenant token exactly once, for a write), registration-time
  `current_organization` injection, and organization deletion. They were each
  verified to fail against the pre-fix rules, not merely to pass against the
  fixed ones. The suite now also exercises an **admin** token: every
  owner/admin rule in the platform had been proven for `owner` only, so an
  allowlist that had lost its `admin` term would have passed the entire suite.

### Removed

- **`message_schemas`, and the two dead fields on `thing_types`.** The Thing Type
  "contract layer" was three collections; only two of them did anything. A
  message schema was never validated against — there is no JSON Schema
  validator anywhere in the platform, so an invalid schema document saved fine
  and silently rendered zero fields — and its only reader was the Publisher
  widget's payload form. It was mirrored into every edge node's local KV, where
  `rule-router` and `agent` have no reference to it at all.

  Gone with it: `thing_types.capabilities`, a hand-maintained union of its
  operations' capabilities that nothing sorted, filtered or derived anything
  from, and which could disagree with those operations with no warning
  (`migrations/widen_capabilities.go` exists purely to have kept its vocabulary
  in step — a data migration maintaining a cache with no readers); and
  `thing_types.nats_role`, a relation read by zero lines of Go and zero lines of
  TypeScript, added as the intended bridge from a contract to a NATS permission
  set and never wired to one.

  **`thing_type_operations` stays a collection.** Folding it into a JSON array on
  `thing_types` was considered and rejected: an operation is a fixed four-key
  shape, nothing validates a PocketBase JSON field, and adding a fifth key later
  would move schema evolution from PocketBase to a hand-written data migration.
  A fixed shape belongs in columns; a freeform document (`metadata_schema`)
  belongs in JSON. Subject resolution — the one thing here that was earning its
  keep — is untouched.

- **The PocketBase widget.** A dashboard widget that listed collection records,
  overlapping the inventory list views entirely and losing to them on
  pagination, server-side search and sortable columns. Its distinctive use in the
  config dropdown was `audit_logs`, which is operator-only, so it rendered empty
  for every tenant. 17 widget types are now 16.

### Added

- **"Infer from sample" on a type's inventory fields.** Paste one example record
  as JSON on the Thing Type or Location Type form and every key becomes a typed
  field to review. The helper came from `MessageSchemaFormView`; it was never
  specific to that collection, so it moved to `ui/utils/inferSchema.ts` and
  `MetadataSchemaCard` rather than being deleted with its old home.

- **Two tests pinning what a schema migration can and cannot do.**
  `migrations/field_removal_test.go` proves against the real importer that a
  `schema.json` re-import with `deleteMissing=false` CANNOT remove a field — it
  re-adds every live field the import did not carry by id — and that an explicit
  `Fields.RemoveById` drops both the definition and the SQLite column. Every
  `schema_update_*.go` in this repo is "re-import schema.json", so that gap
  silently made a whole class of migration a no-op.
  `migrations/drop_contract_layer_test.go` then runs the removal against a
  database restored to the OLD schema, because a fresh one imports the thinned
  `schema.json` and reaches the migration with nothing left to remove — a green
  `migrate up` proves nothing about a removal. It also pins the step ordering:
  PocketBase refuses to delete a collection while a relation still points at it.

- **`demo-seed` — a deployment worth looking at, in one command.** A fresh
  bootstrap leaves an empty console, and most of what this platform does is only
  legible once there is something in it: the location map, the floor-plan
  positioning drawer, the thing-type contract screens, the KV browser. The two
  applications built on the platform each had a seeder; the platform itself, the
  thing both depend on, did not.

  `./stone-age demo-seed --confirm` seeds three fictional tenants — Northwind
  Traders (cold chain), Ironbridge Manufacturing, Galewind Energy — each with
  mapped locations, a type taxonomy carrying JSON Schemas, versioned message
  schemas and operations, things spanning **devices, gateways, applications and
  unattended screens**, NATS roles and signed identities, a Nebula network with a
  lighthouse, edge sites, and members holding all five console roles. Default 120
  things; `--things N` to change it.

  Three properties worth stating, because each was a decision:

  - It runs **through the provisioning hooks, not around them**. An organization
    gets its NATS account and Nebula CA from `RegisterOrgProvisioning`; a leaf
    node gets its NATS user from `RegisterLeafNodeProvisioning`. A seeded
    deployment is indistinguishable from one built by hand in the console, and a
    hook that breaks breaks the seed.
  - It needs **no NATS server**. Keys are generated and JWTs signed locally; the
    claim publish queues in `nats_publish_queue` and drains whenever a server
    appears.
  - It is **idempotent**, keyed on the `code` that ADR 0002 already made unique.
    Re-running tops up rather than duplicating, a run that dies partway heals on
    the next one, and `fill` only runs on create — so hand-edits to demo data
    survive a re-seed.

  The three organization codes are the same ones the helpdesk's `seed-demo` marks
  as platform organizations, so the two demos join on `organizations.code` exactly
  as ADR 0002 intends. Two of the three are `managed`, which wires the
  `helpdesk.>` export into the operator hub remapped to `helpdesk.<code>.>` when
  an operator org exists — and warns, rather than failing, when it does not.

  The seeded read-only console roles (`viewer`, `dashboard`) are paired with a
  genuinely read-only NATS role. A console role is **not** a NATS role and nothing
  enforces the pairing, so a demo that shipped a "read-only" auditor holding
  publish `>` would teach the trap rather than the practice.

  `--confirm` is required. This ships in the binary an operator runs in
  production, and the command writes signed NATS credentials and Nebula
  certificates.

- **The stone-access estate in Northwind's inventory.** `demo-seed` now also
  writes the physical access-control hardware for the Northwind organization:
  three thing types (`access-controller`, `access-door`, `access-gate`), four
  message schemas describing what a controller actually publishes, seven
  operations, four controllers and ten doors and gates.

  Every one of those device codes is *also* a record in the
  [access-control](https://github.com/stone-age-io/access-control) repository —
  a `controllers` or `portals` row with the identical code — and the three site
  codes go the other way, from here into that app's `locations`. Two Go modules
  cannot import each other's fixtures, so the code list in
  `TestTheAccessControlEstateIsPresentAndJoinable` is the contract on this side;
  its mirror is `TestSiteCodesMatchThePlatformDemo` over there.

  The direction of each half is deliberate. A site code is minted HERE, because
  the platform is the inventory system of record for sites. A controller or
  portal code is minted THERE, because those appear in NATS subjects
  (`acc.{location}.{type}.{thing}`) and belong in subject-token form. Whichever
  app first puts an object on the wire names it; the other follows. That is why
  the access codes are lowercase where the rest of the inventory is not.

  Only the controllers carry a Nebula host. A door is I/O on a controller's
  terminal block rather than a network participant, and a test asserts both
  halves of that — a controller without a mesh certificate is a missing
  management path, a door with one misrepresents the topology.

- **`demo/rules/` — live telemetry for the seeded inventory.** `demo-seed` fills
  the console with records, which is what it should do and is not enough to make
  a dashboard move: a chart needs a stream of readings. Three
  [rule-router](https://github.com/stone-age-io) scheduler rule sets — one per
  seeded organization — publish on the exact subjects each thing type declares,
  composed as `subject_prefix` + `subject_suffix` the way the Thing Type screen
  renders them.

  **One directory per organization, run as one rule-router process each**, because
  in operator mode the NATS account *is* the tenant boundary and a connection
  authenticated into one cannot publish into another. Pointing `--rules` at the
  parent loads all three and two thirds of them fail authorization.

  Two constraints are documented in the files rather than worked around: cron
  floors at one minute, and a rule's payload is fixed, so a single rule draws a
  flat line on a chart. Each file gives its one showcase signal a handful of
  phase-shifted rules to produce a waveform, and says why.

  Each ends with a separable block writing **reported** state into the `twin` KV
  bucket — never `twin_desired`, because these rules stand in for devices and one
  writer per bucket is the whole safety property. rule-router has no KV action, so
  they publish to `$KV.{bucket}.{key}`, which is what `nats kv put` does.

  A companion set in the access-control repo drives the same Northwind account
  with real credential presentations at the ten seeded doors.

- **`internal/testutil` — a shared PocketBase test harness.** A real app against
  a throwaway data dir, with the four embedded libraries set up, every migration
  applied, and the platform's provisioning hooks bound. `SetupApp(t)` for a
  per-test app; `NewApp(dir)` for a `TestMain` that wants one app shared across a
  package's read-only tests, which is the difference between a 53-second package
  and a four-minute one.

  It pushes pb-nats's debounce (3s) and publish-queue (30s) timers past the life
  of any test. Neither is tied to the app's lifecycle, so a timer firing after
  `ResetBootstrapState` panics with a nil dereference inside
  `core.BaseApp.RecordQuery` — invisible to a long-lived server, which outlives
  every timer.

- **Nebula certificate expiry is reported without anyone logging in.** The list
  badges only helped someone already on the right screen, and a CA minted with a
  ten-year validity lapses long after everyone who knew about it stopped
  thinking about it — taking every host in the mesh with it, and taking away
  the out-of-band path you would have used to fix it.

  `GET /api/ready` gains a `nebula_cert_expiry` check, and `GET /metrics` gains
  `stone_age_certificate_expiry_seconds{kind}` plus `stone_age_certificates`,
  `_expired` and `_expiring` counts. Three decisions worth knowing: the check
  **warns and never fails**, because readiness failing means "stop sending this
  node traffic" and a lapsed *device* certificate is no reason to pull the
  console out of a load balancer; the gauge is an **absolute Unix timestamp, not
  a countdown**, so the horizon lives in the alert
  (`expiry - time() < 30 * 86400`) rather than being frozen into the exporter
  and going stale in every retained sample; and it reports the **soonest per
  kind, not one series per certificate**, because a per-host series would need an
  identifying label, which is a per-tenant device inventory on an endpoint that
  is open by default. Host certificates count `active = true` rows only.

  This is the one expiring credential the Control Plane can check first-hand —
  it signed these certificates and stores them. The check and the collector share
  one scan function, so a green tick can never sit beside a metric reporting an
  expiry.

### Fixed

- **`nats export --output` wrote nothing when combined with `--config`** (pb-nats
  bumped to v0.2.1). pb-nats registered a bool flag named `config` on the export
  subcommand, and this binary defines `--config <path>` as a persistent root flag.
  Cobra let the subcommand's local bool shadow the root's string, so

  ```
  ./stone-age --config ./config.yaml nats export -o ./nats-config
  ```

  set pb-nats's *preview* flag, swallowed the config path as a positional
  argument, printed `nats.conf` to stdout, created no directory, and exited zero.
  The `--config` global is documented on every command, so the failure was easy to
  hit and gave no sign it had happened.

  Two things follow from the fix. The preview flag is now `--nats-conf`, so any
  script using `nats export --config` needs updating — though in this binary that
  script was never doing what it looked like. And because the export resolves
  paths against its output directory, the generated config now carries **absolute**
  paths: `serve --nats` and an external `nats-server -c` no longer have to be
  started from the export directory.

  `readiness.go` and `natsd.go` both tell operators to run
  `nats export --output ./nats-config/`; that advice was correct all along and now
  works as written.

- **The managed organization's NATS export and import are read-only in the
  console.** Flagging an org `managed` provisions a `helpdesk-events` export on
  its account and a matching import on the operator hub, and `ensureManagedExports`
  reconciles them on every save of that organization. The console offered Edit
  and Delete on both, so a change to the subject was accepted, re-signed into the
  account JWT by pb-nats, and then silently reverted on the next org save;
  deleting one had it reappear.

  Both now show a **Managed** badge with View instead of Edit or Delete, and a
  banner naming the fields that actually get rewritten and pointing at the real
  control — clearing `managed` on the organization, which removes the pair
  together. Not a permission: an owner still has write access to the collection.
  The banner names `subject`, `type` and `description` rather than claiming the
  record is frozen, because `token_req`, `advertise` and `allow_trace` are
  create-only and an edit to those would persist.

- **Seeded `device` and `gateway` NATS roles could not carry the contracts their
  own thing types declare.** Four separate gaps, all invisible in the console —
  every screen renders correctly and the JWT signs cleanly; the permission is
  simply absent from it.

  - `acc.>` was on neither role, so a seeded door could not publish the decision
    its type declares and a controller could not receive the taps it exists to
    answer. The stone-access thing types were added pointing at the stock roles
    without checking that the stock roles reached `acc.`.
  - The subscribe lists carried only `cmd.>` and `config.>`, on the assumption
    that inbound traffic arrives on a command root. Nothing in the fixture works
    that way — a reefer takes its setpoint on `asset.{location}.{thing}.setpoint`,
    a line controller its mode on `line.…mode`, a turbine its curtailment on
    `turbine.…curtail` — so three types shipped unable to receive the instructions
    they declare, and the `.echo` half of each pair had nothing to echo. The two
    lists now cover the same subtrees.
  - `_INBOX.>` was on subscribe only, which is the *requester's* half of
    request/reply. Three types declaring `reply_diagnostics` could not answer one.
  - `$KV.>` was missing from `gateway`, and is **not** covered by `$JS.API.>`.
    Reading a KV bucket goes through the JetStream API, but a write is a plain
    publish to `$KV.{bucket}.{key}`. An access controller authenticating as its
    own seeded Thing booted clean, synced its entire policy graph, armed every
    portal, and then failed on the first status write with a permissions
    violation. A box that starts fine and cannot report state is worse than one
    that refuses to start.

  The first three are now checked structurally:
  `TestEveryThingTypeCanSpeakItsOwnContract` composes every operation's subject
  and asserts the type's default role permits it, in the direction the capability
  implies — `reply` needs the subject on subscribe *and* an inbox on publish, and
  `request` needs the mirror image. The fourth cannot be derived from a thing
  type, so `TestGatewayRoleCanRunAnEdgeService` names the three permissions an
  edge service needs and why each one's absence looks like someone else's bug.

  All four were found by running the thing: four access-controllers against a
  `serve --nats` deployment, each authenticating as its own seeded Thing.

- **Deep links and browser refresh worked on `/` and `/settings` only.**
  `app.use(router)` starts resolving the initial navigation the moment the router
  is installed — before `main.ts` reaches the auth hydration it deliberately
  awaits before mounting. `user` is set synchronously from the stored token, so
  `isAuthenticated` passed and nobody was sent to `/login`; `memberships` arrive
  over the network, so **every capability read false on that first pass and every
  capability-gated route redirected to the dashboard.** Clicking a sidebar link
  worked, because by then the store was populated — which is what made this
  look like anything other than a bug: the sidebar showed the link you had just
  been refused.

  Hydration is now memoized in the auth store and awaited by the guard.
  Deliberately not "call it earlier in `main.ts`": an ordering convention between
  two files is exactly what broke. The guard also carries the intended path
  through as `?redirect=`, so following a deep link while signed out now lands
  where you were going; `LoginView` accepts that value only when it is a
  relative in-app path, since it arrives in a URL someone else can send you.

## [0.4.0] - 2026-08-30

The organization code, and the two things that needed one. `organizations.code`
is now the ecosystem's single *globally* unique identifier: the managed-org
subject rewrite roots at it rather than at a PocketBase id, and QR labels print
it. Everything below an organization stays unique only within it. Codes are
immutable once set, which is the property that makes them safe to sign into an
account JWT and to put on a sticker.

Basemaps moved too, for an unrelated reason: CARTO put its raster tiles behind
an API key and is retiring them, so both themes now render OpenFreeMap vector
tiles through MapLibre GL. That requires WebGL and costs 285 kB gzipped on map
screens.

**Upgrading an existing database.** `migrate up` backfills
`organizations.code` from each organization's name, and aborts rather than
guessing when a name yields nothing valid or two names collide — an invented
code would be permanent, printed on labels and baked into signed JWTs. If it
stops, rename the organization so a valid code derives, run `migrate up` again,
and rename it back afterwards: `name` stays mutable, `code` freezes only once
it has a value. PocketBase runs the whole migration list inside one transaction,
so a refusal leaves nothing half-applied — and no `code` column to fill in by
hand in the meantime.

### Added

- **`organizations.code` — the ecosystem's namespace root.** Optional, unique
  when set (partial index), matching `^[a-z0-9][a-z0-9-]{1,30}$`, derived from the
  organization name on create when one isn't supplied. It is the single
  *globally* unique identifier in the ecosystem; everything below it is unique
  only within its organization, which the existing
  `UNIQUE (organization, code) WHERE code != ''` indexes already enforce. The
  rule is **ids for storage, codes for addressing** — relation columns are
  untouched and stay PocketBase ids. Rationale and rejected alternatives: ADR
  0002 in `platform-docs`.

  Derivation **refuses on collision instead of auto-suffixing**. An invented
  `acme-2` would be printed onto labels and baked into a signed account JWT
  before anyone noticed it named the wrong tenant, so the operator is asked to
  choose. Bootstrap reserves `system` and `operator`, matching the existing
  `is_system_org` / `is_operator_org` special cases.

- **QR labels for things and locations.** Printable, operator-branded, from the
  record detail views. The payload is the **bare code** — no host, no
  organization, no kind token — because a sticker in a public hallway is
  something a stranger can replace, and a URL payload would let a forged label
  send a person to arbitrary content. Sized in millimetres to real stock
  (2″ × 1″ and 4″ × 2″), both reserving the centred RFID inlay keep-out so one
  layout prints on plain or RFID media. The existing scanner widget reads them
  with no changes, since a bare code was always what its `{value}` placeholder
  expected.

### Changed

- **Basemaps moved to OpenFreeMap, and now render as vector tiles.** CARTO put
  its raster basemaps behind an API key and is retiring them, so the dark
  theme's tiles began serving a watermark. Light mode had the quieter version of
  the same problem: it called `tile.openstreetmap.org` directly, which the OSMF
  tile usage policy does not permit for a product. Both themes now use
  OpenFreeMap — `bright` for light, `fiord` for dark — which requires no key,
  sets no request cap, permits commercial use, and can be self-hosted.

  OpenFreeMap publishes no raster endpoint, so this is a rendering change rather
  than a URL swap. The basemap draws through `L.maplibreGL` onto a WebGL canvas
  instead of `L.tileLayer`, and consequently **requires WebGL**. Markers,
  clustering, popups and the `CRS.Simple` floor-plan overlay are unchanged
  Leaflet on top of it, and no consuming component needed edits. The dark-mode
  brightness filter that CARTO's `dark_all` required is gone: `fiord` is a
  designed dark style, and the GL canvas lands in the same `.leaflet-tile-pane`
  the old rule targeted, so keeping it would have washed the basemap out.

  Cost: the lazily-loaded map chunk goes from 10.8 kB to 285 kB gzipped, paid on
  map screens only rather than at app load. Verified on desktop, iPad and
  iPhone, and under Chrome throttling at every speed.

  maplibre-gl is pinned to v5 — see the held-back dependencies note in
  `CLAUDE.md`. v6 loads its tile-parsing worker as a sibling file that no
  bundler emits, which renders the basemap as a flat sheet of colour with
  nothing in the console.

- **An organization code may start with a digit.** The pattern went from
  `^[a-z][a-z0-9-]{1,30}$` to `^[a-z0-9][a-z0-9-]{1,30}$`. The leading-letter
  rule blocked an operator org named `816tech` from migrating at all, and
  nothing downstream justified it — NATS subject tokens, JetStream domains
  (always prefixed `edge-`), KV bucket names and RFC 1123 hostname labels all
  permit a leading digit. RFC 952 did not, but RFC 1123 obsoleted that in 1989.

- **Bootstrap derives the system and operator org codes from their names**
  instead of pinning `system` / `operator`, which are now only the fallback for
  a name nothing valid can be derived from. The two paths that assign an org
  code — `bootstrap.go` and the backfill in `schema_update_org_code.go` — used
  to disagree, so the same deployment got a different operator org code
  depending on whether it had been installed fresh or migrated. The code is
  immutable and gets printed on labels, so that is not a difference worth
  keeping. Consequence: the two strings are no longer reserved by construction
  and an org named "Operator" can take `operator`. Nothing resolves an org by
  those values — they were written and never read — and the unique index still
  prevents two orgs sharing a code.

- **The managed-org subject rewrite roots at the organization code, not the
  PocketBase organization id.** The hub-side import is now
  `helpdesk.{code}.>`. Both directions of the boundary — inbound machine
  tickets and outbound helpdesk events — now name a tenant by the same handle,
  so a consumer can join the two without a mapping table only one database
  could produce. The security property is unchanged: the rewrite is still
  operator-signed, so the token is still unforgeable.

  Consumers were updated **before** this (readers before writers). The helpdesk
  acks an unresolved organization rather than retrying it, so switching the
  emitter first would have dropped every machine-filed ticket with nothing but
  a log line to show for it.

- **`code` is now immutable** on `organizations`, `things`, `locations`,
  `thing_types` and `location_types` (`@request.body.code:changed = false`,
  matching `leaf_nodes`, which was already frozen). Mutability — not
  optionality — was what disqualified the alternative designs. Note this binds
  the record API and **not** a superuser editing in the PocketBase dashboard.

### Fixed

- **`ensureManagedExports` now reconciles an existing import instead of only
  creating a missing one.** It previously backfilled the `organization` stamp
  and never touched `local_subject`, so a change to the routing token would
  have left the signed import pointing at the old one — and the consumer's
  `helpdesk.*.tickets.>` wildcard filter hid it, because traffic still
  *matched* the filter while never arriving. Latent before this release;
  load-bearing now that the token is derived from a code.

- **`orgSlugFor` returns `organizations.code` instead of slugifying the
  organization's name.** The name is mutable, so every route built from it
  changed silently when someone corrected a typo in a display name. Same defect
  as the subject token, found separately.

- **pb-nats upgraded to v0.2.0, which repairs operators that have no usable
  signing key.** Every account JWT is signed with the operator's most recent
  signing key, and the publisher refuses to start without one — so an operator
  whose key list is empty fails every account save and sits in bootstrap mode for
  the life of the process. It is silent while it lasts: a NATS server whose
  resolver directory is already populated keeps authenticating every client, so
  the deployment looks healthy until somebody edits an account.

  Databases created before pb-nats had signing keys are in that state. The
  migration adopts the operator's own *identity* key as signing key #1 rather
  than generating a fresh one, because a new key is absent from the operator JWT
  baked into every running server's `nats.conf` — that list is never published
  over `$SYS` — so signing accounts with it would reject them all until a
  re-export and a fleet restart. The identity key is already trusted and already
  signed those accounts, so adopting it is invisible to a running server: no
  re-export, no NATS restart, no downtime, nothing to do by hand. It is the same
  migration `nsc reissue operator --convert-to-signing-key` performs.

  Upgraded deployments now log a warning on every boot saying identity and
  signing key are the same key. That state works and needs no urgent action, but
  it blocks keeping the identity key offline and blocks
  `strict_signing_key_usage`, both of which assume the identity key signs nothing
  but the operator JWT. Separating them is a deliberate ceremony —
  `add_signing_key` on the operator, `nats export`, restart NATS, re-save every
  account, then `remove_signing_key` for the old key — not something a library
  upgrade should do behind an operator's back.

  Fresh installs are unaffected: they have always generated a distinct signing
  key, and the migration skips any operator that already has one.

- **`nats_system_operator.private_key` and `.seed` are no longer required.**
  Same trap as the legacy signing columns in 0.2.0: PocketBase validates the
  whole record on every save, so a blank required column fails the next write to
  the operator for an unrelated reason — and because pb-nats initializes from
  `OnBootstrap`, which fires for every command, an operator save it cannot
  complete takes down `migrate up` as well as `serve`, leaving no in-band way to
  relax the flag. Nothing clears these fields today; this keeps the door open.

  It has to be done here as well as in pb-nats. `nats_system_operator` is
  declared twice — pb-nats builds it, and `schema.json` carries a dump of it —
  and on a collection that already exists the schema import wins, so relaxing it
  in the library alone would be undone by the next re-import.

Two further pb-nats fixes in v0.2.0 do not affect this deployment but are worth
recording: at-rest encryption silently loaded *zero* signing keys for the
operator and every account (`nats.encryption_key` is unset here), and
`$SYS.REQ.CLAIMS.DELETE` was signed with the operator identity key, which breaks
under `strict_signing_key_usage` (not enabled here).

## [0.3.1] - 2026-08-24

The container image, which 0.3.1 exists to publish: 0.3.0 shipped its binaries
and then failed on the image, so `ghcr.io/stone-age-io/platform:0.3.0` was never
pushed and `:latest` stayed on 0.2.0. Nothing else about 0.3.0 was wrong, and its
archives are still the ones to download if you do not want the container.

### Fixed

- **The container build no longer builds the console under emulation.** The `ui`
  and Go stages now run on the builder's own architecture
  (`--platform=$BUILDPLATFORM`), with the Go stage cross-compiling via buildx's
  `TARGETOS`/`TARGETARCH` instead of running an emulated compiler. Vite output is
  byte-identical whatever the target is, and `CGO_ENABLED=0` cross-compiles for
  free, so both stages were paying QEMU for nothing: `npm ci` alone took 2m
  emulated against 12s native.

  It was also a correctness problem, not just a slow one. On the arm64 leg npm
  installed 207 packages against amd64's 208 — silently, because a platform
  binding is `optional: true` and npm treats a failed fetch as a shrug — and the
  build died four minutes later on `Cannot find module
  '../lightningcss.linux-arm64-musl.node'`, naming a file rather than the package
  that was never installed. Same base-image digest and same lockfile had built
  clean two days earlier, so it was a flake; but only the emulated leg could ever
  hit it. Now only the runtime stage is emulated, and all it does is `apk add`.

### Added

- **CI builds the container image**, amd64 only and without pushing. Nothing
  outside the release workflow built it before, so the Dockerfile's first
  exercise was always a tag push — which is why the 0.3.0 failure could not have
  been caught earlier than the release it broke. Roughly a minute, no QEMU, and
  it runs after the authorization suite so that result never waits behind it.

## [0.3.0] - 2026-08-24

Readiness and metrics endpoints on both binaries — and the schema fixes that
shipping them turned up. `GET /api/ready` immediately reported two migrations
the binary did not know, which led to diffing a running install against
`schema.json` for the first time. That found the Members and Invitations
screens returning 400 for every caller, and 25 field definitions that could
never have been applied to any database. A check earning its keep before it had
a single production alert wired to it.

**Upgrading.** The new `schema_version` check compares the `_migrations` table
against the migrations compiled into the binary, so an install carrying
automigrate files that were never committed will report unready and answer 503
on `/api/ready` the moment it starts. `Automigrate` writes those files under
`go run`, so any database that has ever been pointed at a dev build can have
them. `./stone-age migrate history-sync --dir <pb_data>`, with the server
stopped, deletes exactly the rows the check names — it derives its list from
the same place. Diff the running schema against `schema.json` before you run
it, though: the rows are the only evidence the drift existed, and once they are
gone there is nothing left to tell you what those migrations changed.

### Added

- **Readiness endpoint: `GET /api/ready`.** PocketBase's `/api/health` reports
  that the HTTP server is listening, which is true of every failure worth
  catching here. This returns 503 while the operator is unseeded, the schema was
  never imported, or — the one that is otherwise invisible — the NATS server
  does not trust this platform's operator, so every account claim it publishes
  is rejected, no organization's account ever reaches the bus, and the console
  looks fine. Warnings (at-rest encryption off, no browser-facing WebSocket URL)
  still answer 200; a probe can only restart or de-register, and neither fixes a
  note. Unauthenticated — the callers are orchestrators and uptime checkers that
  hold no session — and every check carries a `detail` plus, when unhappy, a
  suggested `fix`, with the same text in the log. Checks run on a background
  interval and the endpoint serves the cached answer, so a probe never triggers
  a NATS dial.

- **Prometheus metrics: `GET /metrics`.** The readiness checks as a state set,
  this process's HTTP traffic by matched route pattern, row counts from its own
  database, SQLite size on disk, and — only with `serve --nats` — the embedded
  bus's own counters. Open by default; `metrics.token` closes it and is accepted
  as `Authorization: Bearer` or as HTTP Basic with any username, covering every
  scraper in common use. PocketBase's own auth is deliberately not used: its
  tokens expire and no scraper has a refresh flow.

  Nothing is per-organization. This process holds the NATS operator and the
  `$SYS` account and has no credential inside any tenant's account, so it cannot
  see what is in one. `stone_age_records{collection="leaf_nodes"}` counts leaf
  nodes **configured** — it is not availability, and an alert on it can never
  fire.

- **`leaf-sync` exports the health the platform cannot see.** `/ready` and
  `/metrics` behind `observability.addr` (off by default — opening a port on an
  edge appliance should be a decision): sync-cycle freshness, per-collection
  mirrored counts, per-collection errors, whether the uplink to the hub is
  attached, JetStream bytes, and devices actually connected at the site. The
  server-derived numbers come from the leaf's loopback monitoring port, so the
  edge reads its own server without ever holding a `$SYS` credential. An
  islanded site warns rather than fails and still answers 200: local NATS keeps
  working and devices keep running against the mirrored config, which is the
  entire reason a leaf node exists.

  This is not the `leaf_status` heartbeat. That is a tenant-facing feature the
  console renders, delivered over the very link that breaks, so it goes quiet
  during exactly the outage you want detail about. These endpoints are scraped
  locally and keep answering with the WAN down.

- **`HEALTHCHECK` in the container image**, wired to `/api/ready`. It marks the
  container unhealthy rather than restarting it, which is right: most of what
  the endpoint reports is not fixed by a restart.

### Fixed

- **`schema.json` described two collections it could never actually change.**
  `nats_account_exports` and `nats_account_imports` carried hand-written field
  ids (`text_export_name`) while every running install has PocketBase's
  deterministic ones (`text1579384326`), because pb-nats creates those
  collections at bootstrap. `core.ImportCollections` keeps the LIVE field
  whenever the name matches and the id does not — `FieldsList.add` replaces the
  imported field with the existing one — so all 25 of those definitions were
  inert, and a re-import migration touching them would have logged success and
  done nothing. Harmless while they happened to agree; a silent no-op the first
  time anyone widened a field or added a select value. The ids are now aligned
  to what the install has, the same fix `e72557f` applied to `account_id`.
  These are not throwaway collections: pb-nats sets all their rules to `nil`,
  so their org-scoped owner/admin rules come from `schema.json` and are load
  bearing. Changing an id cannot lose data — PocketBase reuses the existing
  field object precisely to avoid recreating the column.

- **`schema.json` now describes pb-nats's `system_account_id`.** The library's
  own migration adds it to `nats_system_operator`; leaving it undescribed meant
  every schema comparison reported it as drift, which buries real findings in
  known noise. Same precedent as `99bf217`.

- **The Members and Invitations screens answered 400 for everyone.** Both
  sorted by `created`, and `memberships` and `invites` were the only two
  collections in `schema.json` without it — the other sixteen have carried
  `created`/`updated` from the beginning. PocketBase rejects an unknown sort
  term while *parsing* the query, before any API rule is evaluated, so the
  failure was total (superusers included) and the response said only
  "Something went wrong while processing your request." Fixed by adding the
  autodate pair to both collections rather than by dropping the sort, which
  also leaves the database able to answer when a member joined and when an
  invite was sent — it could not before.

  The bug was latent from the first commit and only became total in the
  release: before it, MembersView's loader sent `sort: role` and only the
  pager buttons sent `role,-created`, so page one worked and page two 400'd,
  which is invisible on a member list under twenty rows. Consolidating both
  onto one options object — correct, and necessary so page two keeps the
  search filter — moved the bad term onto first paint. That change verified
  every generated *filter* field against `schema.json`; sort terms were not
  checked, and are now, across all 47 in the console.

  Existing rows keep an empty `created` and therefore sort last, deliberately.
  An autodate only fills on write and nothing in the database records when
  those rows were made — PocketBase ids are random, not time-ordered — so a
  backfill would be an invented date, which downstream readers cannot tell
  from a real one.

- **`internal/natsd` claimed clustering the Control Plane was "deliberately not
  supported". It was never true.** `Start` hands the parsed config straight to
  `nats-server` — that is the package's stated design, "no embedded-only mode,
  no options derived in Go" — so a `cluster` block has always been honoured like
  any other directive, and `serve --nats` can be one node of a cluster whose
  other nodes are plain `nats-server` processes. The comment contradicted its
  own package doc three paragraphs above it. `TestEmbeddedServerClustersWithAPeer`
  now pins the behaviour with two real peered servers, and
  `stone_age_nats_cluster_routes` reports the peers. The real cost of the
  topology is unchanged and now stated where the claim used to be: the bus dies
  with the Control Plane process, so restarting the platform takes a cluster
  node with it.

### Changed

- `github.com/prometheus/client_golang` is now a direct dependency. It was
  already in the module graph via `slackhq/nebula`, so the binary does not grow.
  Preferred over hand-writing the text format for the same reason the generated
  leaf config is validated by `nats-server` itself: nothing in CI scrapes this,
  so a malformed exposition would look fine in a terminal and be silently
  unusable. The tests parse it with Prometheus's own parser and run promlint.

## [0.2.0] - 2026-08-22

Two fixes for installs that had not gone wrong yet, and real versions for the
four pb-* libraries this binary is built from.

### Fixed

- **A fresh install could not repair itself.**
  `nats_system_operator.signing_public_key`, `signing_private_key` and
  `signing_seed` were declared required but are never written. pb-nats moved to
  a list in `signing_keys` / `signing_keys_private` and stopped declaring the
  scalars; they survived here only because `schema.json` is a dump taken while it
  still did. pb-nats creates the operator record *before* this schema is
  imported, so on a fresh install the three columns arrive empty and required,
  and the next save of that record fails with `signing_private_key: cannot be
  blank`. Nothing re-saves the operator in normal operation — every path that
  does is a repair (resolving the system account, regenerating the operator JWT)
  — so the trap was armed for the moment something had already gone wrong. It was
  also unrecoverable in place: pb-nats initializes from `OnBootstrap`, which
  fires for *every* command, so an operator save it cannot complete takes down
  `migrate up` as well as `serve`, leaving the binary unable to reach the
  migration that would relax the flag.
  `migrations/schema_update_operator_legacy_signing_optional.go` makes the three
  optional. This protects installs going forward; a database already in that
  state stays stuck.
- **A Control Plane that never left bootstrap mode.** An operator collection
  created before `system_account_id` existed never acquired the field, and
  PocketBase discards writes to undeclared fields silently — so pb-nats's own
  repair path saved the id on every boot while persisting nothing. The symptom is
  a process that logs queued publish operations every 30s while making zero
  connection attempts. Nothing looks wrong until the NATS resolver directory is
  empty — a rebuilt server, or a restore onto a new host — at which point no
  account JWT is ever published and every client fails with `fetching jwt timed
  out`, because `ReconcileAccounts` is what repopulates the resolver on boot and
  is exactly what cannot run. Fixed by the pb-nats migration that adds the field.

### Added

- `--version` now names the pb-* libraries as well as PocketBase. That is where
  NATS credential minting, Nebula CA issuance, the tenancy collections and the
  audit trail actually live, so "which pb-nats is this" is the second fact any
  credential or authorization bug report needs. A Go library cannot be stamped
  with ldflags, so these are read from the build info — which is why the tags
  below matter:

  ```
  stone-age version v0.2.0
    pocketbase  v0.39.11
    pb-nats     v0.1.0
    pb-nebula   v0.1.0
    pb-tenancy  v0.1.0
    pb-audit    v0.1.0
  ```

### Changed

- **The pb-* libraries are pinned to `v0.1.0` instead of pseudo-versions.** All
  four are now tagged and released, so `go.mod` names releases rather than
  commits and this binary can be upgraded deliberately. No behaviour comes with
  the change: diffing each tag against the pseudo-version it replaced, the only
  non-tooling commits are two `gofmt` passes.
- Each of the four libraries also gained a release pipeline and, where it was
  missing, CI — so a future bump refers to something a reader can find.

## [0.1.0] - 2026-08-21

First tagged release. The platform has been in development and in use for
several months; this tag marks the point at which it is packaged for other
people to run, not the point at which it started working.

### Added

Summarising the state at first tag rather than the path to it:

- **Control plane for NATS and Nebula.** Single Go binary extending PocketBase,
  with the compiled Vue console embedded in it. SQLite for storage, via a pure-Go
  driver, so cross-compilation needs no C toolchain.
- **Multi-tenancy and roles.** Organizations, memberships, invitations, and five
  roles (`owner`, `admin`, `member`, `viewer`, `dashboard`) enforced entirely by
  PocketBase API rules in `schema.json`.
- **NATS operator-mode management.** Operator, account and user JWT provisioning;
  per-organization accounts; roles as permission templates; self-service
  credential rotation; account signing-key management; revocation that actually
  disconnects.
- **Nebula overlay management.** Certificate authorities, networks, lighthouses
  and host certificates, issued per organization.
- **Inventory as identity.** A Thing is simultaneously an inventory record, a
  login, a NATS identity and a mesh node. Deactivating one takes all four away in
  a single operation.
- **Edge nodes.** A `leaf_nodes` identity, the `leaf-sync` agent that mirrors an
  organization's configuration into a NATS leaf node's local JetStream KV, an
  optional in-process leaf node (`leaf-sync run --nats`), and liveness
  heartbeats surfaced in the console.
- **Digital twin.** Two KV buckets per organization with one writer each —
  `twin` for reported state flowing edge-to-hub, `twin_desired` for desired state
  flowing hub-to-edge — with drift shown as the values themselves rather than a
  status word.
- **Dashboards.** Grid-based dashboards with 17 widget types, three data-source
  kinds (subscription, consumer, KV), variable substitution, and a live NATS
  WebSocket connection from the browser.
- **Audit logging** across the tenancy, NATS and Nebula collections, with a
  searchable viewer.
- **Operator branding overlay**, so a deployment can be rebranded without
  rebuilding the frontend.

Added while preparing this tag, and therefore part of it:

- **`UNIQUE (organization, code)`** on `things`, `locations`, `thing_types`,
  `location_types` and `leaf_nodes`. `code` was documented as unique and used as
  one — as a NATS KV key at the edge, as a digital-twin key prefix, as the
  argument to `stone thing get` — while nothing enforced it. The migration sweeps
  for existing duplicates first and refuses with them listed rather than dying on
  a bare "UNIQUE constraint failed". The index is partial, so blank codes are
  still allowed, and it is scoped per organization, because two tenants both
  calling a site "HQ" is the tenancy model working.
- `--version` on the `stone-age` binary, reporting the stamped build version and
  the PocketBase module version it was built against. Both binaries now share one
  version variable (`internal/version.Version`), so one `-ldflags -X` stamps both.
- A CI workflow that builds the console, builds and tests the Go side, and runs
  the authorization suite on every pull request. It also asserts the deliberately
  pinned dependency majors (Tailwind 3, daisyUI 4, TypeScript 6), which had no
  automated guard at all — there is no frontend test runner, so `vue-tsc && vite
  build` stays green while the console renders wrong.
- A Dockerfile and a first-boot entrypoint: one container that seeds itself and
  runs the NATS server in the same process. No compose file, because `serve
  --nats` is why the flag exists.
- A goreleaser config and a release workflow: cross-compiled binaries for both
  commands on linux, darwin and windows (amd64 and arm64), plus a multi-arch
  image on GHCR. `modernc.org/sqlite` is pure Go, so none of this needs a C
  toolchain.
- MIT license, a security policy, and this changelog.

### Fixed

Found by reading the codebase against its own documentation before making the
repository public. Each of these was reproduced before being fixed.

- **A user removed from an organization kept reading that organization's
  inventory.** Every read rule on the inventory collections is
  `organization = @request.auth.current_organization` with no membership branch,
  and nothing cleared `current_organization` when the membership behind it was
  deleted. Their writes stopped (every write branch names a role) but their reads
  did not, and because `users` has no `authRule` they could log out and back in
  and land in the same organization again. `hooks/membership_lifecycle.go` now
  clears the context as part of the membership delete, and
  `scripts/test-authz.sh` reproduces the original exposure.
- **Search looked at the page you were on, not the collection.** Sixteen list
  views filtered the twenty records already on screen, so a match on page three
  answered "No results found". Worst in the audit log, where a false negative has
  consequences. All sixteen now filter server-side, debounced, resetting to page
  one, with the filter reaching the pager buttons too.
- **`userRole` defaulted to `member` when no membership resolved**, which handed
  member capabilities to someone with no organization context and made the
  console offer controls the API refused. It is `null` now.
- **`nats_username` uniqueness was checked globally**, so the second tenant who
  wanted `gateway-01` was refused. A NATS user name only has to be unique within
  its account.
- An organization-name slug collision ("Acme Inc" and "Acme, Inc." both shorten
  to `acme-inc`) surfaced as a generic "failed to create thing". It now says what
  happened and what to do about it.
- `leaf_nodes.code` is frozen after creation, matching how `organization` already
  was — it is the JetStream domain suffix and the KV key prefix the edge has
  already written under, so changing it centrally orphaned the site silently. The
  docs already claimed it was immutable.
- `leaf-sync` now logs when it falls back to keying a record by id, naming the
  duplicated handle. The fallback keeps the mirror correct, which is exactly why
  the underlying data problem was invisible.

### Changed

- `pb_public/index.html` is now tracked as a placeholder, so `go build`,
  `go vet` and `go test` work on a fresh clone without installing Node and
  building the frontend first. `npm run build` overwrites it.
- The README leads with the install rather than the architecture. Measured on a
  clean clone: about two minutes of machine time from `git clone` to a serving
  console with the bus up.
- `scripts/test-authz.sh` grew from 135 to 147 checks, covering the membership
  lifecycle, the code uniqueness constraint, and the frozen leaf-node code.

[Unreleased]: https://github.com/stone-age-io/platform/compare/v0.4.0...HEAD
[0.4.0]: https://github.com/stone-age-io/platform/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/stone-age-io/platform/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/stone-age-io/platform/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/stone-age-io/platform/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/stone-age-io/platform/releases/tag/v0.1.0
