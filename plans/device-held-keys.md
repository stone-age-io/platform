# Plan: device-held keys for Things

Status: **proposed**. Nothing built.

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

- **Set at create, never changed.** Both update rules get
  `@request.body.device_key:changed = false`. Existing rows are false, which is
  today's behaviour, so there is no backfill.
- **`public_key` is the whole interface.** Empty means "waiting for the device".
  The libraries sign whatever key is there, and never store a private key, a
  seed or a `creds_file` for a `device_key` row.
- **Write-once.** `public_key` may change only from empty. On NATS, `revoke` is
  what empties it again. On Nebula nothing does (see below).
- **Two ways to set it, and both end in the same library check:**
  - the device itself, through one new route, `POST /api/me/keys`;
  - an owner or admin, through the existing update rule, for a device with no
    agent. They paste what `nk -gen user -pubout` or `nebula-cert keygen` printed.
- **Things only.** The console offers `device_key` on the Thing form and
  `POST /api/org/things`, nowhere else. Console users keep platform keys (see
  "Console users").

### NATS (pb-nats)

| | Today | `device_key` row |
|---|---|---|
| create | mints a seed, JWT and `creds_file` | mints nothing. No JWT until a key arrives |
| a key arrives | n/a | signs a JWT. No `creds_file`: the device builds its own |
| `regenerate` | re-signs the same key | unchanged |
| `revoke` | revokes the key, mints a new pair | revokes the key, empties `public_key` and `jwt`. The device enrolls a new one |
| deactivate | revokes the key | unchanged. With no key yet, there is nothing to revoke and no error |

pb-nats refuses a `public_key` write unless:
- it is a valid user nkey (`nkeys.IsValidPublicUserKey`);
- it is not on the account's revocation list. A revocation is a cutoff time,
  so a revoked key sent again would get a fresh JWT that passes it;
- the row is `device_key` and the old value was empty;
- the row is not `bearer_token`. A bearer JWT never signs the server's nonce, so
  a device key would protect nothing.

`public_key` joins `userJWTFieldsChanged`, so a key arriving mints the JWT.

Side effect: today an owner or admin can overwrite `public_key` on any row, and
`revoke` then revokes the wrong key. The write-once check closes that too.

### Nebula (pb-nebula)

`nebula_hosts` gains `device_key` and `public_key` (PEM, as `nebula-cert keygen`
writes it).

- `GenerateHostCert` signs the stored public key instead of generating a pair.
  Create, the renewal sweep, `renew` and CA rotation all go through
  `generateHostCertAndConfig`, so one branch covers all four, and each re-signs
  the same key.
- No public key yet: no certificate and no `config_yaml`.
- `config_yaml` says `pki.key: /etc/nebula/host.key`, the path in Nebula's own
  example config. Nebula reads a path there as readily as an inline key. No
  setting for it: the agent overrides it, and someone running Nebula by hand
  puts the key there or edits one line.
- The renewal sweep's "does the key pair with the certificate" check
  (`internal/sync/renewal.go`) is skipped. There is no private key to check.
- pb-nebula refuses a `public_key` write unless it is an X25519 public key, the
  row is `device_key`, and the old value was empty.

- Replaced certificates (pb-nebula v0.6.0, `revoked_certificates` and
  `retired_certificates`) work the same for a `device_key` host: renewal and
  rotation retire the old certificate, while `renew`, reactivation and edits
  revoke it. That holds even though the key does not change.

**No in-place key reset for Nebula in v1.** Nebula revokes certificates, by
fingerprint in `pki.blocklist`, never keys, so re-signing a leaked key changes
nothing. A leaked device key means: deactivate the host, which blocklists its
current and retired certificates, and attach a new host to the Thing. Owners
and admins can already re-point `things.nebula_host`.

### Platform (this repo)

- **Schema.** `schema.json` and `migrations/schema_update_device_keys.go`: the
  two fields, and the `device_key:changed = false` term on both update rules.
- **`POST /api/me/keys`** (`hooks/key_routes.go`). Body: `nats_public_key` and
  `nebula_public_key`, both optional. Thing auth only, and no record id: the
  target is the caller's own `nats_user` / `nebula_host`, as in `leaf-config`
  and `nats-creds/rotate`. For each key sent, the route checks that the identity
  exists, is `device_key`, is active and has no key yet, then saves with
  `app.Save()` and lets the library check the key. Sending the key that is
  already enrolled is a 200, so a retry after a lost response needs no special
  case. A different key is a 409.
- **`guardLinkedIdentity`** (`hooks/active_flag.go`). `public_key` joins both
  trigger lists. A key arriving mints a credential, and every path that can mint
  must respect a deactivated Thing.
- **`POST /api/org/things`**. `device_key` in the `nats` and `nebula` auto
  sections. Link mode links whatever exists.
- **`GET /api/me/leaf-config`**. Adds `jwt`. For a `device_key` identity, `creds`
  is empty and the agent builds it. Not enrolled yet: a 409 that names
  `POST /api/me/keys`.
- **Console.**
  - Thing create form: a "Key held by device" checkbox in each auto section.
  - NATS user and Nebula host detail views: a badge, "Waiting for the device"
    while there is no key, a paste field for owner/admin, and for NATS the JWT
    with a copy button in place of the `.creds` download.
  - The `revoke` confirmation on a `device_key` NATS identity says the device
    must enroll a new key before it reconnects.
- **`scripts/test-authz.sh`**. A Thing enrolls its own key (can), enrolls a
  different key over it (cannot), and `dashboard` calls the route (cannot). An
  owner changes `device_key` (cannot) and pastes a key on an empty row (can).
  Bump `EXPECTED_CHECKS`.

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

### stone-cli

`device_key` and `public_key` on the `nats_users` and `nebula_hosts` entities
(`cmd/entity.go`) and in `thing create`, so pull and apply carry them. `stone
creds` on a `device_key` identity says the device holds the key, instead of
printing an empty file.

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

## Phases

Each phase ships on its own. NATS goes first: it is smaller, and it stops the
seed crossing the network.

1. **pb-nats.** The table and checks above, with tests. Release.
2. **Platform, NATS half.** Field, migration, rule term, the route with
   `nats_public_key`, `guardLinkedIdentity`, the Thing route, `leaf-config`,
   console, `test-authz.sh`.
3. **Agent and stone-cli, NATS half.**
4. **pb-nebula.** The section above, with tests. Release.
5. **Platform, Nebula half.** `nebula_public_key`, and the rest as in phase 2.
6. **Agent and stone-cli, Nebula half.**
7. **e2e.** Run two existing tests a second time with a device key: a Thing's
   NATS connect, and the host config through `nebula.Main(configTest=true)`.
   This is not a test for a bug that has not happened. It is the same tests with
   the second key source, and the last two credential bugs passed every unit
   test.
8. **Docs.** `SECURITY.md`: a stolen database holds no usable credential for a
   `device_key` identity. `AUTHORIZATION.md`: what `revoke` does on a
   `device_key` row, and the route. A feature line in the root `CLAUDE.md`, the
   long note in `hooks/CLAUDE.md`.

Rough size: phases 1–3 about a week, 4–6 about a week, 7–8 a few days.

## Fixed while planning

pb-nebula used to blocklist only the current certificate of an inactive host,
so a certificate replaced by a re-issue stayed valid until it expired. That
meant a host kept its old groups, and reactivation un-revoked a leaked
certificate. pb-nebula v0.6.0 keeps replaced certificates and lists them, and
the platform has used it since `551a630`. The Nebula phases build on it.
