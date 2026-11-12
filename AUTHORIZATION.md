# Roles & Authorization — the long form

The role table, the five roles and a one-line version of every rule below are
in the root `CLAUDE.md`. This file is the reasoning a contributor needs to apply
each rule. Read it before you change an API rule in `schema.json`, write a route
or hook that saves with `app.Save()`, touch anything that holds a credential, or
gate a control on a `can.*` capability. How the rules look to a tenant is in
platform-docs `authorization.md`.

## Writing rules

- **Use an allowlist, never a deny-list.** `role ?!= "member"` is satisfied by
  `dashboard`, the least privileged role. Write `(role ?= "owner" || role ?=
  "admin")`, copied verbatim from a neighbouring rule.
- **Every write branch names its roles.** A branch that freezes *which fields*
  may be written still has to say *who* may write. A branch that names no role
  admits `dashboard`.
- **A blank scope is a wildcard.** In PocketBase `'' = ''` is true, and blank
  `organization` / `current_organization` values are ordinary states: an
  organization delete blanks its non-cascade, non-required relations, and
  leaving an org blanks `current_organization`. Every org-scoped read therefore
  requires a non-blank context **and** a correlated membership
  (`schema_update_tenancy_sentinel.go`); the membership clause is the half that
  holds, because `memberships.organization` is required and can never be blank.
  The review question: **what does this rule do when both sides are the zero
  value?**
- **Guard every path that writes `current_organization`**, on create as well as
  update. It is the read boundary. `users.createRule` refuses the field outright,
  and accept-invite fills it in.
- **No apostrophes in rule comments.** PocketBase tokenizes quotes before it
  strips `//` comments, so a later `''` opens a string nothing closes and the
  collection fails to import with `invalid quoted text`. Write "the
  organization", not "the org's".
- **`?`-prefixed operators are row-correlated**:
  `memberships_via_user.organization ?= X && memberships_via_user.role ?= "owner"`
  matches one membership row. Without `?`, the condition must hold for all rows.
- **Keep `dashboard`, the zero-authority role, as the denial probe** in
  `scripts/test-authz.sh`. A role with some authority, `member` or `viewer`,
  can pass a denial for the wrong reason.
- **A new read-only role needs no rule change.** A role that no write branch
  names is denied every write; if you must edit a rule to keep it out, that rule
  is a deny-list. The work is in the UI: a read capability and a write
  capability in `ui/src/stores/auth.ts`, and a gate on every write control.

## Reads, writes and the UI

- **Reads are org-scoped, not role-scoped, on purpose.** Every role in an org,
  `dashboard` included, can read the inventory collections through the API.
  `can.*` and the router are navigation. A read that must be a boundary is a
  branch in `schema.json`, with the upkeep that brings.
- **Gate a write control on the capability its rule needs**, not the one its
  screen needs. A Delete button on a screen members can open still needs
  `deleteRule`'s capability.
- **Gate on what the reader can READ.** A member cannot read `nats_users` (beyond
  its own row), `nebula_*` or `nats_roles`, so an expanded relation comes back
  empty. The relation id on the `things` record is readable, so show "linked,
  not visible to you" rather than "not linked".

## Hooks, routes and rules

- **Hooks enforce invariants, rules enforce permissions, and routes do what a
  rule cannot.** `hooks/relation_tenancy.go` is the one enforcement hook: no
  relation may point into another organization's records, which no rule can
  express, and without it an owner could mint a credential in another tenant's
  account. It is derived from the schema, binds the MODEL hooks, and applies to
  superusers. Do not grow it into a permission system: a hook cannot be a read
  boundary (list hooks run after the scoped query, so totals and pagination
  break), and two authoritative layers give two answers.
  - Its unchanged-id shortcut is off when the record's own `organization`
    moves. `TestRelationTenancy` pins it, and the reload in that subtest is
    load-bearing (see `Record.Original()` in `hooks/CLAUDE.md`).
- **A rule cannot allow one field and forbid the rest.** Hence
  `POST /api/me/nats-creds/rotate` (`credential_routes.go`) and
  `POST /api/org/nats-account/keys` (`nats_account_routes.go`). Both take no
  record id and map each action to one field.
