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

- **`maplibre-gl` 5.24.0 → 6.11.1, which closes GHSA-jrc7-96c5-q579.** The
  critical `DOM.sanitize()` bypass was fixed in 6.4.1 and never backported,
  so the v5 pin carried it with nothing to upgrade to. It was not reachable (its
  only sink, MapLibre's attribution control, is never constructed), but it was
  a critical line in every `npm audit`. `npm audit` now reports 0.

### Added

- **Generated Thing and Location codes** (ADR 0003 in platform-docs). A Thing
  or Location saved without a code gets one, `PFX-XXX-XXX` under its type's
  prefix or `XXX-XXX` without one, from an alphabet with `0 O 1 I 2 Z` removed.
  Random, so a mistyped code finds nothing where a sequential one finds the
  device next door. Installer codes are still accepted exactly as typed.
  `POST /api/org/things` no longer requires a code.
- **`prefix` on Thing Types and Location Types**: 1–4 capitals, copied into
  generated codes. Thing and Location prefixes are separate sets per
  organization.
- **The Scanner's filter accepts `{value:lower}`**, so
  `code:lower = "{value:lower}"` finds a code typed in the wrong case.

### Changed

- **The default subject is `{thing_type_code}.{thing}`**, without
  `{location}`. A Thing Type with an empty subject prefix used to resolve to
  `{thing_type_code}.{location}.{thing}`, so moving a Thing moved every
  subject it published on. `{location}` remains available to a prefix that
  opts in. **Breaking** for any Thing Type relying on the empty default; every
  demo type sets its prefix explicitly and is unaffected.
- **Code uniqueness ignores case** on things, locations, thing_types and
  location_types. `cam-1` and `CAM-1` can no longer both exist in one
  organization. Stored case is kept, and subjects use it exactly. The
  migration refuses, naming them, if existing codes differ only by case.
- **A Thing's or Location's type is frozen once set.** A blank type may be
  set once; a wrong one is fixed by delete and recreate. The forms disable the
  type picker once a type is set, and show the code read-only when editing.
- **The Thing form no longer derives the code from the name.** Left blank, the
  server generates it at save; the help text shows what one will look like for
  the selected type.
- **The v5 pin is gone because its reason had a fix.** v6 no longer inlines its
  tile-parsing worker, and after bundling it looks for a file the build never
  emitted, so the map drew only its background colour. `useLeafletMap.ts` now
  imports the worker with `?worker&url` and passes it to `setWorkerUrl()`. The
  suffix matters: the worker file imports `maplibre-gl-shared.mjs`, and a plain
  `?url` copies it as a lone asset whose import then fails inside the worker,
  just as quietly. Checked in a production build and in the dev server, in both
  styles (bright and fiord): water, roads, landuse and labels all render. With
  the call removed, the same page renders 0 features, so the check can fail.
- **Cost: the map payload goes from 285 kB to 416 kB gzipped** (272 kB for the
  main chunk, 144 kB for the worker). The worker bundle carries its own copy of
  MapLibre's shared code. It is still paid only on screens that show a map.
- **CI checks that the worker is emitted**, and now expects `maplibre-gl` 6.
  The major is still asserted even though it is no longer held back: a v7 that
  moves the worker again would fail the same way, with no error on the page.

## [0.8.0] - 2026-09-19

**A tenant can finally see who changed what.** `activity` is a new org-scoped
feed at `/activity` — actor, action, record, timestamp — readable by every role
in an organization. It is not the audit log, and the difference is the point:
`audit_logs` stays operator-only and carries full record snapshots, while this
carries none at all.

**It also fixes a regression that made 0.7.0 uninstallable from scratch.**
`bootstrap` died partway through on a fresh database. Existing deployments were
never affected — but if you tried 0.7.0 as a first install, this is the release
that works. See **Fixed**.

### Fixed

- **`bootstrap` could not complete on a fresh 0.7.0 install.** Tightening
  `hooks/relation_tenancy.go` in 0.7.0 so that moving a record between
  organizations re-checks the relations it keeps had a consequence nothing
  caught: adopting the pre-seeded `$SYS` records is exactly such a move, from
  blank to the System org. The NATS user was linked while its role was still
  unlinked, so the check compared a user now in the System org against a role
  that was still blank and refused it — `bootstrap` died with *"the role_id
  field must reference a record in the same organization"*, leaving an install
  with no NATS identity and no operator context.

  The guard was right; the order was wrong. Link what is pointed AT before what
  points at it: account, then role, then user.

  **Existing deployments were unaffected** — `linkSingleton` short-circuits on
  records that are already linked, so no re-check happens on an upgrade. Only a
  first-time `bootstrap` against 0.7.0 hit it.

  Nothing caught it because nothing ran the command: the Go suite never invokes
  `bootstrap`, and `scripts/test-authz.sh` stands its server up with
  `superuser upsert` + `migrate up` + `serve`. There is now a test.

### Added

- **A tenant activity feed.** `activity` answers "who on my team changed this
  device, and when" — org-scoped, readable by every role, and surfaced at
  `/activity` in the console. It records the actor, the action, the record and
  a snapshot of both labels. It records **no values at all**.

  It is deliberately not `audit_logs`, which stays operator-only. That is the
  forensic trail: full before/after snapshots, no organization column, and a
  read on it inherits the exposure of every collection in
  `auditSnapshotCollections` at once.

  **The invariant is that an entry is visible to exactly those who could read
  the record it describes.** A flat collection that mirrors other collections
  inherits none of their scoping, which is what `audit_logs` taught the hard
  way in 0.7.0. So the feed covers only the five tenant collections whose own
  reads are org-scoped with no role branch — things, locations, and the three
  type collections — and not memberships, invites, NATS roles or Nebula
  networks, whose reads stop at owner/admin. Adding to that list is an
  authorization change, and a test reads `schema.json` to say so.

  The feed is append-only by construction: all three write rules are nil, so
  nothing can forge or rewrite it through the API, and `actor` is a plain id
  rather than a relation so that deleting a user does not quietly rewrite every
  line that mentions them.

  Deliberately absent, each because it was drafted and cut for being more
  machinery than the feature earns: a changed-field list (pb-audit already
  computes one for these same writes), a retention cron (the row count is now a
  metric — measure first), and a transaction dance for the batch API (disabled
  by default; the cost if enabled is one phantom row in an observation log).

## [0.7.0] - 2026-09-19

**`audit_logs` was holding every credential this platform ever minted, in
plaintext.** Not a suspicion — creating one NATS identity wrote the full
`.creds` file, NKEY seed included, into two audit rows. This release closes
that, and the fix needs one manual step on any existing deployment (see
**Upgrading**).

The cause is two correct decisions meeting. pb-audit snapshotted
`Record.PublicExport()` into `before_changes`/`after_changes` — every field a
collection does not mark `hidden` — and this platform deliberately does not
hide `nats_users.creds_file` or `nebula_hosts.config_yaml`, because the
identity that owns them has to read them back, and **row scoping** is what
decides who sees which row. That reasoning is right for `nats_users`. It does
not transfer to `audit_logs`, which has no row scoping at all: one flat
collection holding a copy of every record. So a value protected by scoping was
not protected once it landed there — outside at-rest encryption, which covers
the `seed`/`private_key` columns rather than the credential files derived from
them, and never expiring, retention being off by default. It also outlived
rotation: the credential replaced *because* it leaked stayed in
`before_changes`.

Audit rows now record **`changed_fields`** — the names of the fields that
moved — and keep full values only for the eleven collections whose diffs a
human actually reads. "user X updated nats_users/abc123, fields: [creds_file,
jwt]" answers who, what and when without being the credential.

