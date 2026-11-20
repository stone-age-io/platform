# Plan: device-held keys for Things

Status: **phases 1, 2, 4, 5 and 8 built** (pb-nats, pb-nebula, this repo),
2026-11-20, on pb-nats v0.5.0 and pb-nebula v0.8.0. Not built: phases 3 and 6
(agent, stone-cli), phase 7 (e2e).

What changed while building, against the text below:
- **A platform-held NATS create may carry `public_key` with its seed**, not
  never: pb-nats' own system user is created that way. The refusal is a
  `public_key` without a `seed` (`ErrPublicKeyWithoutSeed`), which was the
  actual hole. Nebula refuses `public_key` on a platform host outright.
- **The libraries skip their checks for their own saves.** A record saved in its
  create hook still has the blank pre-insert `Original()`, where every field
  looks changed. pb-nats gained a `saveInternal` like pb-nebula's. A consumer
  that saves a created object twice is handled by reading the stored row.
- **A Nebula re-sign in the same second is byte-identical** (same key, same
  validity, deterministic signature), so treating it as a replacement made
  `renew` straight after enrollment revoke the certificate the host still held.
  pb-nebula skips the replacement when nothing changed.
- **An inactive device NATS row gets no JWT, at creation too.** Platform rows
  keep pb-nats' documented "active governs transitions, not creation".

## The problem

The platform generates every NATS user seed and every Nebula host private key,
and keeps them. `creds_file` and `config_yaml` hold them in plaintext, on
purpose (`SECURITY.md`, "At-rest encryption covers minting keys, not issued
credentials"). So:

- a stolen `pb_data/data.db`, or a backup, holds a working credential for every
  device, and the Nebula half is the expensive one to clean up;
- `GET /api/me/leaf-config` sends the seed over the network every time the
  account JWT changes;
- an owner or admin can download any device's working credential.

NATS and Nebula were both built for the other way round: the device makes its
own key pair and sends only the public key. The platform signs a JWT (NATS) or a
certificate (Nebula) for that key. Neither has a CSR format, and neither needs
one. The manual versions are `nk -gen user` then a signed JWT, and
`nebula-cert keygen` then `nebula-cert sign -in-pub`.

**What this does not change:** the account signing keys and the Nebula CA key
stay on the platform. A compromise of a running server can still mint new
identities. This protects keys already issued, not the power to issue.

## The shape

One bool, `device_key`, on `nats_users` and on `nebula_hosts`. True means the
device made the key and the platform has never seen the private half.

- **`public_key` is the whole interface.** Empty means "waiting for the device".
  The libraries sign whatever key is there, and never store a private key, a
  seed or a `creds_file` for a `device_key` row.
- **Two ways to set it, and both end in the same library check:**
  - the device itself, through one new route, `POST /api/me/keys`;
  - an owner or admin, through the existing update rule, for a device with no
    agent. They paste what `nk -gen user -pubout` or `nebula-cert keygen` printed.
  - A create may also carry the key (stone-cli apply, a key generated at
    manufacture). Same checks.
- **Things only.** The console offers `device_key` on the Thing form and
  `POST /api/org/things`, nowhere else. Console users keep platform keys (see
  "Console users").
- **Existing rows are false**, which is today's behaviour, so there is no
  backfill.

### The invariants live in the libraries

Each library owns these, checked in its own update and create hooks. The
platform adds no rule term for them. Rules say who may write, and the libraries
say what a write may do, which is the split `AUTHORIZATION.md` already asks for.
A rule term would also cover only REST, not an `app.Save()`.

1. **`device_key` is set at create and never changes.** The library breaks if it
   flips: true to false leaves a row with no seed to build credentials from, and
   false to true keeps a stored seed while claiming the platform never saw it.
   Converting a row is out of scope (see "Decisions").
2. **`public_key` changes only from empty, and only on a `device_key` row.** On
   any save the library did not make itself, `public_key` must equal the
   original, unless the row is `device_key` and the original was empty. A create
   may set `public_key` only on a `device_key` row (built: a platform NATS row
   may carry it with its seed; see Status).
3. **The key must be valid** for its upstream: a user nkey for NATS, an X25519
   PEM for Nebula.

Refusals are typed errors in each library's `errors.go` (invalid key, already
enrolled, not a device-key row, and so on), returned as 400s. The platform route
tells them apart with `errors.Is`, never by matching on message text.

Invariant 2 also closes an existing hole. Today an owner or admin can overwrite
`public_key` on any `nats_users` row, so `revoke` then revokes the wrong key, and
a create that supplies `public_key` makes pb-nats skip key generation
(`generateUserKeys` returns early) and leaves a row with no seed and no JWT.

