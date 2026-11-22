# Plan: align the apps with the platform, and give them one shared kit

Status: **all five phases done**, 2026-11-21. access-control and kiosk import
the official PocketBase v0.41.1. Decisions A to D are ADR 0008 in
platform-docs (numbers 0005 to 0007 belong to the services repo), alongside
the message-design changes and `building-an-app.md`.

Phase 2 is done: `stone-age-io/kit` is tagged `v0.1.0` and `pbx/v0.1.0`, CI green for both
modules. Where it differs from the text below: no `EnsureStream`, `EnsureKV` or
`KV` (each is one call in `nats.go/jetstream`); `natsx.Ready` holds the NATS
check instead of `health`; `pbx` adds `ServeReady` and `ServeMetrics`; and
`pbx.Branding` was kept, since helpdesk and access-control serve the overlay
identically and only kiosk differs. `pbx/v0.1.0` requires `kit v0.1.0`.

Phase 3 is done:
- **services** is on the kit (services `2a54e71`). Its readiness checks still
  run on each request, as its ADR 0005 decided, through `Registry.Run` and
  `WriteJSON` rather than the prober. `METRICS_TOKEN` is new.
- **The helpdesk contract change** is done (helpdesk `c55c57c`). The envelope is
  `version: 2`. Streams and the ingest consumer are CreateOrUpdate, so the new
  subjects need no manual step on an existing deployment. No rule-router demo
  rule published to the helpdesk, so none changed.
- **The helpdesk is on the kit** (helpdesk `0180d34`), net −795 Go lines. It
  also adopted `pbx.ServeSPA` and `pbx.Branding`, which the plan did not list.
  Its own checks live in `internal/readiness` and its streams in
  `internal/streams`. A Thing code is the new optional `nats.code`; the kit's
  `RegisterService` refuses to register without one, so discovery is skipped
  while it is empty.
- **The access-control contract change** is done (access-control `13fa863`).
  Subjects follow the ADR table, `ACC_EVENTS` is `acc.*.evt.>`, and
  `subjects.EventMsg` puts a `Nats-Msg-Id` on every event. `heartbeat` became a
  reserved keyword. Not done here: giving accessd and each controller a Thing.
  That moves to the kit change, where `natsx.RegisterService` first needs the
  code, as it did for the helpdesk. The platform demo's access Thing types
  followed (platform `25407b6`): their prefix is now `acc.{location}` and each
  operation suffix carries the channel, type and `{thing}`, because the channel
  now sits between the location and the type.
- **Access control is on the kit** (access-control `9bc06f4`). The connection,
  discovery and version are the kit's; its own `internal/natsx` keeps the
  NKey, token, password and TLS options and the KV and stream helpers, which
  the kit deliberately does not carry. A controller registers with
  `controller.code`, accessd with the new `accessd.code`. accessd serves
  `/api/ready` and `/metrics` on its PocketBase port, and its side metrics
  listener is gone; the controller keeps its own, and has no `/api/ready`,
  since it has no HTTP server of its own beyond the opt-in diagnostics page.
  `internal/health` is `internal/liveness`. Net Go lines are about even: the
  new readiness and tests offset what was deleted.
- **Kiosk is in scope, moved and done.** The repository moved to
  `stone-age-io/kiosk` (module path renamed, kiosk `c14392e`). Contract (kiosk
  `de860fc`): `event` → `evt`, and also `command` → `cmd`, which rule 2 asks
  for though the ADR's kiosk row had left it out. Kiosk events had no `ts`, so
  every payload gained one from its own time, and the punch's `occurred_at` is
  its `ts`. `Nats-Msg-Id` is `<kind>:<record id>` where a record identifies
  the event, so a `ledger.republish` is stored once. Kit (kiosk `3b10d98`):
  connection, discovery (new optional `nats.code`, falling back to
  `kiosk.code`), `/api/ready` on all three binaries, and `pbx` for email
  visibility. Kiosk's branding and SPA routes really are different and stay.
  The platform demo's kiosk operations followed. Verified on nats-server 2.14
  that a stream and its filtered consumers move to new subjects in place with
  messages stored, which every app's upgrade note relies on.

