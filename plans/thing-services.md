# Plan: a Thing's NATS service on its page

Status: **not started.** Two PRs. PR 1 adds one nullable text field to
`thing_types` and a migration that re-imports `schema.json`. PR 2 changes no
schema. Neither changes an API rule, so `scripts/test-authz.sh` gets no new
check. If either starts to need a rule change, stop and re-read
**Authorization** below, because that is the decision being changed.

## The problem

The edge agent registers as a NATS micro service, `stone-agent`, and puts its
Thing's code in the service metadata (`metadata.code`,
`agent/internal/nats/handlers.go`). The console can see it: the Services page
(`/nats/services`) lists every instance answering in the org. But nothing
connects the two halves. The Thing page does not say whether its agent is
answering, which version it runs, or what it can be asked. The Services page
shows `code` as plain text with no way back to the Thing.

Anyone can write their own micro service, and it should get the same treatment
as the agent without platform code that knows its name.

## What this is not

- **Not "is this Thing connected?"** A plain NATS client (a sensor that
  publishes and never registers a service) is a different question with a
  different mechanism: account CONNZ (`$SYS.REQ.ACCOUNT.PING.CONNZ`). The hub's
  CONNZ cannot see a device behind a site's leaf node, because that device is
  connected to the leaf's own server. For devices that report state, Live
  State's timestamps already answer most of it. A separate plan if anyone asks.
- **Not monitoring or alerting.** "Tell me when the agent at GW-KC-01 stops
  answering" is a rule-router rule, not a console feature.
- **Not a store of liveness.** Nothing here writes to PocketBase.

## Decisions already made

These are settled. Do not re-open them while implementing.

- **The join is `metadata.code`, a Thing's code, compared ignoring case.**
  `$SRV` requests stay inside the org's NATS account, and a code is unique in
  its org, so the code alone identifies the Thing. It is the same key the twin
  (`thing.<code>.*`), the JetStream domain and the Thing's NATS username already
  use: ids for storage, codes for addressing (ADR 0002). Case is ignored
  because code uniqueness ignores case.

- **The Thing Type declares the service: `thing_types.service_name`.** NATS
  cannot tell "never runs a service" from "its service is down"; silence is
  the same. Only a declaration makes silence mean something, and every Thing
  of a type runs the same software, so the declaration belongs on the type.
  It also means most of the inventory (types that declare nothing) costs
  nothing: no request is sent for them. "Does it describe the contract between
  a Thing and the bus?" (`thing-types.md` §8): yes, "things of this kind answer
  as micro service X" is contract.

- **One name per type, not a list.** A Thing is one NATS identity. A second
  program on the same box with its own job should be its own Thing with its
  own credential (`thing-types.md` §1 already says a Thing can be a service or
  an application). A micro service can carry many endpoints under one name
  using groups. If a list is ever needed, text to JSON is a small migration.

- **Named `service_name`, not `nats_service` or `services`.** Every `nats_*`
  name here is a collection, and a text field with that prefix reads like a
  relation. The agent already uses "services" for operating-system services
  (`cmd.service`, `tasks.service_check`). The form label is "NATS service".

- **No checkbox.** A blank field means no service. A checkbox would be a second
  control that can disagree with the field, the same reason the console has no
  JetStream domain setting.

- **Asked, never stored. No polling.** The Thing page asks once when it opens
  and offers "Ask again", like the Services page. Polling would turn every open
  Thing page into a broadcast to every instance of the service.

- **Ask by name: `$SRV.PING.<service_name>`.** Not `$SRV.PING` (every service
  in the org). The micro protocol has no way to ask one Thing's instance
  directly: its subjects are built from the name and an instance id that
  nats.go generates on every start (`nuid.Next()`) and that cannot be set.
  So every instance of the name answers, and the page keeps the ones whose code
  matches. For the agent that is every gateway in the org; see **Scale**.

- **Say "answering", never "online" or "offline".** A device can be connected
  without running a service, and silence is not proof that anything is down.
  Same discipline as the twin's "differs, never pending".

- **No warning for several instances under one code.** A generic service may
  run replicas behind a queue group, which is micro's default. Show "2
  instances". (The agent's duplicate-code misconfiguration still shows that
  way.)

- **The service's endpoints are shown, never compared with the type's
  operations.** The type's operations (the Subjects card) are what you
  declared; the endpoints are what the service registered. They are
  independent: the agent's endpoints are `agents.<code>.cmd.*`, from its own
  `subject_prefix` setting (default `agents`), whatever the gateway type's
  prefix says. Comparing them is a drift engine nobody asked for.