### NATS (pb-nats)

**One rule, one place.** `generateUserJWT` is the only function that signs a
user JWT, and every path reaches it: create, a key arriving, field edits,
`regenerate`, reactivation, `regenerateUsersWithRole` and
`regenerateUsersInAccount`. It gets one guard at the top:

> A `device_key` row with no `public_key`, or that is inactive, gets no JWT.
> That is not an error. A `device_key` row never gets a `creds_file`.

Without the guard:
- `GenerateCredsFile` refuses an empty seed, so every re-issue of an enrolled
  device-key row fails. A role edit would never reach the device, and a
  signing-key rotation would leave it on a JWT signed by a removed key.
- The update hook re-issues whenever `jwt` is empty (`manager.go`, the
  `GetString("jwt") == ""` term). A waiting row has no JWT, so every save would
  try to sign an empty key and the save would fail. The same applies after a
  device-key `revoke` empties `jwt`.
- `hasPriorState` reads the original `public_key`, so a waiting row skips the
  active-edge handling, and a key arriving on an inactive row would mint a JWT.
  The `active` term closes that inside the library, so no consumer needs a guard
  of its own.

The bulk loops (`regenerateUsersWithRole`, `regenerateUsersInAccount`) need no
change: the guard returns nil and the save is a no-op.

| | Today | `device_key` row |
|---|---|---|
| create, no key | mints a seed, JWT and `creds_file` | mints nothing |
| create with a key, or a key arrives | create: skips key generation and leaves no JWT. Refused from now on (invariant 2) | signs a JWT if active. No `creds_file`: the device builds its own |
| any re-issue (edit, role, signing key, `regenerate`, reactivate) | re-signs the same key | re-signs the same key, through the guard |
| `revoke` | revokes the key, mints a new pair | revokes the key, empties `public_key` and `jwt`. The device enrolls a new one. With no key yet: clears the flag, nothing else |
| deactivate | revokes the key | unchanged. With no key yet, there is nothing to revoke and no error |

**Where the invariant check runs.** `rotateUserCredentials` writes `public_key`
inside the same update hook, and pb-nats has no `saveInternal` marker like
pb-nebula's. So the check runs first in that hook, before the `revoke` branch,
and compares the incoming record with `Original()`. Every write the library
makes comes after it.

pb-nats also refuses a `public_key` write unless:
- it is not on the account's revocation list. A revocation is a cutoff time,
  so a revoked key sent again would get a fresh JWT that passes it;
- no other row holds it. A **partial unique index** on `public_key` where it is
  not empty does this, one line in `collections/manager.go`. Platform keys never
  collide, but a pasted key can land on two rows, and revoking one would cut off
  both, since a revocation is per key in the account.

**Never `device_key` and `bearer_token` together**, checked on every save, create
included, not only when a key is written. A bearer JWT never signs the server's
nonce, so a device key would protect nothing, and `bearer_token` can be switched
on long after the key arrives.

`public_key` joins `userJWTFieldsChanged`, so a key arriving mints the JWT.

### Nebula (pb-nebula)

`nebula_hosts` gains `device_key` and `public_key` (PEM, as `nebula-cert keygen`
writes it). pb-nebula issues CURVE25519 only (`cert/manager.go`), so X25519 is
the only key type to accept.

There are two choke points, not one: signing, and rendering the config.

**Signing: `generateHostCertAndConfig`.** Create, the renewal sweep, `renew`,
edits, reactivation and CA rotation all go through it.
- `GenerateHostCert` signs the stored public key instead of generating a pair,
  and re-signs the same key every time.
- A `device_key` host with no public key: no certificate, nothing stored, no
  error.
- The create hook skips a waiting host, as it already skips one with a
  certificate.
- `public_key` joins the fields that set `needsCertRegeneration`, so a key
  arriving signs. A key arriving on an inactive host is stored and not signed:
  the hook already refuses to re-sign an inactive host and downgrades the change
  to a config refresh, and reactivation re-issues from the record as it stands.
- The renewal sweep's "does the key pair with the certificate" check
  (`internal/sync/renewal.go`) is skipped. There is no private key to check.

**Rendering: `generateHostConfigWith`.** Signing renders a config, but so does the
peer fan-out (`regenerateNetworkHostConfigs`) and every config-only edit, and
neither goes near the signing function.
- A host with no certificate gets no `config_yaml`. Without this, every fan-out
  writes a config with no certificate for each waiting host.