Phase 4 is done: the platform and the Agent each have
`internal/health/conform_test.go`, which drives their own prober through
not-yet-probed, ready, warning, failing, skipped-only and a panicking check,
and runs `healthtest.Conform` on each answer. The kit is a test-only import
in both; their health engines are unchanged. The Agent serves the same body
on `/ready` rather than `/api/ready`; Conform checks the body, not the path.

Phase 5 is done. stone-cli has `apply --template <dir>`: records are matched
by key in the current organization, relations hold keys, collections go in
dependency order, nothing is written back or deleted, and only definitions
(Thing Types, operations, NATS roles, Location Types) may be in a template.
access-control, kiosk, helpdesk and services each ship `contract/` with their
Thing Types, operations and one NATS role per kind of process, the roles
derived from the code (an edge controller reads ACC_POLICY and cannot write
it). All four applied to a demo organization on a throwaway platform and came
back unchanged on a second run; the kiosk and access Thing Types match the
demo seed's exactly. The roles were accepted by pb-nats on create but have not
yet run under a live process. `building-an-app.md` §9 documents it.

Writing ADR 0008 changed decision C: ADR 0007 rejected a platform client
inside each service, so every app gets its creds through creds-keeper, and the
kit has no `platform` package. ADR 0009 (2026-11-22) amended it again: the
keeper's package moved to `kit/credskeeper` and every app runs it in its own
process. It is still not a dependency of the Agent.

Adopting the keeper, app by app (ADR 0009 §2):
- **services**: the watchdog runs it when `PLATFORM_URL` is set (services
  v0.5.0).
- **helpdesk**: done (helpdesk `4c9b915`). A `platform:` block in
  `helpdesk.yaml`, the creds file outside `data_dir` (refused inside it), a
  `/creds` volume in the image. Run against a local platform from its
  `contract/`: first start, restart without the password, and a Revoke
  reaching the running process.
- **access-control**: done (access-control `1a9e180`), in accessd and the
  controller, each its own Thing, through `natsx.StartKeeper`. accessd waits
  30s for a first file then exits; a controller waits without a bound. Run
  against a local platform from its `contract/`, including the `accessd`
  role's JetStream setup and a Revoke reaching a running controller.
- **kiosk**: not yet. It needs the same few lines, including creating the
  credential directory, which the keeper leaves to the application.

It touches six repos: access-control, kiosk, helpdesk, services, platform-docs
and a new `stone-age-io/kit`. No phase changes the platform's schema, an API
rule or a stable surface (`platform-docs/docs/stability.md`).

**None of the apps is in production.** That makes the breaking changes in this
plan cheap now, and it will never be this cheap again. ADR 0003 made the same
argument for the platform's own default subject.

## The problem

helpdesk, access-control, kiosk and services connect to the platform the same
way: a NATS creds file, subjects named by organization and Thing codes, NATS
micro services, JetStream and KV, an embedded PocketBase and a Vue console.
None of them calls the platform's HTTP API except creds-keeper. Each app wrote
the plumbing again, and the apps differ from each other and from the
platform's conventions in four ways.

### 1. Duplicated plumbing

| Concept | Copies | Drift |
|---|---|---|
| Readiness and metrics contract (`GET /api/ready`, `GET /metrics`) | platform `internal/health` (1,655 lines), agent (689), helpdesk (555), services (122) | Three different Go APIs. helpdesk sends `checked` as a string and always sends `detail`. **access-control and kiosk do not serve `/api/ready` at all.** access-control's `internal/health` is a heartbeat monitor with the same name. |
| Metrics scrape token (Basic or Bearer, constant time) | platform, agent, helpdesk | Same behaviour, written twice: helpdesk parses the header itself |
| NATS connect | helpdesk `natsx` (265), access-control `natsx` (372), kiosk `internal/events` (part), services `natsconn` (178) | Only services sets `IgnoreAuthErrorAbort`. The others stop reconnecting after two auth errors, so after a Revoke a fresh creds file does not bring them back (services ADR 0007 §4). |
| `EnsureStream`, `EnsureKVBucket`, `RegisterService` | helpdesk, access-control | Different signatures. helpdesk's service carries no metadata. |
| Version string | platform, helpdesk, access-control | Three functions, three names |
| PocketBase auth fixes (`EnforceEmailVisibility`) | helpdesk, kiosk | Same hook, different collections |
| PocketBase import | helpdesk on `ganigeorgiev.com` v0.41.1 | **access-control on `github.com/pocketbase` v0.39.4, kiosk on v0.38.1**: the archived upstream, which the platform's `CLAUDE.md` forbids. 288 files between them. |