- **Nothing in the console special-cases `stone-agent`.** The agent is one
  declared service among any number. A third-party service gets exactly what
  the agent gets.

- **Send is for owner, admin and member, through a new `callServices`
  capability. The NATS role is the boundary.** See **Authorization**.

- **The console keeps no record of bus actions.** A request goes from the
  browser straight to NATS. It never passes through PocketBase, and the server
  has no credential in an org's account to observe it with. The service's own
  log is the record. Say so in the docs.

## The field

`thing_types.service_name`: text, optional, pattern `^[A-Za-z0-9_-]*$`, max
64. The pattern is micro's own name rule (`nameRegexp` in nats.go's `micro`)
with a blank allowed, so PocketBase enforces it and no hook is needed.

It is a service **name**, not a subject. It becomes one token of
`$SRV.PING.<name>`, and the pattern keeps that token safe.

- Editable at any time, unlike `subject_prefix`: it moves no subject and grants
  no permission. Changing it changes which name Thing pages ask about.
- Not a gateway marker. "Nothing marks a Thing as a gateway" still holds;
  nothing outside this feature reads the field.
- The `thing_types` rules name no fields beyond `organization` and `code`, so
  the field is covered by the existing owner/admin write rule and the
  org-scoped read rule as it stands. No rule change, no backfill.

On the Thing Type form, the input offers suggestions: when it first gets focus,
the form asks `$SRV.PING` once and lists the names answering now, with an
instance count each. Picking one rules out the most likely mistake, a typo.
The form is owner/admin only, and their NATS role normally allows the ask; if
it does not, the input works without suggestions.

## How a Thing gets its service

The service can be written at any time, but it cannot run as the Thing until
the Thing exists: the Thing's code is generated on create (there is no way to
get a code before its record exists), and its `.creds` file comes from the
Thing's NATS user.