Also here: a cross-tenant relation could be created by moving a record's
`organization` rather than its relations, and a failure to provision an
organization's NATS account or Nebula CA was reported to nobody.

### Upgrading

**Existing audit rows still hold the credentials.** The migration adds the new
column and changes what is written from here on; it deliberately does not
delete anything. That is the audit trail, and clearing it is a decision with a
backup attached rather than something that should happen silently on deploy.

Either set `audit.retention` in `config.yaml` and let the rows age out, or,
once you have a backup and have decided the trail can lose those values:

```sql
UPDATE audit_logs SET before_changes = NULL, after_changes = NULL
 WHERE collection_name IN ('nats_users','nebula_hosts','nats_accounts',
                           'nebula_ca','invites');
```

Rotating afterwards is worth considering for anything in there. The NATS side
is cheap and central (`regenerate`, and the account JWT's revocation cutoff is
permanent). The Nebula side is not: there is no CRL, so an old `config_yaml`
holds a live host key until its certificate expires or its fingerprint reaches
every peer's `pki.blocklist`.

One behaviour change worth knowing before deploy: **creating an organization
now fails loudly if its NATS account or Nebula CA cannot be provisioned**,
where it previously returned success. If your deployment has been quietly
producing half-provisioned organizations, this is where you find out.

### Security

- **Audit snapshots no longer carry credentials.** As above. The collections
  that keep full values are listed in `auditSnapshotCollections` (`main.go`)
  and guarded by `audit_snapshots_test.go`, which reads `schema.json` and fails
  if a listed collection has an unhidden credential-bearing field — adding
  `nats_users` to that list would restore the archive in one line, with no
  visible symptom, so the guard is a test rather than a comment.

  `nats_roles` **is** on the list, deliberately: it holds the
  publish/subscribe permission templates, so its diff records someone changing
  what an identity may do on the bus, and it carries no secret.

- **A record could take its relations into another tenant.**
  `hooks/relation_tenancy.go` refuses a relation pointing into another
  organization's records, and skipped ids that had not changed on the grounds
  that they were checked once. That premise fails when the record's *own*
  `organization` moves: everything it keeps was checked against the
  organization it used to have. A `nats_users` row could be moved from org A to
  org B while keeping org A's `account_id`, and pb-nats signs the user JWT with
  whatever `account_id` names.

  Not reachable through the record API — every org-scoped update rule freezes
  `organization` — and reachable through exactly the two paths that guard binds
  model hooks to cover: a superuser editing in the PocketBase dashboard, and a
  route writing with `app.Save()`.

### Fixed

- **Organization provisioning failed silently.** A missing collection was
  skipped by an `if err == nil` with no else, and a failed save was a log line.
  Creating an organization whose NATS account could not be written returned
  success, and what you got was a tenant that can never issue a device
  credential — first noticed as a device that cannot connect, later, for no
  stated reason.

  Failures now reach the caller. The work still runs before `e.Next()` and only
  the error is deferred until after it: returning early would skip the handlers
  bound later and cost the owner their membership row, which is worse than the
  bug. Provisioning is also create-if-missing and bound to update as well as
  create, so **re-saving the organization retries it** — an error with no
  remedy is not a fix.

### Changed

- **pb-audit v0.1.0 → v0.2.1.** Adds `changed_fields`, makes value snapshots
  opt-in per collection, gives `update` success events a real before state (so
  a programmatic `app.Save()` produces a diff rather than only confirming a
  commit), and creates the audit collection with nil API rules instead of
  `@request.auth.type = 'admin'` — PocketBase v0.22 syntax that predates
  `_superusers`. That last one does not affect this platform, which defines
  `audit_logs` in its own `schema.json`.

- `audit_logs` gains a `changed_fields` column
  (`schema_update_audit_changed_fields.go`). Auth events do not carry one:
  there is no before state to compare.

- The "has `schema.json` been imported" check is now one list and one walk
  (`hooks/schema_fields.go`), shared by `bootstrap` and the `schema` readiness
  check. Both carried their own copy of an identical field list, which is the
  parallel-list failure this codebase has paid for before.

### Removed

- `internal/demoseed`'s unused `stamp` helper — the one genuinely unreachable
  function in the tree.

## [0.6.0] - 2026-09-17