### 2. Subjects

The platform already says what a subject looks like. ADR 0002 gives the hub
form `{app}.{org}.…` and the rule that an app's inbound and outbound streams
differ at one token. ADR 0003 rule 6 gives token 0 as the kind, token 1 as the
code, and everything after as belonging to the kind.
`concepts/message-design.md` names the kinds of message: event, command,
request, liveness. Every app follows the outline, and each one spells it
differently:

| | Shape | Event token | Request | Liveness |
|---|---|---|---|---|
| Agent | `agents.{code}.{channel}.…` | `telemetry` | `agents.{code}.cmd.health` | `agents.{code}.heartbeat` |
| kiosk | `kiosk.{code}.event.{domain}.{verb}` | **`event`** | none | `kiosk.{code}.heartbeat` |
| access-control | `acc.{location}.{type}.{thing}.evt.{kind}`, and `acc.{location}.evt.fire` | `evt` | **`acc.{location}.ctrl.{code}.health`** | `acc.{location}.ctrl.{code}.heartbeat` |
| helpdesk | `helpdesk.tickets.create`, which arrives at the hub as `helpdesk.{org}.tickets.create`; out: `helpdesk.{org}.events.{type}` | **`events`** | none | none |

ADR 0002 says the stream-disjointness token "needs an owner before a third app
picks a token". Two more apps picked one since then, and each picked a
different word.

access-control puts the location in its subjects. That is deliberate and
allowed: ADR 0003 keeps `{location}` for fixed equipment and for site-scoped
NATS permissions, and a door controller needs exactly that. The real problem
is narrower. The fire alarm has a shorter scope than every other event, so the
events stream needs two filters of different length (`EventsWildcards`).

### 3. Event payloads

`message-design.md` §4 asks for an RFC 3339 UTC `ts`. The Agent and
access-control send `ts`. kiosk and helpdesk send `occurred_at`. Only the
helpdesk sets `Nats-Msg-Id` on what it publishes to a stream, so the stream
discards duplicates only there. kiosk's controller handles duplicates itself,
using a key in the payload (`source_line_id`). access-control events carry no
id at all.

### 4. Identity and inventory

- **Identity.** Every app process is configured with a creds file that someone
  made by hand. Except for creds-keeper, nothing is a platform Thing that signs
  in and fetches its own credential. Because of that, Regenerate and Revoke in
  the console don't reach the apps, and the console's Services page can't link
  an app's process to its Thing.
- **Inventory.** The helpdesk keeps its own `things`, `locations`,
  `thing_types` and `location_types`, which it describes as "a subset of the
  platform's". access-control keeps its own `locations`. kiosk keeps `kiosks`
  with a `location_code`. They join to the platform by code, and the codes are
  typed in twice. The helpdesk's data model says there can be no live sync,
  because "the platform publishes no event stream for things, and the only read
  paths are a control-plane credential or an edge KV mirror".

## What this is not

- **Not a template or starter repo.** A template is copied once and drifts
  from then on, which is how the first table above happened.
- **Not a monorepo of the apps.** It would move the repos into one place
  without making them smaller.
- **Not a framework.** The kit has no app lifecycle, no plugin registry, no
  config loader and no `kit.Run()`. Each package is a few functions an app
  calls from its own `main`.
- **Not each app's subject builders.** The kit has the grammar: the channel
  words, a token check and a way to build stream filters. `Tap`, `TicketCreate`
  and the rest are an app's contract, and they stay in the app.
- **Not a platform event stream.** The platform does not publish inventory on
  NATS. See the decision below.
- **Not shared logins.** Staff sign in to each app with that app's own
  accounts. The platform is not an OAuth2 or OIDC provider, and making it one
  would be a separate project. See *Later*.
- **Not projections or KV snapshot sync, yet.** See *Later*.
- **Not the frontends.** See *Later*.
- **Not a dependency of the platform or the agent.** See the decisions below.

## Decisions already made

These are settled. Do not re-open them while implementing.

- **The platform and the agent keep their own `health` package.** Their
  package comment records a decision: two small copies that drift are cheaper
  than a third repo, and if they need to agree, write a test on each side. That
  decision still stands for those two. It does not cover the apps, which now
  have five copies, three APIs and two missing implementations. The kit ships
  a **conformance test** (`healthtest`) that the platform and the agent can run
  without importing anything else from the kit.
