# The inventory feed (ADR 0004)

Gets each Thing's and Location's current name, type and place into the TSDB as
two info series, `stone_thing_info` and `stone_location_info`, so dashboards
can join readings against the inventory as it is now. nats-auth-manager keeps a
viewer's token in KV, rule-router polls the list API every minute and fans each
record out to `inventory.thing.<code>` / `inventory.location.<code>`, and
Telegraf writes the series. What it is for, and its limits:
[Observability §5](https://github.com/stone-age-io/platform-docs/blob/main/docs/observability.md)
and ADR 0004.

## Files

```
auth-manager.yaml          nats-auth-manager: the viewer login, the token into KV
rule-router.yaml           rule-router config: scheduler + router, KV, core publish
northwind/inventory.yaml   the rules: two polls, two fan-outs, two truncation guards
```

The Telegraf half is in [../telegraf/northwind.conf](../telegraf/northwind.conf),
beside the readings it joins against.

## Run it

One set of processes per organization, because the token bucket lives inside
the account. Northwind below.

**1. The bucket.** nats-auth-manager opens `tokens` and refuses to start without
it, rather than create one with defaults you did not choose:

```bash
nats kv add tokens --history=1 --storage=file --creds console-dana.creds
```

**2. The token.** `inventory-feed@northwind.example` is seeded by `demo-seed` as a
Northwind viewer, with Northwind as its current organization. Every list rule
scopes by that, which is why the poll URLs carry no organization filter:

```bash
PB_URL=http://127.0.0.1:8090 PB_EMAIL=inventory-feed@northwind.example PB_PASSWORD=demo1234 nats-auth-manager --config demo/inventory/auth-manager.yaml
```

**3. The feed.**

```bash
PB_URL=http://127.0.0.1:8090 rule-router --config demo/inventory/rule-router.yaml --rules demo/inventory/northwind
```

Both configs point at `console-dana.creds`; set `credsFile` to wherever you saved
it (demo/rules/README.md, "Which credential"). Single quotes on Windows paths.

**4. Telegraf**, as in [../telegraf/README.md](../telegraf/README.md). The
inventory inputs are already in `northwind.conf`.

## What you should see

On the seeded Northwind estate, each minute publishes 72 messages: the two poll
responses, 59 Things and 11 Locations.

```bash
nats sub 'inventory.>' --creds console-dana.creds
```

```
[#22] Received on "inventory.thing.TP-003"
{"code":"TP-003","expand":{"location":{"code":"KC-DC1-CH1","path":"/KC-DC1/KC-DC1-CH1/"},"type":{"code":"temp-probe"}},"info":1,"name":"Temperature Probe 003"}

[#4] Received on "inventory.location.KC-DC1-FZ1"
{"code":"KC-DC1-FZ1","expand":{"type":{"code":"zone"}},"info":1,"name":"Freezer Zone 1","path":"/KC-DC1/KC-DC1-FZ1/"}
```

Nothing arrives on `inventory.truncated`. Something will, the first time an
organization holds more records than one poll returns.

That much was run against `serve --nats` with a fresh `demo-seed`. The Telegraf
and VictoriaMetrics half has not been run yet; see
[../telegraf/README.md](../telegraf/README.md#where-things-are).

## Things that will bite

**`PB_URL` unset fails at load, loudly.** The rules refuse to load with `HTTP
action URL must start with http:// or https://`. That is the right failure; the
message just does not mention the variable.

**`forEach.maxIterations` defaults to 100.** A longer list fails the whole
fan-out for that poll and logs `forEach array exceeds limit`.
`rule-router.yaml` raises it to 1000, PocketBase's `perPage` ceiling.

**One poll is at most 1000 records.** Past that, `totalItems` exceeds `perPage`
and the guard publishes

```json
{"collection": "things", "totalItems": 1203, "perPage": 1000}
```

on `inventory.truncated`. The fix is paging, which this demo does not do until an
organization needs it.

**The first connection after `serve --nats` can be refused.** `Authorization
Violation` in the client, `Account fetch failed: fetching jwt timed out` in the
server log. The account JWT arrives a moment after the server starts. Try again.

## The token is a viewer's on purpose

Anything in the account that can read `tokens` can use the token: here the
`gateway`, `application` and `console-readonly` roles, and `gateway` and
`application` can overwrite it, which breaks the feed until the next refresh. A
subject deny does not close this, since a role with `$JS.API.>` can source
`KV_tokens` into a stream of its own. What bounds it is the login: a viewer
lists inventory and changes nothing. **Never point nats-auth-manager at a login
that can write.**
