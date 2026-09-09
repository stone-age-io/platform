# Security policy

## Reporting a vulnerability

Use GitHub's private vulnerability reporting: open the
[Security tab](https://github.com/stone-age-io/platform/security) and choose
**Report a vulnerability**. That keeps the report private until there is a fix,
and it needs no separate mailbox.

Please do not open a public issue for anything that lets one tenant read or
write another tenant's data, or that lets any identity obtain NATS or Nebula
credentials it should not have.

**Response expectations.** This is a one-maintainer project, and the maintainer
also does field installation work — some weeks are spent on a ladder. A realistic
commitment rather than a flattering one:

| | |
|---|---|
| Acknowledgement | within 5 working days |
| First assessment | within 15 working days |
| Fix for a confirmed tenant-isolation or credential issue | prioritised over all other work |

If you have heard nothing after two weeks, ping the report again — it means the
notification was missed, not that it was judged unimportant.

## Supported versions

Pre-1.0. Only the most recent tag receives fixes; there are no maintenance
branches. Treat minor versions as potentially breaking and pin what you deploy.

## What to look at first

Two places carry nearly all of the security-relevant surface, and both are worth
knowing before you go hunting.

**`schema.json` is the enforcement layer.** The PocketBase API rules in that file
are the *only* thing enforcing tenant isolation and privilege boundaries. The
supporting libraries (`pb-nats`, `pb-nebula`, `pb-tenancy`, `pb-audit`) contain no
tenancy logic at all — they never reference `organization`. The console's
capability map (`ui/src/stores/auth.ts`) is navigation convenience, not a
boundary: a role that cannot see a screen can still call the API. So a finding
phrased as "the UI lets role X do Y" is a UI bug; a finding phrased as "`curl` as
role X does Y" is a security bug.

Those rules are plain strings in a JSON file, with no compiler and no type
checker. `scripts/test-authz.sh` stands up a throwaway server and asserts 175
authorization behaviours against it, and CI runs it on every pull request. If you
find a hole, a failing check in that script is the most useful possible bug
report.

**Credentials live in rows, not behind hidden fields.** `nats_users.creds_file`
and `nebula_hosts.config_yaml` are deliberately readable: the identity that owns
them needs them (the browser opens its own NATS connection; an admin downloads a
host config). What protects them is *which rows* a caller can see. A report that
one of these fields is "exposed" needs to show a caller reading a row that is not
theirs.

**At-rest encryption covers minting keys, not issued credentials.** With
`nats.encryption_key` and `nebula.encryption_key` set, the operator seed, the
account seeds and signing keys, and the Nebula CA private key are encrypted in
the database. `nats_users.creds_file` and `nebula_hosts.config_yaml` are **not**,
and cannot usefully be: a `.creds` file *contains* the user seed by construction
(`jwt.FormatUserConfig`), and Nebula's PKI requires the host key inline. The
browser also reads `creds_file` straight from the API to open its own NATS
connection, and a browser can never hold the encryption key — so encrypting that
column would force every read through a server-side decrypting route.

So the boundary a stolen `pb_data/data.db` runs into, with the key held
separately, is this: the attacker **cannot mint new identities** — no operator
seed, no account signing keys, no CA key — but **does** obtain every existing
NATS credential and every overlay host config. That is the line the feature
actually defends, and it is worth stating because the two halves have very
different remediation costs:

- **NATS**: rotate. Set `regenerate` on each `nats_users` row; the revocation
  cutoff in the account JWT is permanent, so the old `.creds` stays dead.
  Central, scriptable, and delivery already exists (`GET /api/leaf/bootstrap`,
  the console download).
- **Nebula**: re-issue *and* blocklist *and* redeliver. There is no CRL, so a
  revoked certificate is a fingerprint in every peer's `pki.blocklist`, applied
  when that peer's config is redeployed. This is the expensive half.

Treat a `pb_data` compromise as requiring both. The controls that actually
address it are operational rather than cryptographic — full-disk encryption on
the host, encrypted backups, and for tenants who need the blast radius to be
zero by construction, a dedicated single-tenant deployment with its own
database, operator seed and CA.

## Out of scope

- Anything requiring PocketBase superuser access. A superuser bypasses every API
  rule by design; that account is the platform operator.
- Missing hardening in a self-hosted deployment that
  [`operations.md`](https://github.com/stone-age-io/platform-docs) tells you to
  configure — for example running without `nats.encryption_key` set, or exposing
  the admin panel to the internet.
- The public demo instance's data. Its traffic is simulated.
- Denial of service by an authenticated tenant against their own account limits.