- **The wire formats are the platform's.** `kit/health` comes from
  `platform/internal/health`. The event rules come from `message-design.md`.
  Where an app differs, the app changes.
- **The Control Plane stays an admin-only NATS client.** It holds no credential
  in a tenant account and publishes nothing on tenant subjects
  (`architecture.md` §2, the comment in `platform/internal/health/health.go`).
  So inventory never reaches the apps over NATS from the platform.
- **Only code that is copied in three places or more, and that implements a
  contract, goes in the kit.** "Might be useful" is not enough to get a package
  in.
- **Two Go modules in one repo**: `github.com/stone-age-io/kit` (no PocketBase)
  and `github.com/stone-age-io/kit/pbx` (PocketBase). services uses no
  PocketBase and must not get it in its `go.mod`.
- **The kit is pre-1.0 and tags `v0.x`.** Each app pins a version, and
  Dependabot opens the bumps. `kit/pbx` pins the one PocketBase version every
  app uses.

## Decisions this plan asks for

A to D are decided and recorded in ADR 0008. E waits on stone-cli.

### A. One subject grammar for applications

```
{app}.{scope}.{channel}.{rest}        inside a tenant account (relative)
{app}.{org}.{scope}.{channel}.{rest}  at the hub, after a managed-org import
```

- **`{scope}` is a fixed number of tokens for each app**, and every one is a
  code. For kiosk and the Agent it is the instance's Thing code. For
  access-control it is the location code, with the portal type and code after
  the channel (decided 2026-11-21, so the fire alarm needs no padding). The
  helpdesk has no scope inside a tenant account, and at the hub the org is
  its scope. Fixing the count is what lets one filter cover a whole channel.