**An edge site is a Thing.** The `leaf_nodes` collection, the `leaf-sync`
binary and the config mirror are gone, and the edge agent now lives in its own
repository at
[stone-age-io/agent](https://github.com/stone-age-io/agent). A site that runs a
NATS leaf node is an ordinary Thing whose agent bootstraps from one new route,
`GET /api/me/leaf-config`. That removed about 5,900 lines net and, more to the
point, removed the reason an edge identity needed read grants spread across the
inventory.

**This release is breaking twice, and both halves have an order.** Stand each
site's agent up against its Thing *before* upgrading the Control Plane — an
older `leaf-sync` keeps running on its generated `nats-leaf.conf` afterwards
but can no longer log in or re-fetch anything. And
`POST /api/tenancy/accept-invite` is now `POST /api/org/invites/accept`, which
matters to anything driving invitations outside the console.

The migration deliberately leaves each dropped leaf node's `nats_users` and
`nebula_hosts` rows **working**, and lists them on the way past. Those are live
credentials at real sites, and a migration that runs on deploy is not where a
fleet gets taken off the bus. Deactivate them from the console once each site
is running against its Thing — deactivate rather than delete, since revoking a
Nebula certificate needs the certificate in the database to fingerprint it.

Also here: pb-tenancy is absorbed into the platform, the invitation email is
editable in `/_` and six invitation bugs behind it are fixed, and a widget-form
validator that had been rejecting every NATS system subject — `$SYS.*`,
`$JS.API.*`, `$KV.*` — is corrected.

### Added

- **`GET /api/me/leaf-config`** — everything an agent needs to stand up a NATS
  leaf server, in ten named fields (`code`, `domain`, `creds`, `account_jwt`,
  `account_pub`, `operator_jwt`, `sys_account_jwt`, `sys_account_pub`,
  `hub_leaf_url`, `hub_domain`). Bound to `things` and taking no record id: the
  target is the caller's own authenticated record, like
  `POST /api/me/nats-creds/rotate`.

  There is deliberately no marker, flag or capability check on it. Everything it
  serves is either public trust material — the operator, account and `$SYS`
  account JWTs, which every server in the network validates anyway — or the
  caller's own credential, which it must already hold to connect at all. A Thing
  that will never run a leaf node can call it and learns nothing it could not
  already read. A gate would have been a permission over data that is not
  secret, and it would have needed a marker field to gate on.

  The JetStream domain is **computed from the Thing's code**, not stored. Two
  new config keys feed it: `nats.leaf_url` (where an edge box's leaf remote
  dials this hub — startup rejects anything not beginning `nats-leaf://` or
  `tls://`) and `nats.jetstream_domain` (this hub's own domain, default
  `hub`). Neither is derived from `nats.server_url`, for the same reason
  `nats.websocket_urls` is not: what this process dials says nothing about what
  an edge box can reach.

- **Site connectivity is a dashboard recipe.** Point a Publisher or Button
  widget at `$SYS.REQ.ACCOUNT.PING.CONNZ` with a `{}` payload: the reply lists
  this organization's leaf connections, each named by the leaf server's
  `server_name`, which the agent sets to the Thing's `code`. Both widgets
  already do request/reply against a free-text subject, so this needs no
  platform code and the operator can aim the same widget at `SUBSZ` or `JSZ`.
  The `console-readonly` demo role allow-lists `$SYS.REQ.ACCOUNT.PING.>` for it.

  A built-in badge on every Thing's detail page was written and then removed
  before release. Nothing in the schema marks which Things are gateways —
  `thing_types` has no such field, and "gateway" is a naming convention a tenant
  chooses — so the badge could not be gated, rendered on every device, and had
  to state "no leaf node attached" neutrally about temperature probes because it
  could not tell them from a site that was down. It also polled a whole
  account's connection list every 15 seconds to do it. A widget asks once, when
  someone wants to know. If a *board* of site status is wanted later, the thing
  to build is a `request` entry in `DataSourceType` so display widgets can poll,
  not a bespoke view.

  One caveat for a clustered hub: the widgets use `request()`, which takes the
  first reply, while CONNZ is answered by every server holding connections for
  the account. On the single-binary hub this platform ships, one reply is the
  complete answer.

- **The invitation email is editable in `/_`.** A new superuser-only
  `email_templates` collection holds the subject and body of every mail the
  platform composes for itself, rendered with `html/template` against the
  built-in copy as a fallback. Previously the invitation was a Go `const` in
  pb-tenancy, so changing one word of it meant editing a library, tagging it,
  `go get`, rebuild, redeploy.

  PocketBase's own editable templates are a closed set of five, all record flows
  on an auth collection, with no mechanism for registering a sixth — which is why
  anything the platform composes itself had nowhere to live but Go. Rows are
  seeded on first serve from the compiled-in values and never overwritten after
  that, so an operator's edit survives every upgrade; deactivating or breaking a
  row falls back to the built-in and logs it. The body is an HTML *fragment* —
  the document shell is added at send time, so `/_`'s rich-text editor is not
  fighting a `<!DOCTYPE>` it intends to strip.

### Changed

- **No demo role ships a `$SYS` publish deny any more, and adding one back is a
  regression.** `console-app` carried `$SYS.>` and `console-readonly` carried
  `$SYS.REQ.SERVER.>`; both are gone from `internal/demoseed/contract.go`.

  The operator-wide endpoints are served inside the `$SYS` **account**, and an
  account is a closed subject namespace — so a tenant publishing
  `$SYS.REQ.SERVER.PING.LEAFZ` reaches no responder whatever its permissions
  say. The new `TestTenantCannotReachServerEndpoints` proves it with a
  credential carrying no deny list at all. A deny therefore restated the account
  boundary somewhere strictly weaker, while carrying a real hazard to do it: in
  NATS a publish DENY beats a publish ALLOW, so the `$SYS.>` form silently kills
  the account-scoped endpoints a console session legitimately needs, and adding
  an allow beside it does not help. The symptom is a bare request timeout with
  the real reason arriving asynchronously on the connection's error handler.
  Allow-list the account endpoints; write no deny.
  `TestDenyingAllOfSysBlocksAccountMonitoring` pins all four shapes.

- **pb-tenancy absorbed into the platform.** `organizations`, `memberships` and
  `invites` are platform code now (`hooks/org_membership.go`,
  `hooks/invites.go`); the dependency is gone from `go.mod` and from
  `--version`.

  Only half the library ever ran here. `schema.json` has owned those three
  collections and every one of their API rules since the initial import, so
  `createCollections` and `setAPIRules` were dead code on this deployment — and
  the rules they would have written had long since diverged (organization
  creation is operator-only here, not "any authenticated user"), as had the
  roles, which gained `viewer` and `dashboard`. What came back in-tree is the
  half that did run: owner membership, the invitation lifecycle, and the accept
  endpoint.

- **`POST /api/tenancy/accept-invite` is now `POST /api/org/invites/accept`,**
  matching `/api/org/things`. The old path named a library that no longer
  exists. Invitation links already in someone's inbox are unaffected — they point
  at the console route `/accept-invite`, which posts to the API, not at the API
  directly.

- **`tenancy.log_to_console` is gone.** The one thing it did on this deployment
  was silence invitation-email failures. They go through the application logger
  unconditionally. An existing `config.yaml` carrying the key is unaffected;
  viper ignores it.

- **Resending an invitation always sent a dead link.** The console only offers
  Resend once an invitation has *expired* — `InvitationsView.vue` disables the
  button otherwise — and pb-tenancy's resend hook re-sent the mail without
  touching the token or the expiry. So every Resend mailed the link that had
  already lapsed, and redeeming it returned 410 and deleted the invitation.
  Resend now mints a fresh token and a fresh expiry before it mails anything,
  post-commit, so the link is durable before it is sent.

- **A failed invitation email produced no output anywhere.** The send error was
  swallowed behind the library's `LogToConsole` flag, which `config.yaml` set to
  `false`. The invite row was created, the console listed it as pending, and
  nothing had gone wrong as far as anyone could see. Failures are now logged at
  ERROR with the invite id, address and organization.

- **An invitation created outside a REST request got no token and no expiry.**
  Token and expiry were set on `OnRecordCreateRequest`, which fires for REST
  calls only — so an invitation written by a seed, a migration or a test landed
  with a blank token and a zero expiry, which the accept endpoint reads as
  expired and deletes. They are set on `OnRecordCreate` now. Same trap as the
  pb-nats trigger fields already documented in CLAUDE.md: if it has to happen on
  every path, it belongs on the model hook.

- **A hook bound after `app.Bootstrap()` on the `organizations`
  AfterCreateSuccess chain never ran.** pb-tenancy's handler returned without
  `e.Next()`, terminating the chain, and because it registered from inside its
  own `OnBootstrap` callback it was always last — so nothing downstream noticed.
  Anything binding later got silence: no handler, no error, no log.
  `RegisterManagedOrgExports` on an already-bootstrapped test app provisioned
  nothing. Guarded now by `TestOrgCreateDoesNotTerminateTheHookChain`, which
  binds on the bootstrapped app the harness hands out.

- **The invitation email interpolated a user-settable display name into HTML
  unescaped.** The template was parsed with `text/template`, not
  `html/template`, and `InviterName` comes from `users.name`.

- **The invitation link never carried the invitee's address.**
  `AcceptInviteView` reads an `email` query parameter to prefill the
  registration form an invitee without an account has to fill in first, and the
  template sent only the token — so that field was always blank and every
  invitee retyped an address the system already knew. Tokens are also unpadded
  base64url now, so they survive a query string without escaping.

- **New-device login alerts are off for `things`** (and, while it still existed
  in this release, `leaf_nodes`). Both are auth collections whose addresses are
  synthetic and undeliverable by
  construction — `hooks/thing_routes.go` mints `<code>@<org>.thing.local` so the
  `(organization, code)` join key has somewhere to live, not so anyone can be
  written to. The send is *blocking*: `apis/record_helpers.go` waits on it with a
  15-second timer before returning the auth response. With no SMTP configured
  PocketBase falls back to `mailer.Sendmail{}` and the runtime image has no
  sendmail binary, so every device auth from an unseen origin paid that wait.
  The fingerprint is `MD5(ip + user-agent)` and `maxAuthOrigins` is 5, so a
  device on a changing address kept regenerating origins and kept paying. Left
  enabled on `users` and `_superusers`, who are people with real addresses and
  can act on the alert.

### Fixed

- **Widget subject validation rejected every NATS system subject.**
  `validateSubject` carried a character allowlist with no `$`, so a Publisher or
  Button widget refused `$SYS.REQ.ACCOUNT.PING.CONNZ` — the site-connectivity
  recipe documented one release earlier — along with every JetStream
  (`$JS.API.*`) and KV (`$KV.*`) subject. All three are subjects a console
  session is explicitly allow-listed to publish to, so the form was refusing
  input the server would have accepted, and the message blamed the operator's
  characters rather than the rule.

  The allowlist is gone rather than extended by one character: it was a
  deny-list in the other costume, a guess at NATS's character set that could
  only fail silently and permanently. The validator now checks what NATS
  actually enforces — non-empty, no whitespace, no control characters, no empty
  tokens, `>` last and `*` alone in its token — and invents nothing else.

- **KV key validation had the opposite defect** and is corrected in the same
  pass. It named five forbidden characters and let everything else through, so
  `$`, `:`, `#` and `@` passed the form and were then refused by `@nats-io/kv`
  at write time, with the error arriving far from the field that caused it. It
  now mirrors the client's own rule (`/^[-/=.\w]+$/`, no leading or trailing
  dot). Wildcards stay rejected: every caller writes a concrete key, and the KV
  watcher builds its own filter internally.

  New `ui/src/composables/useValidation.spec.ts` — 49 cases, the file's first
  tests. Verified by reverting: reinstating the old allowlist fails nine of
  them, including all five system subjects.