- **A gateway reads nothing in `nats_*` or `nebula_*` beyond its own identity.**
  Its agent gets the JWTs and its creds from `GET /api/me/leaf-config`
  (`hooks/leaf_config_routes.go`), which returns ten named fields. Extend the
  route rather than add a read branch, so the edge's exposure stays a fixed list.
- **A route that writes with `app.Save()` bypasses every API rule**, so it
  restates each check. `POST /api/org/things` takes the organization from the
  caller, never the body, and verifies a linked `nats_user` / `nebula_host`
  belongs to it.
- **Provisioning that spans collections goes in one server transaction.**
  `POST /api/org/things` role-checks per section and runs in one transaction.
  PocketBase defers `*AfterCreateSuccess` to the commit, so a rollback means
  pb-nats never signed or published.

## Credentials

- **Credentials are protected by row scoping, not hidden fields.**
  `nats_users.creds_file` and `nebula_hosts.config_yaml` stay readable because
  their own identity needs them. Do not add `hidden: true`.
  - **The review question: does anything copy this record somewhere the row
    rules do not reach?** `audit_logs` is such a copy, so it records values only
    for `auditSnapshotCollections` (`main.go`), and `audit_snapshots_test.go`
    fails if a listed collection has an unhidden credential field.
  - **A view collection is a copy too.** `org_stats` (operator-read) holds only
    aggregates; `migrations/org_stats_test.go` pins its columns and
    `test-authz.sh` section 27 checks the read.
- **At-rest encryption covers minting keys, not issued credentials.** A
  `.creds` file contains the seed, Nebula needs the host key inline, and the
  browser reads `creds_file` to connect. Do not try to encrypt those columns;
  `SECURITY.md` says what a stolen database gives an attacker and why.
- **`nats_users.publish_permissions` is copied verbatim into the signed JWT**, so
  write access to it is granting NATS permissions: owner/admin only.
- **An empty NATS allow list grants nothing**: pb-nats signs `deny: [">"]` for
  that direction. Write full access as an explicit `>`, and list `_INBOX.>` on
  subscribe for a role that sends requests.
- **An unset NATS limit is -1, and 0 is refused** (`hooks/nats_limits.go`). In a
  JWT 0 means none, and a credential with 0 subscriptions is disconnected at
  CONNECT. A request that sends 0 gets a 400, because rewriting it to -1 would
  grant what the writer meant to withhold.
  - On a role, -1 means "the account limit applies"; on an account it is
    unlimited, so accounts get only the refusal, edge-keyed.
  - `nats.default_limits` uses the JWT convention, so `stone-age` refuses to
    start with a 0 there (`OrgProvisioningOptions.Validate`).
  - The `real server` case of `TestNatsLimits` checks the server's side.

## Taking things out of service

- **An `authRule` is checked only at the auth endpoint, never on an existing
  token.** Thing tokens last 7 days, so disabling an identity also needs
  `RefreshTokenKey()`, as `hooks/active_flag.go` does.
- **A flag is not a control unless something acts on it.** `nats_users.active`
  is the control (pb-nats: true→false revokes and reissues nothing; false→true
  mints past the cutoff). Do not add a status field without deciding what
  enforces it.
  - **`revoke` is not suspend.** It issues a new key pair and a working
    replacement, and the identity stays active. `regenerate` re-signs the same
    seed, so a leaked file keeps working.
  - **Every path that can mint must respect a suspension.** The rotate route
    refuses an inactive identity; a new route that sets `regenerate` needs the
    same check.
- **A device's capability is its credentials.** Taking a Thing out of service
  must reach `nats_users` **and** `nebula_hosts`, independently, since a device
  may hold either, both or neither. `hooks/active_flag.go` does both.

## Ownership and Things

- **Authority is `memberships.role`; `organizations.owner` grants nothing.**
  `hooks/org_membership.go` gives a new or reassigned owner an owner membership.
  Never add a rule branch on the field.
- **Only an owner or admin ever knows a Thing's password**, because whoever can
  sign in as a Thing can read the identity later attached to it.
  `POST /api/org/things` returns the password to owner/admin only, a request
  hook replaces a password a member sends, and `hooks/thing_routes.go` refuses
  `request-password-reset` on `things`. Ask the same of every email flow on any
  new auth collection members can create records in.

## Testing

Run `./scripts/test-authz.sh` after any rule change, and add a check. Pair every
"cannot" with a "can" on the same record, or a blanket deny passes.