1. On the Thing Type, set `service_name` (or later; it stays editable).
2. Create the Thing with a NATS identity. That gives the code and the `.creds`.
3. Run the service with those credentials, `Name` set to the type's service
   name and `Metadata["code"]` set to the Thing's code. Its NATS role must
   allow subscribe on `$SRV.>` and publish on `_INBOX.>` (micro answers to the
   asker's inbox), plus its own endpoint subjects.
4. The Thing page shows it.

## The Thing page

A line in the Connectivity card, under Nebula. One condition decides whether
anything is asked: **the Thing's type names a service.** No `nats_user` check
and no `active` check: the states below already explain an unprovisioned or
deactivated Thing, and a service running on a shared credential still shows.

The request is `$SRV.PING.<name>` with the `timer` strategy and a 2-second
wait. Each matching reply is shown as it arrives, so an answering service
appears as fast as it replies, and silence is declared only when the wait ends.
**Not `stall`**, which `collect()` in `utils/natsServices.ts` uses: it ends the
request 300 ms after the previous reply, so a slow site's answer would be
dropped and the page would wrongly say nothing answered. Do not stop at the
first match either: several instances may answer as one code.

One pure function decides the state: `serviceStatus(replies, code)` returns
`{ mine, others }` (`mine` the instances whose `metadata.code` matches,
ignoring case; `others` how many instances answered as anything else).

| Condition | Shown |
|---|---|
| The type has no `service_name` | nothing, and nothing is asked |
| Not connected to NATS | nothing (Live State's hint already covers it) |
| The user's NATS role cannot publish `$SRV.PING.>` | nothing |
| Asking | *Checking camera-ctl…* |
| At least one instance answers as this code | a row per instance: green dot, name, version, "answering"; **Details** opens the panel |
| None as this code, some for other Things | *camera-ctl isn't answering for this Thing. It answers for 12 others.* |
| None at all | *Nothing named camera-ctl is answering in this organization.* |

- Both silent states end with "asked 2s ago · Ask again".
- The two silent states are the diagnosis. "Answers for others" points at this
  instance being down or its `metadata.code` being wrong. "Nothing at all"
  points at a typo in the name, a service not deployed, or a NATS role without
  `$SRV.>`.
- Owners and admins also see the contract under a silent state: *Register a
  NATS micro service named `camera-ctl` with metadata `code: CA-9KD-4PX`, under
  a NATS role that can subscribe to `$SRV.>`.* Behind a leaf, the site's
  uplink credential needs `$SRV.>` too
  (`TestServiceDiscoveryCrossesALeafOnlyIfTheUplinkAllowsIt`); the tooltip
  says so.
- A denied request is hidden, not explained. The Services page already
  explains roles to admins, and a viewer does not need to hear about them.

## The instance panel

The body of the instance modal in `ServiceDetailView.vue` (version, id,
started and uptime, metadata, endpoints with subject, queue group, requests,
errors, average time) moves into `components/nats/ServiceInstancePanel.vue`.
Add each endpoint's own metadata, which INFO already carries.

- **The Services page** keeps its `<dialog>` and puts the panel in it, passing
  the instance it already has.
- **The Thing page** opens the same panel after one targeted request each to
  `$SRV.INFO.<name>.<id>` and `$SRV.STATS.<name>.<id>`, using the id the PING
  returned. One reply each, not one per instance in the org. If the instance
  has restarted under a new id, the answer is no responders: *This instance is
  no longer answering. Ask again.*
- Validate the id before it goes into a subject (`^[A-Za-z0-9]+$`, what nuid
  produces), as `discoverService` already validates the name.

## Send (PR 2)

A Send box on each endpoint in the panel, shown when `can.callServices`.

- **Subject.** The endpoint's advertised subject. micro accepts any subject
  matching `^[^ >]*[>]?$` (`subjectRegexp` in nats.go), so `orders.*.get` and
  `logs.>` are legal endpoints. Each `*` becomes an input for exactly one
  token, and a trailing `>` an input for one or more dot-separated tokens.
  Send stays disabled until every input holds a valid value: non-empty tokens,
  no whitespace, no wildcard. NATS refuses a publish to a wildcard subject
  anyway.
- **Payload.** Sent exactly as typed, empty allowed. No JSON enforcement: the
  agent's `ping` takes nothing and its `service` takes JSON.
- **Request.** `nc.request(subject, payload, { timeout: 10000 })`.
- **Result line**, one of:
  - **OK**, with the round-trip time;
  - **the service's error**, from the `Nats-Service-Error-Code` and
    `Nats-Service-Error` headers;
  - **no responders**: *Nothing answers on this subject now.*;
  - **denied**: *Your NATS role can't publish to `agents.GW-1.cmd.exec`.*;
  - **timed out**: *No reply within 10s. The request may still have run.* A
    timeout is "unknown", not "failed", which matters for `exec` and
    `service restart`.
- **Body.** Pretty-printed if it parses as JSON, otherwise text, in a scrolling
  `<pre>` with a max height.
- **No history, no repeat button, nothing stored.** The payload stays in the
  box, so pressing Send again resends it.

A denied publish is reported on the request itself, not as a timeout. Checked
in nats-core: `MuxSubscription.handleError` matches a publish permission error
to the pending request by its subject and rejects it with
`PermissionViolationError`. Keep the request on the default mux; a `noMux`
request does not get this.

## Authorization

What the console shows, and what actually decides:

| | Console gate | What decides on the bus |
|---|---|---|
| The answering line | none beyond the Thing page (`viewInventory`) | the user's NATS role publishes `$SRV.PING.>` |
| The instance panel | same | the role publishes `$SRV.INFO.>` and `$SRV.STATS.>` |
| Send | `can.callServices`: owner, admin, member | the role publishes the endpoint's subject |
| `service_name` on a Thing Type | `manageDefinitions` (the existing form and rule) | — |
| The Services pages | `manageMessaging` (unchanged) | the role publishes `$SRV.>` |

- **Why members get Send.** The console does not limit bus controls by role
  today: the Publisher, Button, Switch and Slider widgets work for every role,
  and none of them reads `authStore.can`. A member can already point a Button
  widget at any subject their NATS role allows. Hiding Send would take away
  convenience, not power. `authorization.md` already says a login's power on
  the bus comes from its linked `nats_users` role.
- **Why viewers do not.** The role table promises a viewer "no write control
  anywhere", and the seeded `console-readonly` NATS role would refuse anyway.
  Viewers get the line and the read-only panel. `dashboard` never reaches the
  Thing page.
- **`callServices` is its own name**, with the same roles as
  `manageInventory`, for the same reason `viewActivity` is named apart from
  `viewInventory`: the reason differs. Its comment: "not a boundary: the
  caller's NATS role decides."
- **The demo seed's member role is too wide for this.**
  `natsRoleForConsoleRole` (`internal/demoseed/inventory.go`) gives owner,
  admin and member the same `console-operator` role, which publishes `>`. So
  in the demo a member could Send to every agent's `exec` (runs allowed
  commands as root on Linux and FreeBSD), `service`, `rotate_creds`, `nebula`
  and `leaf_refresh`, whose console equivalents the role table reserves for
  owners and admins. It is possible today through a widget; Send puts it one
  click from every gateway page. PR 2 adds a seeded **`console-member`** role:
  `console-operator`'s lists plus a publish deny on `agents.>`.
  - One subtree, not a list of commands, so a command the agent adds later is
    denied to members too.
  - Subscribe is untouched; members still see agent telemetry on dashboards.
  - `agents` is the agent's default `subject_prefix`. An org that changes it
    must change the deny. The role's description says so.
  - Orgs outside the demo set member NATS roles by hand. Docs: *Send for
    members is exactly as narrow as the member's NATS role. If members should
    not operate agents, deny `agents.>` on the role their memberships use.*
- **`metadata.code` is a claim, not proof.** Any service in the account can
  claim any code, and NATS does not tell the asker who sent a reply. That is
  the same trust level as everything else inside one account, but once Send
  exists, the request goes to the subjects the claimant advertises. A false
  claimant shows up as an extra instance. The protocol cannot fix this; the
  docs say it.

## Scale

Estimates: a PING reply is about 250 bytes; an agent's INFO and STATS replies
together about 2.6 KB.

| Instances of the declared service | Thing page (`$SRV.PING.<name>`) | Services page today (INFO + STATS for all) |
|---|---|---|
| 100 | ~25 KB | ~260 KB |
| 1,000 | ~250 KB | ~2.6 MB |
| 5,000 | ~1.25 MB | ~13 MB |

- Per page view, not per Thing, and only for types that declare a service.
- The panel is one reply each for INFO and STATS, whatever the count.
- The bus does not notice; each site's leaf carries one small reply per ask.
- It starts to hurt around 5–10k instances of one name in one org, or with
  many people on phones. Nothing in this plan addresses that, deliberately.
- The Services page is the heavy one, and also uses `stall`, so a site on a
  slow link can be missing from its list. If that shows up, raise `stall` to
  about 1 s. Not part of this plan.

## Traps

- **`$SRV.PING.>` does not match `$SRV.PING`.** The Thing page asks by name,
  so the read-only role needs `$SRV.PING.>`. The Thing Type form's suggestion
  ask is the bare `$SRV.PING`, which only owners and admins send.
- **The name is compared exactly; the code ignoring case.** The name goes into
  a subject, and subjects are case-sensitive.
- **Validate the name and id before building a subject**, even though the
  field has a pattern: a superuser edit or an old import is not bound by it.
- **The type is frozen on its Things, so the expectation is per type.**
  Declaring a service promises it for every Thing of the type. If only some
  cameras run it, that is two types, chosen before provisioning. Say so on the
  form's help text.
- **`/things/by-code/:code` and `things/:id`.** The two-segment static path
  ranks above `things/:id` in Vue Router, as `things/types` already does. Give
  it the Thing detail's meta (`viewInventory`) so a `dashboard` login is turned
  away. Look the code up with `code:lower = {:c}`, the parameter lowercased,
  and show "No Thing has code X" when there is none rather than redirecting.
- **stone-cli lists Thing Type fields one by one** (`cmd/entity.go`, the
  `thing_types` entity). Without a change there, a workspace `pull` leaves
  `service_name` out.
- **The root `CLAUDE.md` role table** says the browse row is the only one the
  API rules do not enforce. The Send row is a second; change the sentence.
- **Re-seeding a demo database.** Check whether `demo-seed` moves an existing
  member's NATS identity to `console-member`, or only sets the role on new
  identities. If only new, say a fresh seed is needed.

## Phases

**PR 1: declare it and show it.**

- `service_name`: `schema.json`, `migrations/schema_update_thing_type_service_name.go`
  (re-import, same shape as `schema_update_thing_type_metadata_schema.go`), the
  Thing Type form input with suggestions.
- `pingService`, `discoverInstance` and the pure `serviceStatus` in
  `utils/natsServices.ts`.
- `ThingServices.vue` in the Connectivity card, with the states above.
- `ServiceInstancePanel.vue`, read-only, used by the Services page and the
  Thing page.
- `/things/by-code/:code`, and the `code` metadata value on the Services
  detail page links to it.
- Demo seed: `service_name: "stone-agent"` on the three `edge-gateway` types;
  `$SRV.PING.>`, `$SRV.INFO.>` and `$SRV.STATS.>` on `console-readonly`.
- stone-cli: the field in the `thing_types` entity.
- Docs (below).

**PR 2: Send.**

- `callServices` in `stores/auth.ts`.
- Send in `ServiceInstancePanel.vue`, with the subject filler and the result
  line.
- `console-member` in `internal/demoseed/contract.go`, and
  `natsRoleForConsoleRole` maps `member` to it.
- The role table rows (root `CLAUDE.md`, platform-docs `authorization.md`).
- Docs (below).

**Later, only if asked.**

- Online dots on the Thing list. One `$SRV.PING.<name>` per declared name on
  the page answers every row.
- Example payloads carried in endpoint metadata, to prefill Send.
- "Is it connected?" for plain clients, through CONNZ (see **What this is
  not**).
- A list of service names per type.

## Files

PR 1, new:

- `ui/src/components/things/ThingServices.vue`
- `ui/src/components/nats/ServiceInstancePanel.vue`
- `ui/src/views/things/ThingByCodeView.vue` — the lookup and redirect
- `migrations/schema_update_thing_type_service_name.go`

PR 1, changed:

- `schema.json` — the field
- `ui/src/types/pocketbase.ts` — `service_name` on the Thing Type interface
- `ui/src/utils/natsServices.ts` — `pingService`, `discoverInstance`,
  `serviceStatus`
- `ui/src/views/things/ThingDetailView.vue` — the line in Connectivity
- `ui/src/views/things/ThingTypeFormView.vue` — the input and suggestions
- `ui/src/views/nats/ServiceDetailView.vue` — the panel in its dialog; the
  `code` link (the list page shows no metadata, only searches it)
- `ui/src/router/index.ts` — `things/by-code/:code`
- `internal/demoseed/contract.go` — the gateway types; `console-readonly`

PR 2, changed:

- `ui/src/stores/auth.ts` — `callServices`
- `ui/src/components/nats/ServiceInstancePanel.vue` — Send
- `ui/src/utils/natsServices.ts` — `fillSubject`, `describeReply`
- `internal/demoseed/contract.go` — `console-member`
- `internal/demoseed/inventory.go` — `natsRoleForConsoleRole`
- `CLAUDE.md` — the role table row and the sentence under it

Other repos:

- stone-cli `cmd/entity.go` (PR 1)
- platform-docs (both PRs, below)

## Testing

Vitest is pure logic in a node environment, and nothing renders in CI.

- `serviceStatus`: matching ignoring case, `others` counting, replies with no
  `metadata` or no `code`, replies that are not JSON.
- `fillSubject`: `*` takes one token, `>` takes one or more, refusal of empty
  tokens, whitespace and wildcards.
- `describeReply`: the two service error headers, a body that is JSON and one
  that is not.

Go:

- The migration adds `service_name` with its pattern; a value with a `.` or a
  `*` is refused on save. `TestCodeValidatorIsEnforcedOnSave`
  (`code_validators_test.go`) is the shape.
- The seed's `edge-gateway` types carry `stone-agent` (`seed_test.go`).
- PR 2: members are paired with `console-member`, and it denies publish on
  `agents.>`, beside `TestReadOnlyConsoleRolesGetAReadOnlyNatsRole`.
  `TestConsoleReadonlyRoleCannotPublishAnywhere` still passes with the three
  `$SRV` subjects added, since it only refuses `>` and `*`.
- Optional, against real servers beside
  `internal/health/leaf_visibility_test.go`: a `console-member` credential
  cannot publish `agents.X.cmd.exec` but can call a service elsewhere.

Nothing for `scripts/test-authz.sh`: no rule changes. Nothing for e2e until a
bug gets past the unit tests.

By hand, since nothing renders in CI: a gateway page with its agent running,
stopped, and with a mistyped `service_name`; the same page as a viewer; Send
as a member to an agent endpoint (refused) and to a custom service (answers).

## Docs

PR 1:

- platform-docs `thing-types.md`: the `service_name` row in §2, and a section on
  making your own service show on a Thing — the four steps, the NATS role
  requirement, the trust limit on `metadata.code`.
- platform-docs `agent.md` §E: gateway types declare `stone-agent`.
- platform-docs `console.md`: what the Thing page shows.
- Root `CLAUDE.md`: a feature line pointing at `ui/CLAUDE.md`, which gets the
  rules (asked never stored, "answering", the `timer` strategy, one name per
  type).

PR 2:

- platform-docs `authorization.md`: the Send row in the capability table, and
  the member sentence.
- "The console keeps no record of bus actions", in `authorization.md` and in
  the Send section of `console.md`.