- **The CA's 90-day expiry warning now reaches the console, which is the only
  surface that reaches anyone who can act on it.** 0.5.0 split the warning
  windows — 30 days for a renewable host certificate, 90 for a CA that can only
  be rotated — but did it in `hooks/cert_expiry.go` alone. The console kept one
  constant, so `NebulaCADetailView` badged a CA at 30 days and
  `CaRotationPanel` did not go urgent until then either.

  That is worse than an inconsistency between two screens. A Nebula CA belongs
  to a **tenant organization**, and only its owners and admins may rotate one
  (`POST /api/org/nebula-ca/rotate`) — deliberately, because the wait in the
  middle of a rotation belongs to whoever operates the devices. The 90-day
  window existed only on `/api/ready` and `/metrics`, which are the platform
  operator's surfaces, and the operator cannot rotate a tenant's CA. So the
  longer warning was delivered exclusively to the party unable to use it, while
  the party who could got 30 days — less than the procedure needs.

  `ui/src/utils/expiry.ts` gains `CA_EXPIRY_WARNING_DAYS` (90) beside
  `EXPIRY_WARNING_DAYS` (30), and `expiryState`/`expiryLabel` take the window as
  their second argument. New `ui/src/utils/expiry.spec.ts` covers both windows
  and fails if they are ever collapsed back into one constant — the first tests
  this module has had.

### Removed

