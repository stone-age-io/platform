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
checker. `scripts/test-authz.sh` stands up a throwaway server and asserts 140
authorization behaviours against it, and CI runs it on every pull request. If you
find a hole, a failing check in that script is the most useful possible bug
report.

**Credentials live in rows, not behind hidden fields.** `nats_users.creds_file`
and `nebula_hosts.config_yaml` are deliberately readable: the identity that owns
them needs them (the browser opens its own NATS connection; an admin downloads a
host config). What protects them is *which rows* a caller can see. A report that
one of these fields is "exposed" needs to show a caller reading a row that is not
theirs.

## Out of scope

- Anything requiring PocketBase superuser access. A superuser bypasses every API
  rule by design; that account is the platform operator.
- Missing hardening in a self-hosted deployment that
  [`operations.md`](https://github.com/stone-age-io/platform-docs) tells you to
  configure — for example running without `nats.encryption_key` set, or exposing
  the admin panel to the internet.
- The public demo instance's data. Its traffic is simulated.
- Denial of service by an authenticated tenant against their own account limits.
