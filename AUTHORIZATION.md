# Roles & Authorization — the long form

The role table, the five roles, and a one-line version of every rule below are
in the root `CLAUDE.md`. This file is the reasoning behind each rule and the
incident that taught it. Read it before you change an API rule in `schema.json`,
write a route or hook that saves with `app.Save()`, touch anything that holds a
credential, or gate a control on a `can.*` capability.

Rules to follow when touching authorization:

- **Use an allowlist, never a deny-list.** `role ?!= "member"` was satisfied by
  `dashboard` — the *least privileged* role — so its holders passed every admin
  check. Write `(role ?= "owner" || role ?= "admin")`. Copy the canonical snippet
  verbatim from a neighbouring rule; do not hand-write a variant.
- **Restricting fields is not restricting roles.** The same bug came back in a
  second costume: `things` create/update admitted a member through a branch that
  froze `nats_user`/`nebula_host` but named no role, and `locations` create/update
  had no role check at all — so `dashboard` satisfied both and could write
  inventory. A branch that constrains *what* may be written still has to say *who*
  may write it. Every write branch names its roles.
- **An empty string is a valid match, so a blank scope is a wildcard.** The
  third costume of the same bug, and the one no role audit could have caught:
  every inventory read rule was `organization = @request.auth.current_organization`,
  both sides are TEXT defaulting to `''`, and in PocketBase `'' = ''` is true. A
  record with a blank `organization` was therefore readable by any caller whose
  own context was blank — no rule mis-written, no role bypassed, two sentinels
  comparing equal. Both halves were ordinary product states: deleting an
  organization blanks `organization` on the 15 relations into it that are
  non-cascade AND non-required (PocketBase blanks rather than deletes, via
  `SaveNoValidate`), and `hooks/membership_lifecycle.go` blanks
  `current_organization` on the way out of an org. A second set of branches, for
  a `leaf_nodes` collection that has since been dropped, had the identical hole
  on `@request.auth.organization`. Fixed in `migrations/schema_update_tenancy_sentinel.go` by
  requiring a non-blank context **and** a correlated membership — the membership
  clause is the load-bearing half, because `memberships.organization` is
  required and cascade so it can never be `''`, which kills the sentinel by
  construction rather than by a comparison someone may later tidy away. Note
  every *write* rule already had that clause, which is exactly why only the
  reads were exposed. **The review question is not "does this rule name the
  right roles" but "what does this rule do when both sides are the zero
  value."**
- **`current_organization` is the read boundary, so guard every path that
  writes it.** `users.updateRule` froze it to organizations the caller holds a
  membership in; `users.createRule` did not mention it, so an invited registrant
  could name any organization id at signup and then read that tenant. The create
  branch is anonymous and cannot check membership even in principle, so it
  refuses the field outright and accept-invite fills it in afterwards. A field
  that scopes reads needs a guard on *create* as well as update.
- **Do not put an apostrophe in a rule comment.** PocketBase tokenizes quote
  characters in the rule string before `//` comments are stripped, so one stray
  apostrophe re-pairs every quote after it: a later `''` literal then opens a
  string that nothing closes, and the whole collection fails to import with
  `invalid quoted text`. This cost a debugging cycle — the rule looked correct
  and the failure named a fragment of the expression rather than the comment.
  Several existing comments contain apostrophes and are harmless only because
  no string literal follows them. Write "the organization" rather than
  "the org's".
- **Keep a zero-authority role in the test matrix.** The two role bugs above were caught
  by the same thing: a role that holds no capability at all, used as the probe in
  `scripts/test-authz.sh`. `dashboard` is that role. Don't "simplify" the suite by
  testing denials with `member` — a role with *some* authority cannot prove an
  allowlist, because it passes for the wrong reason. (This role was called `badge`
  until it was renamed in `migrations/schema_update_dashboard_role.go`; commits and
  PRs before that date say `badge` and mean this.) `viewer` cannot take over the
  job either: it holds read capability, so a denial it passes proves less. Two
  roles, two purposes — don't merge them to save an enum entry.
- **Reads are org-scoped, not role-scoped, and that is deliberate.** Every read
  rule on `things`, `locations`, `thing_types`, `location_types` and
  `thing_type_operations` scopes on the active organization plus
  a correlated membership in it (see the sentinel bullet above), with no ROLE
  branch, so *every* role in an org — `dashboard` included — can
  `curl` the whole inventory. `viewer` therefore reads exactly what `member`
  reads; the difference between them is writes plus which screens
  `ui/src/router/index.ts` navigates to. Do not describe the console's
  navigation as a read boundary, and do not "tighten" a view for a role by
  editing `can.*` — that is theatre. A read that must actually be a boundary is
  a branch in `schema.json`, with the cost that comes with it: eight collections
  to keep in step, and a new failure mode where a relation expansion silently
  returns nothing.