- **`leaf_nodes`, `leaf-sync`, and the collection mirror.** An edge site is a
  Thing now. The `leaf_nodes` auth collection is dropped
  (`migrations/schema_update_drop_leaf_nodes.go`), along with `cmd/leaf-sync/`,
  `internal/leafsync/`, `hooks/leaf_node_provisioning.go`,
  `hooks/leaf_node_routes.go` (`GET /api/leaf/bootstrap` and its superseded
  `/api/leaf/operator-jwt` alias), the console's Leaf Nodes screens, and the
  `leaf-sync` release archive.

  **This is a breaking change and it has an order.** Stand each site's agent up
  against its Thing first — install
  [stone-age-io/agent](https://github.com/stone-age-io/agent), point it at the
  platform, run `agent -leaf-config` — and only then upgrade the Control Plane.
  An older `leaf-sync` keeps running afterwards against its already-generated
  `nats-leaf.conf`, but it can no longer log in or re-fetch anything.

  Three things went and each for its own reason:

  - **The mirror.** `leaf-sync` copied an organization's config collections
    into the edge's local JetStream KV so devices could read them offline.
    Nothing consumed the mirrored rows — no rule-router rule, no firmware — so
    it was moving data nobody asked for, and it was the reason a leaf node
    needed read grants spread across the inventory.
  - **The collection.** With the mirror gone, a leaf node was a Thing with a
    `domain` column and one server-provisioned NATS user. `thing_types` already
    says whether a device is a gateway, so the split was a second way to state
    something the schema stated once, and the `domain` column was a second copy
    of the code that could disagree with it.
  - **`leaf_status`.** The heartbeat bucket is gone and nothing replaced it on
    the platform side. A heartbeat travels over the very link whose failure it
    is meant to report, so a missing beat could not tell "edge box down" from
    "WAN down" from "agent crashed" — and the Control Plane could never read one
    anyway, holding the operator and `$SYS` and no credential inside any
    tenant's account. The hub always knows which leaves it is holding, so the
    console asks it.

  **Credentials are left working on purpose.** A leaf node's `nats_users` and
  `nebula_hosts` rows are non-cascade, so they survive the collection and stay
  live; the migration lists them rather than revoking them, because a migration
  that runs on deploy is not the place to take a fleet off the bus. Deactivate
  each from the console once its site is running against a Thing — and
  deactivate rather than delete, since revoking a Nebula certificate needs the
  certificate still in the database to fingerprint it.

  `stone_age_records{collection="leaf_nodes"}` and
  `stone_age_inactive_records{collection="leaf_nodes"}` are gone with it. The
  `leaf-sync` binary is no longer built or released from this repo; the agent
  releases on its own tags.

## [0.5.1] - 2026-09-13

One layout fix, shipped on its own because it is on the first screen of the two
most-used sections and it is worse on a phone than it sounds: the primary action
was off the edge of the viewport, not merely cramped.

Nothing else changed. No schema migration, no dependency bump, no API surface —
upgrading from 0.5.0 is replacing the binary or the image.

### Fixed

- **The New button on the Things and Locations lists overflowed the screen on
  mobile.** DaisyUI's `.btn` carries `flex-shrink: 0`, and the New button was
  `w-full sm:w-auto` — correct while it was the only thing in that flex row,
  wrong the moment the Labels button joined it in 0.5.0. It asked for 100% of a
  row it was now sharing, neither button could give any back, and the primary
  action rendered past the right edge of the viewport.

  Both are now `flex-1 sm:flex-initial`, which restores shrink over DaisyUI's
  zero and splits the row evenly. That is the idiom the Thing and Location
  DETAIL views already used for their action rows, added in the same commit
  that introduced these buttons — the list views simply kept the older
  single-button `w-full`. Reused rather than writing a second responsive
  pattern for the same problem two files away.

  Sixteen other views have a lone `btn ... w-full sm:w-auto` and are fine: one
  button owning its row is what that class pair is for. The bug is not the
  class, it is the class surviving the arrival of a sibling — which nothing
  here can catch, since `vue-tsc && vite build` has no opinion about layout
  and there is no visual test.

## [0.5.0] - 2026-09-13

Tenancy enforcement, and the discovery that the review question was wrong. Every
inventory read rule scoped on `organization = @request.auth.current_organization`,
both sides are TEXT columns whose zero value is the empty string, and in
PocketBase an empty string equals an empty string — so a record with a blank
organization was readable by any caller whose own context was blank. No role was
bypassed and no rule was mis-written; two sentinels compared equal. Both halves
were ordinary product states an owner could reach in two clicks. The question to
ask of a rule is not "does this name the right roles" but "what does this do when
both sides are the zero value".

The platform also learned to report on itself. `GET /api/ready` and `GET /metrics`
on the Control Plane, `/ready` and `/metrics` on `leaf-sync`, with one rule
behind every check: it must be answerable first-hand by the process running it.
That is the NATS account boundary restated, and it is why there are no per-org
labels and no platform credential inside a tenant's account. Nebula certificate
expiry is the one exception, and it earns it — those are certificates this
process signed itself and stores.

Nebula caught up with pb-nebula v0.3: CA rotation is a three-step route the
tenant owns, the `/32` certificate defect has an audit and a per-host re-issue,
and decommissioning finally closes the overlay door as well as the console and
NATS ones. It had been closing two of three.

And the console got a test runner, which it did not have. 179 tests over the pure
logic `vue-tsc && vite build` stays green while it breaks.

**Upgrading an existing database.** `migrate up` handles all of it, but four
changes are visible afterwards:

- **Orphaned records stop being readable through the API.** Anything left at a
  blank `organization` by an organization deleted under a previous version —
  things, locations, types, leaf nodes, `nats_accounts`, `nebula_ca` — no longer
  matches any tenant read rule. The rows are still there and a superuser still
  sees them in `/_/`. That is the fix working, but it means a console that could
  list those records before will not afterwards.
- **`message_schemas` is dropped**, along with `thing_type_operations.schema`
  and the dead `capabilities` / `nats_role` fields on `thing_types`. The
  migration also prunes `message_schemas` out of any leaf node's
  `synced_collections`, since the edge never synced it and a badge naming a
  collection that no longer exists tells the operator something untrue.
- **`leaf-sync` opens a loopback listener it did not open before.**
  `observability.addr` now defaults to `127.0.0.1:9100` instead of empty. Set it
  back to `""` to switch it off. The Control Plane's `/api/ready` and `/metrics`
  are routes on the existing server and open no new port — but `/metrics` is
  open by default there, so put `metrics.token` or a proxy in front of it if the
  server is exposed.
- **A `nebula_hosts` record created without an `active` field now lands ACTIVE**
  (pb-nebula v0.2.0+). A host born inactive is one every peer blocklists at
  birth. Anything creating hosts through the API and relying on the old default
  should set the field explicitly.

### Security

- **`locations.floorplan` is restricted to image mime types.** It was the only
  upload field on the platform with an empty `mimeTypes` list, so it accepted
  any file at all. The exposure was the thumbnail rather than the upload:
  PocketBase allows `?thumb=100x100` on any file field regardless of that
  field's own `thumbs` list (`defaultThumbSizes` is checked before
  `fileField.Thumbs` in `apis/file.go`), so `"thumbs": []` never meant "no
  thumb can be generated", and a thumb request runs the stored bytes through
  `imaging.Decode` + `Resize`. `disintegration/imaging` registers
  `golang.org/x/image/tiff` and panics on a crafted TIFF (CVE-2023-36308); it
  is unmaintained, v1.6.2 is the last release, and there is no patched version
  to upgrade to, so narrowing what reaches the decoder is the only lever
  available.

  Rated Low upstream and hardening rather than a fix here: the panic is
  recovered by `net/http` so the process survives, and the caller has to be an
  authenticated member of the organization requesting a thumb of a file they
  uploaded themselves. Existing files are unaffected — `mimeTypes` is validated
  on upload, not on read.

- **Documented why the `maplibre-gl` v5 pin does not carry
  GHSA-jrc7-96c5-q579.** The advisory is critical, its range covers every
  published 5.x, and the fix landed only in 6.4.1 — a line this repo holds back
  for the worker-splitting reason in CLAUDE.md, so there is nothing to upgrade
  to. The vulnerable `DOM.sanitize()` has exactly one sink, MapLibre's own
  attribution control, and that control is never constructed:
  `@maplibre/maplibre-gl-leaflet` hardcodes `attributionControl: false` when it
  builds the `maplibregl.Map`, `useLeafletMap.ts` passes the same, and it is
  the app's only MapLibre instance. The credit on screen is Leaflet's control,
  fed by a constant.

  No code changed. What changed is that `attributionControl: false` now says it
  is a security setting, because flipping it to render the OpenFreeMap credit
  through MapLibre would reintroduce a critical XSS with nothing to catch it.
  The comment also records the larger hazard: the Leaflet binding lifts a
  style's source `attribution` into Leaflet's attribution control, which
  assigns to `innerHTML` unsanitized, so making `STYLE_URLS` configurable would
  open a path upgrading maplibre does not close.

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

- **The inventory-fields builder silently stripped every field's `title`.**
  `title` is the human label — it is what turns `dock_doors` into "Dock doors"
  on the form a member fills in, and both `JsonSchemaForm` and
  `MetadataEditor`'s read-only view render it. `schemaFields.ts` never had a
  `title` on its internal `Field`, so `schemaToFields` did not read the
  keyword and `fieldsToSchema` did not write it back. Opening a thing type or
  location type on the Form tab and changing anything at all rewrote the schema
  without it.

  There was no warning, because `isFormCompatible` — which decides the "switch
  to JSON view" banner — knows about `$ref` and the combinators and nothing
  about which keywords survive the round trip. Every type in the demo seed uses
  `title` on every property and none uses `description`, so the first edit to a
  seeded type reverted its whole form to raw key names. The builder now has a
  Title input beside Name, and `schemaFields.spec.ts` asserts the round trip
  against a copy of the seeded `warehouse` fixture.

