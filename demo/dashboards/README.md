# Demo dashboards

`stone-age demo-seed` fills the console with inventory, and the rules in
[../rules](../rules/README.md) put traffic on the bus. Neither creates a
dashboard, so the ones here are the third piece: console exports, ready to
import.

```
northwind-cold-chain.json    Northwind Traders: probes, reefers, sites, WMS feed, heartbeats, twin
```

## Importing

Sign in as a member of the organization, open **Dashboard**, open the dashboard
sidebar, and choose **Import** on the Local tab. The file is the console's own
export format (`version` plus a `dashboards` array), so it imports as a local
dashboard with fresh ids. Share it from there if other members should see it.

## What it needs

Every widget subscribes to a subject that `../rules/northwind/telemetry.yaml`
publishes, or reads a twin key those rules write. Run that rule-router first:

```bash
RR_NATS_CREDSFILE=/path/to/console-dana.creds \
  rule-router --config demo/rules/rule-router.yaml --rules demo/rules/northwind
```

All subscriptions are core NATS (`useJetStream: false`), matching the rules'
`mode: core`: the seed creates no streams, so there is nothing to replay. Widgets
start empty and fill as messages arrive. The freezer chart's three-minute window
is full about three minutes after the page loads.

Change a subject in the rules and this file goes stale with it. The probes, the
reefers, the gateways, `RULES-COLD-01` and the twin keys `thing.TP-001.temp` and
`thing.REEFER-1180.setpoint` are the ones it reads.
