# The inventory feed (ADR 0004)

Long-term readings carry only `thing`, a code that never changes. Where a Thing
is, what it is called and what type it is live in the platform, and they change.
This directory gets them into the TSDB as two **info series**, so a dashboard can
join readings against the inventory as it is *now*:

| Series | Labels |
|---|---|
| `stone_thing_info` | `thing`, `name`, `thing_type`, `location`, `location_path` |
| `stone_location_info` | `location`, `name`, `location_type`, `location_path` |

The decision and the reasons are ADR 0004 in platform-docs. This file is how to
run it.

```
nats-auth-manager ──► KV tokens.pocketbase           signs in as a viewer, keeps the token fresh
rule-router, every minute:
    GET /api/collections/things/records ──► inventory.things
    GET /api/collections/locations/records ──► inventory.locations
rule-router, on each: forEach item, merge {"info": 1}
    ──► inventory.thing.<code> / inventory.location.<code>
Telegraf ──► VictoriaMetrics                           stone_thing_info, stone_location_info
```

No platform route, no new credential type, no state. The standard list API, a
viewer login, and the three tools the tenant already runs.

## Files

```
auth-manager.yaml          nats-auth-manager: the viewer login, the token into KV
rule-router.yaml           rule-router config: scheduler + router, KV, core publish
northwind/inventory.yaml   the rules: two polls, two fan-outs, two truncation guards
```

The Telegraf half is in [../telegraf/northwind.conf](../telegraf/northwind.conf),
beside the readings it joins against.

## Run it

One set of processes per organization, for the reason every demo here is one
process per organization: the NATS account is the tenant, and the token bucket
lives inside it. Northwind below.

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

**`features.router: true` or nothing happens.** The polls are scheduler rules,
but the fan-out and the guards are core-NATS triggers, served by the router. With
it off, the polls publish to a subject nobody reads and the log says nothing.

**`forEach.maxIterations` defaults to 100, and it truncates silently.**
rule-router processes the first hundred items and drops the rest, with no
error. `rule-router.yaml` raises it to 1000, PocketBase's `perPage` ceiling. The
guard rules cannot catch this one, because the poll itself was complete.

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

## The token is readable, and that is why it is a viewer's

The bucket holds a live PocketBase token. Anything in the account that can read
`tokens` can use it. In this demo that includes the `gateway`, `application` and
`console-readonly` roles. `gateway` can also overwrite it, which breaks the feed
until the next refresh.

A subject deny list does not close this, and so the demo does not add one. A role
holding `$JS.API.>` can create a stream of its own that sources `KV_tokens`, and
subject permissions do not apply to sourcing. The roles without `$JS.API.>`
(`device`) cannot read any KV at all.

What bounds it is what the token can do. It belongs to a **viewer**: it lists the
organization's inventory and changes nothing. A viewer or dashboard session can
already read that inventory, so for them it adds nothing. For a gateway or
application credential it adds read access to the inventory. Never point
nats-auth-manager at a login that can write.