- **A schema built in the form could not be reopened in the form.** The builder's
  nesting cap was enforced in two places that disagreed by exactly one level:
  `isFormCompatible` accepted four levels of nested properties, while
  `SchemaFieldEditor`'s `depth < maxDepth` let you author five. Nest five deep,
  save, reload, and the Form tab refused its own output with "nesting deeper
  than 4 levels. Switch to JSON view." (Arrays were inconsistent with objects on
  top of that — `isPropCompatible` charged array-of-object an extra level that
  the editor did not.)

  The cap is gone rather than corrected. Everything else that predicate rejects
  is a fact about what the builder can *represent* — there is no form equivalent
  of `$ref` or `anyOf` — whereas a depth limit is a rendering preference, and
  mixing the two kinds of judgement in one function is what produced the
  off-by-one. The recursion terminates on the data either way, and nothing in
  the product nests past one level.

- **A numeric `enum` stored its selection as a string.** The enum `<select>` in
  `JsonSchemaForm` emitted `event.target.value` verbatim, which is always a
  string, while the builder happily authors `{"type": "integer", "enum": [1, 2,
  3]}`. Picking `3` wrote `"3"` into `metadata`. That document is read back off
  the bus by firmware and rule-router, so this was a type mismatch at the far end
  of the wire rather than a display bug. It now casts by the declared type, the
  same way the free-text input already did.

- **A `number` field blocked the form it was on.** `JsonSchemaForm`'s numeric
  input set no `step`, and `<input type="number">` defaults to `step=1` — so a
  schema-declared `number` rejected `20.5` as invalid and the surrounding thing
  or location form would not submit. `MetadataEditor`'s free-form number row
  already carried `step="any"` for this reason. The schema-driven one now sets
  `any` for `number` and `1` for `integer`, and passes `minimum`/`maximum`
  through as `min`/`max` — the builder had been authoring bounds that nothing
  enforced.

- **"Infer from sample" could hand back a schema the form then refused.** A
  `null` in the pasted sample inferred `{"type": "null"}`, which is not a type
  the builder has, so the incompatible banner appeared in the same click that
  toasted "review before saving". A null in a sample means "this key exists and
  was empty when I looked", not "this key may only ever be null", so it now
  infers `string`. A mixed-type array still yields `anyOf` and still trips the
  banner; that one is genuinely outside what the form can express.

- **The host detail view's "Regenerate Certificate" button did nothing, and said
  it had.** It sent `{ regenerate: true }`. There is no `regenerate` field on
  `nebula_hosts` and there never has been; PocketBase drops unknown keys from an
  update body without complaint, so the request returned 200, the console
  toasted "Certificate regenerated", and the certificate on screen afterwards was
  the same certificate. It now sends `renew`, pb-nebula's actual action field.

- **A CA now gets 90 days' warning rather than 30.** `certExpiryWindow` was
  shared between host certificates and the CA. A host certificate is renewable —
  pb-nebula re-issues one automatically — so 30 days is ample. A CA cannot be
  renewed at all: the only remedy is rotation, and rotation needs long enough in
  the middle for every host to fetch a config nobody told it to fetch. Nebula's
  own guide asks you to begin two to three months out. Thirty days' notice on a
  CA is notice that the remedy no longer fits. pb-nebula warns at 90 too, but on
  a log line this deployment never sees, because `nebula.log_to_console` is
  false here.

- **`ConfirmDialog` is now an actual dialog.** It is the gate in front of every
  destructive action in the console — deleting a Thing, revoking a credential,
  decommissioning a device — and it was a plain `<div>`: no `role="dialog"`, no
  `aria-modal`, no labelling, no Escape handler, no focus trap, and `autofocus`
  on the **destructive** button, so Enter on a dialog nobody had read deleted the
  thing.

  It now announces itself as a modal, labels and describes itself from the title
  and message it already renders, hides the decorative emoji from assistive
  technology, cancels on Escape, traps Tab, and returns focus to whatever opened
  it — tolerating that element being gone, since the confirmed action has often
  removed the row whose button opened the dialog. Focus lands on the dialog
  container rather than a button, so nothing is pre-selected and Enter cannot
  confirm by accident; the first Tab reaches Cancel because it comes first in the
  DOM. Also a visible `:focus-visible` ring on the buttons, and the animations
  respect `prefers-reduced-motion`.

  Fourteen tests cover it, and they are the one place in this suite that mounts
  a component — what is under test there *is* the DOM contract. Writing them
  caught a bug in the implementation: the focus trap filtered candidates on
  `offsetParent !== null`, which is null for every element under jsdom and for
  anything inside a `position: fixed` subtree in some engines, so the trap was
  silently a no-op.

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

  This needs pb-nebula v0.2.0, which is now pinned (see **Changed** below).
  Against v0.1.0 the flag was mirrored correctly and no blocklist was ever
  produced, so the platform half was inert but harmless.

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

- **The at-rest encryption boundary is now stated instead of implied.**
  `nats.encryption_key` / `nebula.encryption_key` protect the material needed to
  *mint* identities — the operator seed, account seeds and signing keys, the
  Nebula CA key. They do not protect `nats_users.creds_file` or
  `nebula_hosts.config_yaml`, and cannot usefully: a `.creds` file *contains* the
  user seed by construction, Nebula requires the host key inline, and the browser
  reads `creds_file` straight from the API to open its own NATS connection — so
  encrypting that column would force every read through a decrypting route.

  So a stolen `pb_data/data.db`, with the key held separately, yields **no
  ability to mint new identities** and **every existing credential**. That is the
  line the feature defends, and the two halves cost very different amounts to
  remediate: the NATS side is a central, scriptable `regenerate` with a permanent
  revocation cutoff; the Nebula side has no CRL, so it needs re-issue plus a
  blocklist entry in every peer plus redelivery.

  No code changes beyond the `encryption_at_rest` readiness check, which reported
  a bare "enabled for NATS and Nebula" — accurate about the config and misleading
  about the guarantee. It now names what it covers. The at-rest threat is
  answered by disk encryption, encrypted backups, and single-tenant deployments,
  which is where `SECURITY.md` now points.

- **CI now runs the checks that already existed.** `scripts/check-sort-fields.sh`
  was written, worked, and was called by nothing — guarding a failure mode that
  had already killed the Members and Invitations screens for every caller, since
  an unknown `sort` field is a 400 raised before any rule is evaluated, names no
  field, and fails for superusers too. It runs on every pull request now.

- **The pinned-major guard covers `maplibre-gl`.** It checked Tailwind, daisyUI
  and TypeScript, and omitted the one of the four whose failure is completely
  silent: v6 splits its tile-parsing worker out of the bundle and resolves it as
  a sibling file that Vite never emits, so nothing throws, nothing reaches the
  console, and the map renders as a flat sheet of theme colour that reads as a
  design choice.

- **`gofmt` is now a gate, which it could not previously be.** Two migrations
  carried `''` inside a doc comment, and gofmt rewrites that into a typographic
  quote — so running it would have corrupted the comment's meaning, which is why
  the project's notes said not to add this check. Those comments were reworded to
  avoid the construct rather than accepting the rewrite, and an import ordering
  slip in `observe_test.go` was fixed, so every tracked Go file is clean and the
  gate is safe.

- **`go test -count=1`**, because `setup-go` caches the build cache between runs
  and a cached pass is a memory of a result from some other commit. Plus
  `go mod tidy -diff`, which was clean and unguarded.