- `pki.key` comes from `recordToHostModel` into the config generator. For a
  `device_key` host it is `/etc/nebula/host.key`, the path in Nebula's own
  example config. Nebula reads a path there as readily as an inline key. No
  setting for it: the agent overrides it, and someone running Nebula by hand
  puts the key there or edits one line.

**Replaced certificates** (pb-nebula v0.6.0, `revoked_certificates` and
`retired_certificates`) work the same for a `device_key` host. Renewal and
rotation retire the old certificate, while `renew`, reactivation and edits
revoke it. That holds even though the key does not change.

No unique index on `nebula_hosts.public_key`. Nebula revokes by certificate
fingerprint, so two hosts sharing a key do not revoke each other.

**No in-place key reset for Nebula in v1.** Nebula revokes certificates, by
fingerprint in `pki.blocklist`, never keys, so re-signing a leaked key changes
nothing. A leaked device key means: deactivate the host, which blocklists its
current and retired certificates, and attach a new host to the Thing. Owners
and admins can already re-point `things.nebula_host`.

### Platform (this repo)

- **Schema.** `schema.json` and `migrations/schema_update_device_keys.go`: the
  two fields. No rule change: the existing owner/admin update rule already says
  who may paste a key, and the libraries say what a paste may do.
- **`POST /api/me/keys`** (`hooks/key_routes.go`). Body: `nats_public_key` and
  `nebula_public_key`, both optional. Thing auth only, and no record id: the
  target is the caller's own `nats_user` / `nebula_host`, as in `leaf-config`
  and `nats-creds/rotate`. The route does only what a rule cannot:
  - find the caller's own identity (404 if it has none);
  - sending the key that is already enrolled is a 200, so a retry after a lost
    response needs no special case;
  - otherwise set `public_key`, `app.Save()`, and map the library's typed errors:
    already enrolled is a 409, anything else the library refuses is a 400.

  It repeats none of the library's checks (device-key row, active, empty): two
  copies of a check drift apart.
- **`guardLinkedIdentity`: no change.** A key arriving on an identity of a
  deactivated Thing mints nothing, because `mirrorActiveFlag` keeps the
  identity's `active` in step with the Thing's, and both libraries refuse to
  sign for an inactive identity. (Adding `public_key` to its trigger list would
  not work anyway: the triggers are read with `GetBool`, and a key string is
  false.)
- **`POST /api/org/things`**. `device_key`, and an optional `public_key`, in the
  `nats` and `nebula` auto sections. Link mode links whatever exists.
- **`GET /api/me/leaf-config`**. Adds `jwt`. For a `device_key` identity, `creds`
  is empty and the agent builds it, so the "no creds_file" 404 applies only to
  platform-key identities. Not enrolled yet: a 409 that names
  `POST /api/me/keys`. Update the comments on both routes that say leaf-config
  carries the seed.
- **Console.**
  - Thing create form: a "Key held by device" checkbox in each auto section.
  - NATS user and Nebula host detail views: a badge, "Waiting for the device"
    while there is no key, a paste field for owner/admin, and for NATS the JWT
    with a copy button in place of the `.creds` download.
  - The `revoke` confirmation on a `device_key` NATS identity says the device
    must enroll a new key before it reconnects, and that **if the device itself
    is compromised, reset the Thing's password too**. Whoever can sign in as the
    Thing enrolls the next key.
- **`scripts/test-authz.sh`**. A Thing enrolls its own key (can), enrolls a
  different key over it (cannot), and `dashboard` calls the route (cannot). An
  owner changes `device_key` (cannot, refused by the library) and pastes a key on
  an empty row (can). Bump `EXPECTED_CHECKS`.

### Agent

One rule: **if the platform has no key for me, make a new one.** Never send
again a key the platform dropped.

| The platform's `public_key` | The agent |
|---|---|
| empty | generates a pair, writes it (0600), then calls `POST /api/me/keys` |
| the same as mine | runs |
| something else | stops, and says an admin enrolled a different key |

- NATS: the seed in a file beside `nats.auth.creds_file`. The agent builds the
  `.creds` from the record's `jwt` with `jwt.FormatUserConfig` whenever the JWT
  changes.
- Nebula: the key in a file beside `nebula.cache_file`. The agent sets
  `pki.key` to it before it loads the config.
- Write the key first, then enroll. A crash in between leaves a key nobody
  enrolled, and the next start finds the platform empty and makes another.
- After a `revoke`, the agent learns through the poll it already runs: the
  account JWT changes, `leaf-config` answers 409, and the agent enrolls.