- **`{channel}` is the kind of message, from a short fixed list**, taken from
  `message-design.md` §1:

  | Channel | Kind | Transport | Stored |
  |---|---|---|---|
  | `evt` | Event: something happened | Publish | Yes, the app's stream |
  | `cmd` | Command or request | Core publish, or request and reply | No |
  | `heartbeat` | Liveness | Core publish | Never |
  | `telemetry` | Measurement | Publish | Yes |

  An app may add its own channel for raw input (access-control's `tap`) as long
  as no stream filter on `evt` captures it.
- **The ADR 0002 disjointness rule becomes a consequence.** A stream captures
  `{app}.*…*.evt.>`, with one `*` per scope token, and an inbound command
  stream captures `cmd`. Two channels never overlap, so no app has to choose a
  token for it again.

The per-app renames are in phase 3. The Agent already fits and doesn't change.

### B. One event shape

- **`ts`**, RFC 3339 in UTC, in every event payload. `occurred_at` goes.
- **`Nats-Msg-Id` on every publish to a stream**, set to a stable id from the
  producer. Apps that also handle duplicates in the consumer (kiosk) keep
  doing it, because the stream's duplicate window is finite.
- **The payload names things by code** (`code`, `location`, `thing_code`), as
  `message-design.md` §3 already says. Nothing new here.
- **No envelope wrapper.** Metadata goes in headers and the facts in the body,
  so a stored event still makes sense on its own and no consumer has to unwrap
  anything. The helpdesk's `schema` and version fields stay, as the documented
  way to version a contract (§5).

### C. Every app process is a platform Thing

accessd, access-controller, the kiosk node and controller, helpdesk, the
watchdog: each one gets a Thing of its own type. creds-keeper runs beside it,
signs in as that Thing and keeps its creds file current. The app connects with
`IgnoreAuthErrorAbort()`, so the next reconnect reads the new file after a
Revoke, and it registers a NATS micro service with `metadata.code` set to that
Thing's code. ADR 0007 rejected building a platform client into each service,
and the reasons hold for the apps, so the kit does not contain one.

What it gets the operator:
- Regenerate and Revoke reach the apps.
- Deactivating the Thing stops the app's process.
- The console's Services and Thing pages show the app
  (`plans/thing-services.md` joins on `metadata.code`).
- A creds file on disk becomes the fallback for development, not the way
  production works.

### D. Reading inventory from the platform: the code join, for now

**Decided 2026-11-21: option 1.** The apps keep the code join, loaded by an
operator's export then seed. Option 3 is deferred to *Later* and needs its own
platform plan when an app needs live inventory. The rest of this section is
the reasoning, kept for when that comes up.

The apps join to the
platform by code today, and the codes are typed in twice. There are three ways
to read inventory instead, and none of them involves the platform publishing
on NATS:

| | How | Platform change | Fits |
|---|---|---|---|
| **1. Keep the code join** (today) | An operator exports and seeds. Codes are entered in both places. | None | Every app, including the helpdesk across many organizations |
| **2. A `viewer` membership for the app** | The app signs in as a platform user with the `viewer` role and polls `things`, `locations` and the type collections. | None | Single-org apps. It gives a machine a person's login, which works against "a Thing is the login". |
| **3. `GET /api/me/inventory` for a Thing** | A route returns the caller's organization's locations, things and types, as codes, names, type codes, location codes and `active`, and never a credential. Only a Thing whose type allows it may call it. | A route, a flag on `thing_types`, and checks in `test-authz.sh` | Single-org apps, once phase C has given them a Thing |

**Recommendation: option 1 now, and option 3 only when an app needs it.**
Option 3 follows the platform's existing rule for machines: a gateway gets
`GET /api/me/leaf-config` instead of a read branch in the rules. It needs its
own platform plan, because it is an authorization change. Its first user would
be access-control's `locations`.

Whichever option is used, the app's copy is a read-only cache keyed by the
platform record id and addressed by code. It is refreshed by a periodic full
pull and diff, with no realtime stream, because a full pull also sees deletes
and a few thousand rows is cheap. The app keeps its own fields (a portal's
schedule, a kiosk's catalog) in its own collections, joined by code. Rows that
exist only in the app (the helpdesk's gear that was never onboarded) have a
blank platform id and are left alone by the sync.

The helpdesk stays on option 1 under every choice. It serves many
organizations from the operator's org, and no read path crosses organizations.
The only cross-org channel is the managed-org export, and that carries events,
not inventory.

### E. An app's contract is a `stone` workspace

Each app ships a `contract/` folder: its Thing Types, operations and NATS role,
as the YAML `stone pull` writes. Onboarding an organization to an app means
applying that folder. This needs `stone apply --template` for one tenant,
which the November review already deferred as an open stone-cli item. The kit
doesn't depend on it, so it comes last.

## Phase 0: access-control and kiosk onto the official PocketBase

This doesn't depend on anything else here, and it is the most urgent item.

- Replace `github.com/pocketbase/pocketbase` and `github.com/pocketbase/dbx`
  with the `ganigeorgiev.com/pocketbase/...` paths, at pocketbase v0.41.1 and
  dbx v1.13.0 (the versions helpdesk and the platform use). Go treats the old
  path as an unrelated module, so this rewrites every import. It is not a
  version bump.
- Read the PocketBase changelog from v0.38 and v0.39 up to v0.41 for breaking
  changes before running the tests.
- One commit per repo. Done when `go build ./...` and `go test ./...` pass and
  `grep -r github.com/pocketbase` finds nothing.

## Phase 1: ADR 0008 and the docs (done)

- **`platform-docs/docs/decisions/0008-application-namespace.md`**, scoped to
  the whole ecosystem the way ADR 0002 is. It records decisions A to D as
  decided, and it gives the ADR 0002 disjointness rule its owner.
- **`concepts/message-design.md`**: the channel table, `ts` with no exceptions,
  and `Nats-Msg-Id` on every publish to a stream. Remove the "the helpdesk
  (`occurred_at`)" exception from §4.
- **`building-an-app.md`**, one owning page. It covers what message-design
  doesn't, and links to it for the rest:
  1. Identity: the app is a Thing (decision C).
  2. Codes: address by code, compare ignoring case (ADR 0002, ADR 0003).
  3. Inventory: the code join, loaded by export then seed (decision D).
  4. Services: register with `metadata.code`.
  5. JetStream and KV: when to use a stream and when a bucket, one writer per
     key.
  6. Projections: kiosk's controller and access-control's `mirror` as worked
     examples, and how they differ.
  7. The process contract: `/api/ready` and `/metrics`.
  8. The contract folder (decision E).

Done when the ADR is accepted. The link from each app's README to the page
goes in that app's contract PR in phase 3. The contract folder (item 8) waits
for phase 5, so the page doesn't describe it yet.

## Phase 2: the kit

New repo `stone-age-io/kit`, MIT, with CI running `go test ./...` and
`gofmt -l .` in both modules.

### Module `github.com/stone-age-io/kit`

**`health`**: lifted from `platform/internal/health/{health,prober}.go`.
- `State` (`ok`, `warn`, `fail`, `skipped`), `Result`, `OK`, `Warn`, `Fail`,
  `Skip`, `Func`, `Registry`, `Report`, and `Prober` with `OnRefresh`. A
  panicking check becomes a failure. Checks run concurrently under the caller's
  deadline.
- `Handler(p *Prober) http.Handler`: `GET /api/ready`, which returns `503` only
  on `fail`, and `503` before the first probe.
- `Metrics(g prometheus.Gatherer, token string) http.Handler`: the platform's
  `requireToken`.
- `NATS(nc func() *nats.Conn) Func`: connected or not.
- Not lifted: the platform's NATS and leaf checks. They ask questions only the
  Control Plane can answer.

**`health/healthtest`**: `Conform(t *testing.T, h http.Handler)` asserts the
wire format. It uses only the standard library.

**`natsx`**: lifted from `services/internal/natsconn`, plus helpers from
`access-control/internal/natsx`.
- `Connect(o Options)`. It always sets `MaxReconnects(-1)` and
  `IgnoreAuthErrorAbort()`, and logs through `log/slog`.
- `EnsureStream`, `EnsureKV`, `KV`.
- `RegisterService(nc, name, description, code string, meta map[string]string)`.
  `code` is required and goes into `metadata.code`, with the build version
  added (decision C). It works before NATS is reachable.
- `PublishEvent(ctx, js, subject, id string, v any)`: marshals `v`, sets
  `Nats-Msg-Id` to `id`, and refuses a payload with no `ts` field (decision B).

**`subjects`**: the grammar (decision A), not any app's subjects.
- The channel constants `Evt`, `Cmd`, `Heartbeat`, `Telemetry`.
- `Join(tokens ...string) (string, error)`: refuses an empty token, `.`, `*`,
  `>` and whitespace.
- `Filter(app string, scopeTokens int, channel string) string`: for example
  `acc.*.evt.>`.
- `Split(subject, app string, scopeTokens int) (scope []string, channel string, rest []string, ok bool)`.
- A test helper, `Disjoint(t, filters ...string)`, generalizing the helpdesk's
  `TestStreamsDisjoint`.

**`codes`**: `OrgPattern` (pinned by a test to `platform/hooks/org_code.go`),
`ValidOrg`, `Equal` (ignores case), and `ValidThing` (the ADR 0003 pattern).

**No `platform` package.** Identity comes from creds-keeper running beside the
app (ADR 0007, ADR 0008 rule 4). `natsx.Connect` is the only part of it that
lives in the kit. *Superseded by ADR 0009: `kit/credskeeper` is the keeper,
run in the app's process.*

**`version`**: `Build`, `Semver` and `Dependency`, all from
`debug.ReadBuildInfo`.

### Module `github.com/stone-age-io/kit/pbx`

- `EnforceEmailVisibility(app, collections ...string)`.
- `health.Func` builders: `Database`, `Schema`, `SchemaVersion`.
- `ServeSPA(se, fsys)`, which must not shadow `/api/` or `/_/`.
- `Branding(se, dir)`: compare the three apps' overlays first. If they don't
  agree on what an overlay may replace, leave branding out and write down why.
- `go.mod` pins `ganigeorgiev.com/pocketbase/pocketbase` at the version from
  phase 0.

Done when both modules are tagged `v0.1.0` and `kit/health` passes
`healthtest`.

## Phase 3: each app, two PRs

For each app, the **contract PR** makes the breaking changes from decisions A
to C and says so at the top of its CHANGELOG. The **kit PR** then adopts the
kit and deletes the app's own copies in the same change. They are separate so
that a subject regression and a plumbing regression can't hide in one diff.
The order runs from closest to the kit to furthest.

**1. services**
- Kit PR: `internal/health` and `internal/natsconn` become the kit.
  creds-keeper stays as it is.

**2. helpdesk**
- Contract PR:
  - `helpdesk.tickets.create` → `helpdesk.cmd.tickets.create`, which arrives
    at the hub as `helpdesk.{org}.cmd.tickets.create`.
  - `helpdesk.{org}.events.{type}` → `helpdesk.{org}.evt.{type}`.
  - `occurred_at` → `ts`.
  - The managed-org export maps the whole of `helpdesk.>`, so the platform
    doesn't change. The rule-router demo rules and the vendored protocol docs
    do.
- Kit PR: delete `internal/health` (keep the SMTP, inbound email and
  application URL checks as `health.Func` values in the app), `natsx`,
  `version` and `authfix`. Its `/api/ready` moves to the platform's wire
  format.

**3. access-control**
- Contract PR:
  - Every subject moves the channel to right after the location, per the
    before-and-after table in ADR 0008: `acc.{location}.{type}.{thing}.evt.tap`
    → `acc.{location}.evt.{type}.{thing}.tap`, and the same for `cmd`, `tap`
    and the controller's `heartbeat` and `health`. The fire alarm,
    `acc.{location}.evt.fire`, stays as it is. `EventsWildcards` becomes the
    single filter `acc.*.evt.>`, and the notify consumer's filters become
    `acc.*.evt.*.*.alarm`, `acc.*.evt.fire` and `acc.*.evt.ctrl.*.state`.
  - The vendored protocol doc, the demo rules and every subject test change
    with it.
  - `Nats-Msg-Id` on every event.
  - accessd and every controller get a Thing and run beside creds-keeper.
- Kit PR: adopt `natsx`, `subjects` and `version`. **Add**
  `/api/ready` and `/metrics`. Rename `internal/health` (the heartbeat monitor)
  to `internal/liveness`.

**4. kiosk** (waits on open question 1)
- Contract PR:
  - `kiosk.{code}.event.…` → `kiosk.{code}.evt.…`.
  - `occurred_at` → `ts`.
  - `Nats-Msg-Id` on every event.
  - The node and the controller get a Thing and run beside creds-keeper.
- Kit PR: adopt `natsx`, `subjects` and `authfix`, and add
  `/api/ready`.

Each kit PR is done when the app's tests pass, `healthtest.Conform` passes
against its readiness handler, `subjects.Disjoint` passes against its stream
filters, and the PR description states the line count it removes.

## Phase 4: the platform and the agent run the conformance test

One test file in each: `healthtest.Conform` against the real readiness
handler. This adds the kit as a test-only import, and nothing else changes.

## Phase 5: contract folders

Once stone-cli has `apply --template`, each app ships `contract/` (decision E),
and `building-an-app.md` documents onboarding an organization as one
`stone apply`.

## Later, with the conditions that start it

- **`GET /api/me/inventory`** (decision D, option 3, deferred on
  2026-11-21): its own platform plan, started when an app needs live inventory. access-control's `locations` is the
  likely first. A `kit/pbx/inventory` package (pull, diff and upsert into a
  read-only collection) follows the route, not before it.
- **Projections.** Extract when a third app builds a read model from a durable
  consumer in the same shape as one of the two today.
- **KV snapshot down.** Same condition.
- **Staff logins through the platform.** This needs the platform to become an
  OAuth2 or OIDC provider. Worth considering only if operators complain about
  keeping separate staff accounts in each app.
- **Frontends.** First align the versions: the PocketBase JS SDK is 0.26.8 in
  helpdesk and access-control and 0.27 elsewhere, and kiosk moved to Tailwind 4
  with no daisyUI. A shared package comes after that.
- **demoseed.** Extract only if a helper appears unchanged in three of them.

## Open questions

1. ~~**Is kiosk in scope?**~~ Yes (2026-11-21): moved to `stone-age-io/kiosk`
   and unified in phase 3.
2. **Should the platform import `kit/codes`?** It would keep the code pattern
   in one place, at the cost of a dependency on a repo the platform doesn't
   release. It can wait until after phase 2.

## Size, honestly

The kit removes roughly 2,500 to 3,500 duplicated lines from four apps and adds
about 1,500 to 2,000 in one place. It doesn't
shrink the products. What it buys:
- one readiness format
- one reconnect behaviour after a Revoke
- one PocketBase version
- one subject grammar
- every app process visible, and revocable, as a Thing

The alignment work in phases 1 and 3 is worth more than the kit itself, and
it's the part that gets more expensive once anything is in production.