- **A read-only role costs one enum entry, and that is the point.** `viewer` was
  added (`migrations/schema_update_viewer_role.go`) with **zero rule text
  changes**: a role value naming itself in no write branch is denied everywhere
  by construction. That is the dividend of the allowlist rule above, and it
  doubles as a test of it — if you ever find yourself editing a rule to keep a
  new read-only role *out*, that rule is a deny-list and it is the bug. The
  work of such a role is all in the UI: split the capability
  (`viewInventory` vs `manageInventory` in `ui/src/stores/auth.ts`), point
  list/detail routes at the read one and forms at the write one, and gate every
  create/edit/delete control in the views.
- **Gate a write control on the capability that matches its rule, not the one
  that matches its screen.** Adding `viewer` surfaced two buttons that had been
  wrong since before it existed: the Delete buttons on the Things and Locations
  lists, and on the Location detail view, were ungated, so a `member` saw a
  Delete the server then refused (`deleteRule` is owner/admin). Any control
  whose rule differs from its screen's entry capability needs its own `v-if`.
- **Hooks enforce INVARIANTS; rules enforce PERMISSIONS; routes handle what a
  rule cannot express.** "The API rules are the only enforcement layer" is about
  permissions, and `hooks/relation_tenancy.go` is the one deliberate exception —
  worth knowing where the line is, because the temptation to move authorization
  into hooks recurs. It refuses a relation pointing into another organization's
  records, which no rule *can* express (PocketBase traverses a STORED field, but
  `@request.body.account_id` is a raw submitted string with no target to
  traverse into). pb-nats signs the user JWT with whatever `account_id` names,
  so without it an owner could mint themselves a credential inside another
  tenant's account. It is DERIVED from the schema rather than listing the
  nineteen relations, binds the MODEL hooks so it covers `app.Save()` routes as
  well as REST, and applies to superusers — a cross-tenant relation is corrupt
  data whoever wrote it.

  Three reasons not to generalise this into a permission system. Audit and
  enforcement need **opposite failure modes** — an audit hook must never block a
  write, an enforcement hook must always block one, which is the same Before vs
  `After*Success` distinction `hooks/membership_lifecycle.go` already turns on.
  Hooks **cannot be a read boundary**: `OnRecordsListRequest` hands you
  `Records` and `Result` after the rule-scoped query has run, so filtering there
  gives wrong `TotalItems` and broken pagination, and there is no pre-query hook
  to inject a scope. And a second authoritative layer means "can role X do Y"
  has two answers. Tenant-defined roles, if they ever land, belong in the rules:
  make `memberships.role` a relation to an org-scoped role collection with
  boolean capability columns and let the rules traverse it, the way
  `organization.memberships_via_organization.role` already does.

  **The unchanged-id shortcut inside that guard is off when the record's own
  `organization` moves.** Ids already present in `Original()` are skipped as
  "checked once" — but checked against the organization it *used to have*, so a
  record could be walked into another tenant keeping every relation it held.
  Unreachable through the record API (every org-scoped update rule freezes
  `organization`) and reachable through exactly the two paths the model-hook
  binding exists for. `TestRelationTenancy` pins it, and the reload in that
  subtest is load-bearing: a record built in memory has no `Original()`, so the
  shortcut never engages and the gap does not reproduce.
- **`?`-prefixed operators are row-correlated** on the same relation path, so
  `memberships_via_user.organization ?= X && memberships_via_user.role ?= "owner"`
  matches one membership row. Without `?`, the condition must hold for *all*
  related rows.