- **A guard on the `pb_public/index.html` placeholder.** It is tracked so
  `go build` can satisfy its `//go:embed` on a fresh clone with no Node
  installed, and `npm run build` overwrites it — so a stray `git commit -a`
  silently commits the built console into the placeholder's slot, and the next
  fresh clone embeds a stale hashed-asset reference. Checked before the build
  step, since afterwards the file legitimately differs.

- **`HEALTHCHECK` honours `STONE_AGE_HTTP_PORT`.** It hardcoded 8090 while
  `docker-entrypoint.sh` made the port configurable, so setting that variable
  produced a permanently unhealthy container that was serving correctly.

- **Every API-rule rejection was counted as a server error.**
  `stone_age_http_requests_total` bucketed by status class, and PocketBase's
  router runs its error handler *after* the middleware chain unwinds — so a
  handler that returns an error leaves the tracked status at 0 when the metrics
  middleware sees it. That case returned `5xx`, and *every* authorization
  rejection arrives that way: 400 on a denied create, 404 on a denied update,
  401/403 from `RequireAuth`. The 4xx bucket sat near-empty while a `5xx` alert
  fired on a platform doing exactly its job. The status is now resolved through
  `router.ToApiError`, which is what the error handler itself calls before
  writing the header.

  Split into a pure `statusClassFor(status, err)` so it can be asserted at all —
  the bug was invisible partly because nothing could construct a
  `core.RequestEvent` to test it.

- **A second widget-defaults function was silently reverting the first.**
  `createWidget` called `createDefaultWidget` and then `applyWidgetDefaults`,
  which ran afterwards and won every conflict — so the newer defaults in
  `types/dashboard.ts` were being overwritten by an older copy that had drifted:
  `kvtable` lost its reported-state twin bucket for an empty one (making the
  `TWIN_BUCKET` import and its "reads reported state" comment dead), and
  `button`, `switch` and `slider` lost their `cmd.thing.*` / `twin_desired`
  subjects for older placeholders. It also had no `scanner` case at all — 15
  branches for 16 types — surviving only because the other function ran first.

  `createDefaultWidget` is now the only source. The titles and `$.value` JSON
  paths the second function contributed were carried across, so what a new
  widget gets is unchanged apart from the reverted values being restored; net
  −94 lines. The file also carried a literal `// ... rest of file unchanged ...`
  placeholder, which went with it.

- **`configComponents` is typed `Record<WidgetType, Component>`.** It was
  `Record<string, Component>`, so a widget type with no config component was a
  modal that opened onto nothing — no error anywhere, at build time or runtime.
  It is now a compile error, verified by removing an entry and watching
  `vue-tsc` report `TS2741: Property 'scanner' is missing`.

- **`WIDGET_TYPES` is a runtime list, with `WidgetType` derived from it.** The
  union existed only at compile time, so nothing could iterate the types and
  every "is every type handled" question needed a second, hand-maintained copy
  of the list. `Record<WidgetType, …>` still fails to compile when a type is
  missing, and tests can now walk all sixteen.

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

### Changed

- **pb-nebula bumped to v0.2.0**, which is what makes the Nebula half of
  decommissioning above actually do something. Against v0.1.0 `active` was
  mirrored onto `nebula_hosts` correctly and no `pki.blocklist` was ever
  produced, so that half was inert. No platform code changed: pb-nebula's
  `options.go`, `nebula.go` and `errors.go` are untouched between the two tags
  and the whole feature lives under its `internal/`, so this is a go.mod bump.

  It does change one behaviour that is not the platform's own. A `nebula_hosts`
  record created **without** an `active` field now lands active, because
  pb-nebula forces the flag on create — PocketBase bools have no schema default,
  and an inactive host is one whose certificate every peer blocklists, so a host
  born inactive would be refused by the whole network from the moment it was
  signed. Nothing in this platform relied on the old behaviour: both
  `POST /api/org/things` and the console's Nebula host form always sent
  `active` explicitly. `scripts/test-authz.sh` now pins the contract anyway
  (176 checks), because it is a dependency's guarantee rather than one of our
  rules, and a downgrade would otherwise be silent.

- **A documentation truth pass**, in this repo and in `platform-docs`. The
  headline feature list still sold message schemas — "versioned JSON Schema" —
  which is the worst place for a stale claim, since a buyer demos the thing they
  were sold. The widget table listed a **PocketBase** widget that has no
  component behind it (sixteen types, none of them `pocketbase`), the role count
  said four, the agent's binary was called `stone-age-agent` where its own
  install snippet says `agent`, and this file said bootstrap is three commands
  where it is four — `nats export` is required by `serve --nats` and cannot run
  before `bootstrap`, which is why `docker-entrypoint.sh` does all four.

  The authorization check count was wrong in two places and is now correct in
  both. `README.md` also described operations as carrying "an optional schema"
  and the console as having an infer-from-sample tool for it; both went with
  `message_schemas`.

  ADR 0002 was **amended rather than rewritten**, since an ADR records what was
  decided: the org-code pattern now permits a leading digit, and the option that
  would have reserved `system` and `operator` is marked as not what shipped.

  Two claims a technical buyer would break are gone: "hundreds of clients using
  a single deployment", which sat three pages from "the Control Plane scales
  vertically" on a single-writer SQLite database, and an unsupportable
  superlative about security that traditional platforms "simply cannot match".
  Both are replaced with the mechanism, and with an explicit note that there are
  no production deployments to quote figures from.

  `demo-seed` is now in the getting-started guide, having been documented only
  here despite being the fastest path from a fresh install to something legible.

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

- **`migrations/widen_capabilities.go`, which could never have run.** Migrations
  sort by filename, so `schema_update_drop_message_schemas.go` — which removes
  `thing_types.capabilities` — runs before `widen_capabilities.go`, which
  rewrites it. `GetStringSlice("capabilities")` was empty for every row on fresh
  and upgraded databases alike, forever, and `remapCapabilities` was dead code. A
  file whose stated job is impossible is worse than no file: it is the
  counter-example to the migration discipline the rest of the package documents
  carefully.

- **Two dead writes in the demo seeder.** `seedThingTypes` still set
  `capabilities` and `nats_role` on `thing_types`, both of which were dropped
  with the contract layer — and PocketBase silently discards a write to a field
  that does not exist, so this looked like it was seeding data nobody could find.
  The `Capabilities` fixture field went with them (23 initializers). The role
  *lookup* stays as a fixture check, because `roleForThingType` still uses
  `tt.Role` to pick the identity for each device of that type, and a fixture
  naming a role that does not exist should fail there rather than at the first
  device.


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

- **A Title field in the inventory-fields builder**, beside Name. Name is the key
  stored in `metadata` and read back off the bus; Title is the label a member
  sees on the record form, falling back to the name when blank. Until now the
  only way to set one was the JSON tab, and the Form tab deleted it on the next
  edit.

- **A duplicate property-name warning in the builder**, at every nesting level
  including array item properties. `fieldsToSchema` writes into a plain object,
  so two properties sharing a name silently collapse to one and the field typed
  first disappears on save. `MetadataEditor` already warned about exactly this
  for free-form keys; the schema builder now says the same thing, in the same
  words, and marks the colliding input.