### stone-cli

`device_key` and `public_key` on the `nats_users` and `nebula_hosts` entities
(`cmd/entity.go`) and in `thing create`, so pull and apply carry them. An apply
that creates a `device_key` identity with its key goes through invariant 2 like
any other create. `stone creds` on a `device_key` identity says the device holds
the key, instead of printing an empty file.

## Console users

Not in scope, and not planned. The browser opens NATS with `creds_file`
(`ui/src/stores/nats.ts`). To hold its own key it would keep the seed in
IndexedDB as raw bytes, because nats-core's `Authenticator` is synchronous and a
non-extractable WebCrypto key signs asynchronously. An XSS reads IndexedDB as
easily as it reads the API. And a `nats_users` row has one key, so a second
browser would replace the first. If a console credential should live less long,
shorten `jwt_expires_at`.

## Decisions, and why

- **A field, not "the seed is empty".** A revoked device key and a platform row
  whose keys are not generated yet would look the same.
- **Invariants in the libraries, permissions in the rules.** The libraries break
  if `device_key` flips or `public_key` moves, so they refuse it, for every
  consumer and every write path. The rule keeps saying only who.
- **One guard per choke point.** pb-nats has one (`generateUserJWT`), pb-nebula
  two (signing and rendering). The plan names the function, not the events,
  because the events outnumber what a table remembers.
- **Frozen at create.** Converting a row means a `revoke` whose result depends
  on the mode. Not worth it until someone has a fleet to convert. When they do:
  NATS is `device_key` and `revoke` in one save, and Nebula is a new host.
- **One route for both keys.** One enrollment call per device, one file to read.
- **No proof of possession.** Nebula host keys are X25519 and cannot sign. On
  NATS, enrolling a key you do not hold gets you a JWT you cannot use. Trust in
  enrollment is the Thing's sign-in, as it is today for reading `creds_file`.
- **A fixed Nebula key path.** One fewer setting. The agent overrides it anyway.

## Deliberately not done

- Converting existing identities.
- Console users.
- Rotation the device starts itself. On NATS an owner or admin uses `revoke`; on
  Nebula it is a new host.
- An in-place Nebula key reset. pb-nebula v0.6.0 makes it possible: a key
  change would revoke the current certificate and every retired one, then sign
  the new key. Add it when someone needs it.
- Hardware-backed keys. The agent can add a TPM later behind the same route.
- A key-history table, a CSR format, a proof-of-possession signature.
- A unique index on Nebula public keys (see the Nebula section).

## Phases

Each phase ships on its own. NATS goes first: it is smaller, and it stops the
seed crossing the network.

1. **pb-nats.** The fields, the three invariants and the bearer check, the guard
   in `generateUserJWT`, the partial unique index, typed errors. Tests for each
   row of the table, plus: an edit to a waiting row saves; a role edit and a
   signing-key rotation re-issue an enrolled device-key row; a key on an inactive
   row mints nothing; a platform-key create with `public_key` is refused.
   Release.
2. **Platform, NATS half.** Field, migration, the route with `nats_public_key`,
   the Thing route, `leaf-config`, console, `test-authz.sh`.
3. **Agent and stone-cli, NATS half.**
4. **pb-nebula.** The section above, with tests, including: a fan-out over a
   network with a waiting host writes it no config; a key arriving on an
   inactive host signs on reactivation. Release.
5. **Platform, Nebula half.** `nebula_public_key`, and the rest as in phase 2.
6. **Agent and stone-cli, Nebula half.**
7. **e2e.** Run two existing tests a second time with a device key: a Thing's
   NATS connect, and the host config through `nebula.Main(configTest=true)`.
   This is not a test for a bug that has not happened. It is the same tests with
   the second key source, and the last two credential bugs passed every unit
   test.
8. **Docs.** `SECURITY.md`: a stolen database holds no usable credential for a
   `device_key` identity. `AUTHORIZATION.md`: what `revoke` does on a
   `device_key` row, the route, and that the invariants are library-enforced. A
   feature line in the root `CLAUDE.md`, the long note in `hooks/CLAUDE.md`.

Rough size: phases 1–3 about a week, 4–6 about a week, 7–8 a few days.

## Fixed while planning

pb-nebula used to blocklist only the current certificate of an inactive host,
so a certificate replaced by a re-issue stayed valid until it expired. That
meant a host kept its old groups, and reactivation un-revoked a leaked
certificate. pb-nebula v0.6.0 keeps replaced certificates and lists them, and
the platform has used it since `551a630`. The Nebula phases build on it.