- **Credentials are protected by row scoping, not hidden fields — and that
  doctrine is about the RECORD API only.** `nats_users.creds_file` and
  `nebula_hosts.config_yaml` stay readable because the identity that owns them
  needs them (the browser's NATS connection and the admin download button). The
  read rules restrict *which rows* a caller sees. Do not add `hidden: true` to
  them — it breaks both and buys nothing.

  **It does NOT transfer to anything that holds COPIES of records**, and
  `audit_logs` is exactly that: one flat collection, no `organization` column,
  no row scoping, operator-read. pb-audit snapshots `Record.PublicExport()` —
  every field a collection does not mark hidden — so for as long as it
  snapshotted everything, the two correct decisions above combined into a
  plaintext archive of every credential the platform had ever minted. Verified,
  not inferred: creating one NATS identity wrote the full `.creds` file, NKEY
  seed included, into two audit rows. That archive also sat outside at-rest
  encryption (which covers the `seed`/`private_key` columns, the hidden ones,
  not the credential files derived from them) and never expired, retention
  being off by default.

  Closed by pb-audit v0.2.0: values are opt-in per collection and everything
  else records `changed_fields`, the NAMES of the fields that moved. The
  allowlist is `auditSnapshotCollections` in `main.go` and it is guarded by
  `audit_snapshots_test.go`, which reads `schema.json` and fails if a listed
  collection has an unhidden credential-bearing field. Adding `nats_users` to
  that list restores the archive in one line, with no symptom — which is why
  the guard exists rather than a comment.

  **The review question this yields:** not *"is this field hidden"* but
  **"does anything copy this record somewhere the row rules do not reach."**
- **At-rest encryption covers minting keys, not issued credentials, and that is
  deliberate.** `encryption_key` protects the operator seed, account seeds and
  signing keys, and the Nebula CA key. It does NOT protect `creds_file` or
  `config_yaml`, and cannot usefully: a `.creds` file *contains* the user seed
  (pb-nats `jwt.FormatUserConfig`), Nebula requires the host key inline, and
  `ui/src/stores/nats.ts` reads `creds_file` from the API to open the browser's
  own NATS connection — a browser can never hold the key. Encrypting the column
  would therefore force a decrypting route plus changes in the edge agent and
  five UI call sites, and `pb-nats`'s `EncryptField`/`DecryptField` live in
  `internal/`, so the platform cannot even call them without the library
  exporting a primitive. `migrations/schema_update_credential_scoping.go`
  reached the same conclusion for `hidden: true`.

  What that buys is worth knowing precisely: a stolen database with the key held
  elsewhere yields **no ability to mint new identities** and **every existing
  credential** — plus, in any database that predates the audit change above and
  has not had those rows cleared or aged out, **every historical one too**,
  including credentials rotated *because* they leaked. The NATS side survives
  that (the account JWT's revocation cutoff is by issue time and permanent);
  the Nebula side does not, since there is no CRL and an old `config_yaml`
  holds a live host key until the certificate expires or its fingerprint is
  blocklisted. The `UPDATE` that clears them is written out in
  `migrations/schema_update_audit_changed_fields.go`, deliberately as a comment
  rather than as migration code: that is the audit trail, and deleting it is a
  decision with a backup attached rather than something that should happen
  silently on deploy.

  Rotating the NATS side is central and cheap (`revoke`, which issues a new key
  pair -- `regenerate` re-signs the same seed, so a stolen `.creds` would keep
  working); rotating the
  Nebula side needs re-issue plus a blocklist entry in every peer plus
  redelivery. Don't "fix" this by encrypting the column; state the boundary
  and let disk encryption, encrypted backups and single-tenant deployments carry
  the at-rest threat. See SECURITY.md.
- **A gateway reads nothing in `nats_*` or `nebula_*` beyond its own identity.**
  Its agent gets the account JWT, the operator JWT, the `$SYS` account JWT and
  its own creds from `GET /api/me/leaf-config` (`hooks/leaf_config_routes.go`),
  which reads those records with the privileges of the app itself and returns ten
  named fields. Don't add a read branch to those collections to make some edge
  feature work — extend the route instead. The point is that the edge's blast
  radius is a fixed list rather than a consequence of rules that change for
  unrelated reasons.
- **A rule cannot express a single-field allowlist.** That is why self-service
  rotation is `POST /api/me/nats-creds/rotate` (`hooks/credential_routes.go`) and
  account key management is `POST /api/org/nats-account/keys`
  (`hooks/nats_account_routes.go`), rather than update-rule branches: the
  alternative is `:isset = false` on every other field, which silently opens up
  when a field is added. Both routes take no record id — the target is derived from
  the caller's own identity or active organization — and a `switch` maps each
  action to exactly one field, rejecting anything unrecognised.
- **A route that writes with `app.Save()` bypasses every API rule.** So each check
  the rules would have made has to be restated in the route. `POST /api/org/things`
  (`hooks/thing_routes.go`) is the worked example: organization comes from the
  caller's own record and never the body, and a `link`ed `nats_user`/`nebula_host`
  is verified to belong to that organization — without that second check the route
  would be a cross-tenant credential-theft path, because a Thing may read the
  credential of its own linked identity. Rules protect the CRUD endpoints, not
  yours.
- **A rule cannot express two authority levels for one operation.** Creating a
  Thing is a member action; attaching an identity to it is not. `things.createRule`
  approximates this by freezing `nats_user`/`nebula_host` in the member branch, but
  a *provisioning* endpoint that mints those records cannot be expressed as a
  create rule at all — hence `POST /api/org/things`, which role-checks per section.
  It also replaced three unguarded client calls whose partial failure orphaned a
  signed NATS credential, and which never sent `active`, so every Thing the console
  created was locked out by `things.authRule`. When provisioning spans collections,
  it belongs in one transaction on the server. PocketBase defers
  `*AfterCreateSuccess` hooks to commit (`core/db.go`, `txInfo.OnComplete`), so a
  rollback means pb-nats never signed or published either.
- **UI capability gates must match what the reader can READ, not just write.**
  Members hold inventory rights but cannot read `nats_users` (beyond their own row),
  `nebula_hosts`, `nebula_networks` or `nats_roles`. A view that expands those
  relations gets nothing back, and a `v-else` reading "No NATS user linked" then
  states something false about a Thing that *is* linked. The relation **id** on the
  `things` record is readable by any org member, so "linked but not visible to you"
  and "not linked" stay distinguishable without any rule change — three states, not
  two. Same rule as the twin markers: show what you actually know.
- **`nats_users.publish_permissions` is copied verbatim into the signed JWT**
  (pb-nats `internal/jwt/generator.go`). Write access to that collection is
  equivalent to granting NATS permissions, so it is owner/admin only.
- **An empty allow list grants NOTHING, since pb-nats v0.3.0.** Role and user
  lists are unioned per direction; if the union is empty the JWT gets
  `deny: [">"]` for that direction. Before v0.3.0 it got `>` — everything in
  the account — which was the "blank scope is a wildcard" bug in a fourth
  costume, inside the NATS trust chain this time. Two details that follow: an
  empty allow list in a JWT is NO restriction to nats-server, which is why the
  fix is a deny rather than `[]`; and `_INBOX.>` is never added for you, so a
  role that sends requests must list it on subscribe. Full access is written
  as `>` explicitly, so it is visible on the role. A credential minted before
  the upgrade keeps its `>` until revoked, since a reissue keeps the key pair.
- **An unset NATS limit is -1, and 0 is refused.** The same family the other
  way round: a value nobody set did not mean "no limit", it meant NONE.
  PocketBase number fields have no default, so a `nats_roles` record created
  without `max_subscriptions`, `max_data` and `max_payload` (the record API,
  `stone nats-role create` without the flags, a `stone apply` workspace that
  does not list them) stored 0. pb-nats copies the value into the user JWT and
  maps only a negative to `NoLimit`, which is correct NATS and documented. And
  nats-server v2.15 checks the subscription limit at CONNECT: a credential with
  `Subs: 0` is closed 20ms after it connects, before its first subscribe, so it
  cannot publish either. The browser reconnected every ~3s with nothing but
  console errors and one server log line. The console form avoided it only by
  pre-filling -1. `hooks/nats_limits.go` stores -1 for an unset role limit on
  every save and refuses a request that sends 0, with a field error saying
  what 0 would do. `schema_update_nats_role_limits.go` corrected the existing
  rows and re-signed their users.
  - **0 is refused rather than kept, because it expresses nothing.**
    "Publish-only" is not a `Subs: 0` credential, which cannot stay connected.
    It is an empty subscribe allow list, which signs deny `>` (the bullet above).
    Payload 0 closes the connection on the first non-empty message, and
    nats-server does not enforce a data limit at all. Rewriting an explicit 0
    to -1 would silently grant what the writer meant to withhold, so a request
    that asks for 0 gets a 400. Go code cannot be told apart from "unset" at
    the model layer, so there a 0 becomes -1.
  - **On a role, -1 means "the account limit applies", not unlimited.**
    nats-server applies the lower of the user limit and the account limit, and
    the account limits are operator-only, so the role default cannot widen a
    tenant's envelope. On an ACCOUNT, -1 really is unlimited. So accounts get
    only the refusal: no model rewrite, no backfill, JetStream 0 still allowed
    (it means JetStream off), and the refusal is edge-keyed, so a 0 already
    stored does not block later saves. The console limits form resubmits all
    six fields on every save.
  - **Two conventions meet here.** In `nats-server.conf`, 0 means "not set, use
    the server default", which for subscriptions is unlimited. In a JWT, 0
    means zero. `nats.default_limits` in `config.yaml` looks like the first and
    is copied into the second, so `stone-age` refuses to start with 0
    for `max_connections`, `max_subscriptions` or `max_payload`
    (`OrgProvisioningOptions.Validate`).
  - **The review question:** not "is this field set" but **"what does the zero
    value mean to whatever reads it."** The `real server` case of
    `TestNatsLimits` pins the server's side against a real nats-server. If an
    upgrade ever lets a `Subs: 0` connection stay up, that case fails and says
    to revisit this.
- **An `authRule` is checked at the auth endpoint only, never on an existing
  token.** PocketBase evaluates it in `apis.RecordAuthResponse`
  (`apis/record_helpers.go`), reached from `/auth-with-password` and friends —
  not in the middleware that loads a bearer token. `things` sets
  `authToken.duration` to 7 days, so `active = true` on its own would leave a
  deactivated device with a working session for a week. `hooks/active_flag.go`
  calls `RefreshTokenKey()` on the true→false flip, which invalidates every
  outstanding token at once. Any future "disable this identity" feature needs
  the same pairing; the rule alone is a latch, not a switch.
- **A flag is not a control unless something acts on it.** `nats_users.active`
  was once exactly that: read into pb-nats's model and consulted by nothing, while
  the console showed it as an editable checkbox beside a red/green badge, so an
  admin could "deactivate" a device that kept publishing. Since pb-nats v0.2.1 it
  **is** the control (`internal/sync/manager.go`, the `OnRecordUpdate` handler):
  true→false revokes the public key and deliberately reissues nothing;
  false→true mints a JWT issued after the cutoff, so it connects while the old
  file stays dead. Edge-triggered, which is safe here because pb-nats checks
  `hasPriorState` — see the `Record.Original()` bullet in `hooks/CLAUDE.md`.
  - **`revoke` is not suspend.** It is the "credentials leaked" button: a NEW key
    pair, the OLD public key onto the account's revocation list, and a working
    replacement `.creds` on the same record. The identity stays active. The
    console's Revoke modal said "the user will be marked inactive" until this was
    caught; any copy describing revoke as a way to take an identity out of
    service is wrong. `regenerate` is weaker still — it re-signs for the SAME
    seed, so a leaked file keeps working.
  - **Every path that can mint must respect the suspension.** A regenerate on an
    inactive record re-mints past the cutoff just as reactivation does, so
    `POST /api/me/nats-creds/rotate` refuses an inactive identity — without
    that, the self-service rotate button was a self-service un-suspend. A new
    route that sets `regenerate` needs the same check.
  - The checkbox stays gone from the NATS user form: for a device the lever is
    `things.active`, which exists only because `hooks/active_flag.go` gives it
    teeth — the flag, the token kill, and the NATS and Nebula suspension are one
    operation. Do not add a status field to a device without deciding what
    enforces it.
- **A device's real capability is its credentials, not its PocketBase session.**
  Anything that takes a Thing out of service has to reach
  `nats_users` **and** `nebula_hosts`, or it has only closed some of the doors.
  This was a live gap until the Nebula half was added: the console door and the
  NATS door closed while the overlay network stayed open until the certificate
  expired. `hooks/active_flag.go` mirrors the flag to both, and the two cascades
  are independent — a device may hold either identity, both, or neither, so
  neither may short-circuit the other.
- **Authority is the membership role. `organizations.owner` grants nothing.**
  The memberships and invites rules carried an `organization.owner =
  @request.auth.id` branch that never looked at the owner's membership, so the
  field was a standing grant: an owner an admin had demoted to `dashboard`, or
  removed, still managed invitations and members and could PATCH their own role
  back. owner and admin were meant to be identical in the rules, and the
  membership already says which is which. `hooks/org_membership.go` keeps the
  field honest instead: creating an organization, or an operator reassigning
  its owner, gives that user an owner membership. Do not add a rule branch on
  the field.
- **Only an owner or admin ever knows a Thing's password.** A Thing can read
  the credential of its own linked identity, so whoever can sign in as a Thing
  can read whatever identity an admin attaches to it later. Members create
  Things, so `POST /api/org/things` returns the generated password to owner and
  admin only, and a request hook replaces the password a member sends to the
  record API, which no rule can refuse because PocketBase requires one to
  create an auth record. The same line `things.manageRule` draws for setting
  one.
- **Run `./scripts/test-authz.sh` after any rule change** and add a check. Pair
  every "cannot" with a "can" on the same record, or a blanket deny passes.