- **Specs for `schemaFields.ts` and `inferSchema.ts`.** Both are pure functions
  that `vue-tsc && vite build` stays green over while they silently lose data —
  the `title` bug above is what a round-trip assertion is for. `schemaFields`
  pins the `schemaToFields ∘ fieldsToSchema` identity over titles, formats,
  enums, bounds, nested objects and array-of-object, and pins that there is no
  depth at which the form stops accepting its own output. `inferSchema` pins the
  invariant that what it produces stays editable in the builder.

- **Nebula CA rotation, from the console.** `POST /api/org/nebula-ca/rotate`
  (owner/admin) drives pb-nebula's three steps — `prepare`, `commit`, `finish` —
  and the CA detail view presents them as a three-step panel with the state it
  derives from the certificates themselves.

  It is a route rather than a rule branch because `nebula_ca.updateRule` is
  operator-only, and its comment has said since the authz hardening pass: "There
  is no tenant-triggered CA rotation today because there is no trigger field for
  one. If that changes, add a route rather than a branch here." pb-nebula v0.3.0
  added the trigger field. A branch would have to deny-list ten fields and would
  silently re-open every field added afterwards — the same deny-list shape this
  repo has already been bitten by twice, on the record holding the trust anchor
  for a tenant's whole mesh.

  Three steps and not one because Nebula verification is mutual and config
  distribution is pull-based: a single write carrying both the new trust bundle
  and the new certificate splits the mesh, since a host that has fetched presents
  a new-CA certificate to one that has not and the handshake fails in *both*
  directions. `prepare` publishes trust and moves no issuance, so it is fully
  reversible. `commit` swaps issuance and re-signs every active host. `finish`
  drops the outgoing CA and is refused while any active host still holds a
  certificate signed by it.

  The tenant owns the lever because the dangerous part of rotating a CA is the
  **wait** in the middle, and the wait belongs to whoever operates the devices.

- **A certificate audit for the `/32` defect**, at `GET /api/org/nebula/cert-audit`
  (owner/admin), surfaced as a "Wrong mask" badge on the host list and a banner
  with a re-issue button on host detail.

  pb-nebula signed host certificates at `/32` until v0.3.0. Nebula does not read
  a certificate's network as "this host's address" — it puts the prefix straight
  onto the tun device and installs a link route for it, so the mask in the
  certificate **is** the host's route to the overlay. A `/32` gives a host a
  route covering only itself: the certificate verifies, the config renders, the
  host starts, the handshake completes, and no packet ever crosses the mesh.
  Nothing errors anywhere, which is why it survived from that library's first
  commit.

  Existing hosts are **not** re-signed automatically. Re-signing moves a
  certificate's fingerprint, and a fingerprint is what `pki.blocklist` revokes,
  so a sweep would rewrite every peer config in the mesh on the strength of a
  dependency bump. The audit names the affected hosts and the console offers a
  per-host re-issue instead. Inactive hosts are excluded: they are revoked, and
  re-signing one would publish a new fingerprint while the old certificate
  stayed valid.

  It is a route because answering it means parsing a Nebula certificate, which
  the browser cannot do.

- **The rest of pb-nebula v0.3's host surface, in the console.** Relay
  (`is_relay`, with the `public_host_port` a relay needs or it listens on an
  ephemeral port while peers hold its overlay IP), gateway routing
  (`unsafe_networks` on the provider, `unsafe_routes` on the consumer — two
  halves on two different hosts, neither derived from the other),
  `preferred_ranges` for underlay path selection, and per-host `mtu` and
  `tun_device` overrides. The host list now badges lighthouse and relay
  independently, because a host can be both.

- **126 frontend unit tests**, across the five places most dangerous to change
  blind. All pure logic, no component mounting:

  - **`twinDrift`** — twenty lines the documentation spends several hundred words
    specifying: subset semantics for objects, exact for arrays and scalars, and
    no operators, ever. None of it was pinned, and the function is typed
    `(any, any)`, so a well-meaning change to make a range work would have
    compiled, passed the build, and quietly redefined what every desired value
    on every deployment means. One test asserts that an operator-shaped desired
    value is compared as a plain value — if it ever starts passing, someone has
    begun building a rules engine inside a KV browser.
  - **`useSubscriptionManager`** — the module singleton every live value flows
    through, previously untested: refcounted listeners, a shared key per core
    subject, and close-on-last-leave. Driven by a fake connection.
  - **The `can` map** — the console's entire authorization surface, mirrored from
    `schema.json` by hand with nothing checking the mirror. The full 5×8
    role/capability matrix from `CLAUDE.md`, transcribed, plus the fail-closed
    cases: no membership is `null` rather than a role, a membership in a
    *different* organization grants nothing, and `dashboard` holds nothing at
    all.
  - **Dashboard import/export** — a round trip, new ids on import, the storage
    location stripped, malformed entries skipped rather than thrown, and the
    limit enforced *before* anything is written. The `replace` strategy clears
    every local dashboard first, so a partial import is how a user loses work.
  - **`createDefaultWidget`** — all sixteen types (added with the runner).

  Two real defects fell out of writing them: `extractJsonPath` was typed
  `path: string` while every caller passes a possibly-undefined `jsonPath`, and
  the `manageDefinitions` comment still listed message schemas.

- **A frontend test runner.** Vitest, Node environment, no component mounting —
  the highest-risk logic in the console is pure (widget defaults, the capability
  map, dashboard import/export, twin drift) and all of it was previously
  unguarded, since `vue-tsc && vite build` stays green while any of it is wrong.
  `npm test` runs it, and CI runs it before the bundle so a logic regression
  fails fast. The first suite pins `createDefaultWidget` across all sixteen
  widget types, which is what made the defaults merge above safe to attempt.


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
  record detail views, and from the Things and Locations lists, where a
  `Labels (n)` button prints the whole result set of the current filter rather
  than the current page — the search box is the selection mechanism, so there are
  no row checkboxes and no selection state. The modal takes a **list** — a detail
  view passes a list of one — so a batch and a single sticker are the same code
  path; records without a code are skipped and named rather than dropped
  silently. The
  payload is the **bare code** — no host, no organization, no kind token —
  because a sticker in a public hallway is something a stranger can replace,
  and a URL payload would let a forged label send a person to arbitrary
  content. Sized in millimetres to real thermal stock (2″ × 1″ and 4″ × 2″).
  The existing scanner widget reads them with no changes, since a bare code was
  always what its `{value}` placeholder expected.

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

[Unreleased]: https://github.com/stone-age-io/platform/compare/v0.8.0...HEAD
[0.8.0]: https://github.com/stone-age-io/platform/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/stone-age-io/platform/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/stone-age-io/platform/compare/v0.5.1...v0.6.0
[0.5.1]: https://github.com/stone-age-io/platform/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/stone-age-io/platform/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/stone-age-io/platform/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/stone-age-io/platform/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/stone-age-io/platform/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/stone-age-io/platform/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/stone-age-io/platform/releases/tag/v0.1.0
