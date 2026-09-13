package hooks

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"

	pbnebula "github.com/skeeeon/pb-nebula"
)

// NebulaRoutesOptions names the collections involved, so a deployment that
// renamed them via config.yaml still works.
type NebulaRoutesOptions struct {
	NebulaCACollection      string
	NebulaNetworkCollection string
	NebulaHostCollection    string
	MembershipCollection    string
}

// RegisterNebulaRoutes adds the two Nebula operations an organization's owners
// and admins are meant to perform on their own overlay, neither of which a
// PocketBase API rule can express.
//
// WHY ROTATION IS A ROUTE. `nebula_ca.updateRule` is operator-only, and its
// comment has said since the authz hardening pass: "There is no tenant-triggered
// CA rotation today because there is no trigger field for one. Rolling a CA is an
// operator operation. If that changes, add a route rather than a branch here."
// pb-nebula v0.3.0 added the trigger field, so that is what this is.
//
// The reason it is not a rule branch is the reason nats_account_routes.go exists:
// a rule cannot say "this one field and nothing else". An owner/admin branch on
// `nebula_ca` would have to deny-list `certificate`, `private_key`,
// `next_certificate`, `next_private_key`, `previous_certificate`, `expires_at`,
// `curve`, `validity_years`, `name` and `organization` — and would silently
// re-open every field added to the collection afterwards. On the collection that
// holds the trust anchor for the tenant's entire mesh, that is the wrong shape.
// The switch below IS the allowlist: one field, three permitted values.
//
// WHY THE TENANT IS ALLOWED TO DO IT AT ALL. Rotation is bounded and interlocked
// on the library side. `prepare` is fully reversible — it publishes trust and
// touches neither issuance nor any host certificate. `commit` is idempotent and
// resumable. `finish` refuses while any active host still holds a certificate
// from the outgoing CA, which is the step that would otherwise take a fleet off
// the mesh silently. The dangerous part of rotating a CA is the WAIT in the
// middle, and the wait belongs to whoever operates the devices — which is the
// tenant, not us.
//
// WHAT THE TENANT STILL CANNOT DO: write the certificate or key material
// directly, set a validity, or aim any of this at another organization. The CA is
// derived from the caller's own active organization and never named in the
// request, exactly as in resolveOwnOrgNatsAccount.
//
// WHY THE AUDIT IS A ROUTE. pb-nebula signed host certificates at /32 until
// v0.3.0, which gives a host a route covering only itself — every existing
// certificate carries the wrong mask until it is re-signed. The library finds
// them and logs one line per host, which is the right surface for an operator
// tailing a server and the wrong one for a console that wants to badge the rows
// and put the fix beside them. Answering it needs a Nebula certificate parsed,
// so it cannot be done in the browser; `pbnebula.HostCertNetworkIsStale` is the
// one predicate, and this hands its verdict to the UI.
//
// The audit is READ-ONLY, and that restraint is the point. Re-signing moves a
// certificate's fingerprint, and a fingerprint is what `pki.blocklist` revokes,
// so a sweep would rewrite every peer config in the mesh. The fix is `renew` on
// one host at a time, which `nebula_hosts.updateRule` already permits an
// owner/admin to set — so there is deliberately no bulk endpoint here.
func RegisterNebulaRoutes(app *pocketbase.PocketBase, opts NebulaRoutesOptions) {
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.Router.POST("/api/org/nebula-ca/rotate", func(re *core.RequestEvent) error {
			var body struct {
				Step string `json:"step"`
			}
			if err := re.BindBody(&body); err != nil {
				return re.BadRequestError("invalid request body", err)
			}

			// The allowlist. pb-nebula validates the TRANSITION (commit before
			// prepare, finish with hosts still on the old CA) and returns a
			// message naming the blocker; this only guarantees that whatever
			// reaches the record is one of the three verbs.
			switch body.Step {
			case "prepare", "commit", "finish":
			default:
				return re.BadRequestError(`step must be one of "prepare", "commit", "finish"`, nil)
			}

			ca, err := resolveOwnOrgNebulaCA(re, opts)
			if err != nil {
				return err
			}

			ca.Set("rotate", body.Step)

			if err := re.App.Save(ca); err != nil {
				// pb-nebula's validator refuses an out-of-order step and names
				// the host blocking a finish. Surface it: that message is the
				// whole reason the interlock is usable.
				return re.BadRequestError(err.Error(), err)
			}

			return re.JSON(200, map[string]any{
				"applied":   body.Step,
				"nebula_ca": ca.Id,
			})
		}).Bind(apis.RequireAuth("users"))

		se.Router.GET("/api/org/nebula/cert-audit", func(re *core.RequestEvent) error {
			stale, err := auditOwnOrgHostCerts(re, opts)
			if err != nil {
				return err
			}
			return re.JSON(200, map[string]any{"stale": stale})
		}).Bind(apis.RequireAuth("users"))

		return se.Next()
	})
}

// resolveOwnOrgNebulaCA returns the Nebula CA of the caller's active
// organization, but only if the caller is an owner or admin there.
//
// Mirrors resolveOwnOrgNatsAccount exactly, including the refusal to distinguish
// "not a member" from "insufficient role". The organization comes from the
// authenticated record and never from the request, so this cannot be aimed at
// another tenant's CA.
func resolveOwnOrgNebulaCA(re *core.RequestEvent, opts NebulaRoutesOptions) (*core.Record, error) {
	orgID, err := requireOwnOrgManager(re, opts.MembershipCollection)
	if err != nil {
		return nil, err
	}

	ca, err := re.App.FindFirstRecordByFilter(
		opts.NebulaCACollection,
		"organization = {:org}",
		dbx.Params{"org": orgID},
	)
	if err != nil || ca == nil {
		return nil, re.NotFoundError("no Nebula CA for the active organization", nil)
	}
	return ca, nil
}

// auditOwnOrgHostCerts returns the ids of the caller's organization's hosts
// whose certificate no longer matches the network they belong to.
//
// INACTIVE HOSTS ARE EXCLUDED, and not as an optimization. An inactive host is
// revoked — its fingerprint sits in every peer's pki.blocklist — so re-signing
// it would publish a new fingerprint while the old certificate stayed valid and
// unblocklisted, silently un-revoking it. Badging one would invite exactly that.
// pb-nebula's own sweeps skip them for the same reason.
//
// A host whose certificate or network cannot be read is omitted rather than
// reported. The caller uses this to decide whether to offer a re-issue button,
// and a row that cannot be evaluated is not a row we know to be wrong; the
// library's bootstrap audit logs those cases for an operator.
func auditOwnOrgHostCerts(re *core.RequestEvent, opts NebulaRoutesOptions) ([]string, error) {
	orgID, err := requireOwnOrgManager(re, opts.MembershipCollection)
	if err != nil {
		return nil, err
	}

	hosts, err := re.App.FindAllRecords(opts.NebulaHostCollection,
		dbx.HashExp{"organization": orgID, "active": true})
	if err != nil {
		return nil, re.InternalServerError("could not list Nebula hosts", err)
	}

	// One lookup per network rather than per host: a 200-host network would
	// otherwise re-read the same CIDR 200 times.
	cidrs := map[string]string{}

	// Never nil: the caller reads this as "no host needs attention", and a JSON
	// null would read as "the audit did not run".
	stale := []string{}

	for _, host := range hosts {
		networkID := host.GetString("network_id")
		cidr, seen := cidrs[networkID]
		if !seen {
			network, err := re.App.FindRecordById(opts.NebulaNetworkCollection, networkID)
			if err == nil && network != nil {
				cidr = network.GetString("cidr_range")
			}
			cidrs[networkID] = cidr
		}
		if cidr == "" {
			continue
		}

		isStale, err := pbnebula.HostCertNetworkIsStale(
			host.GetString("certificate"), host.GetString("overlay_ip"), cidr)
		if err != nil || !isStale {
			continue
		}
		stale = append(stale, host.Id)
	}

	return stale, nil
}

// requireOwnOrgManager returns the caller's active organization id, but only if
// they are an owner or admin of it.
//
// Shared by both routes here so the two cannot drift apart on who is allowed to
// look: the audit tells you which hosts hold a wrong certificate, which is the
// same class of infrastructure detail the rotation lever acts on, and
// nebula_hosts' own read rules already stop at owner/admin.
func requireOwnOrgManager(re *core.RequestEvent, membershipCollection string) (string, error) {
	if re.Auth == nil || re.Auth.Collection().Name != "users" {
		return "", re.UnauthorizedError("user authentication required", nil)
	}

	orgID := re.Auth.GetString("current_organization")
	if orgID == "" {
		return "", re.BadRequestError("no active organization selected", nil)
	}

	membership, err := re.App.FindFirstRecordByFilter(
		membershipCollection,
		"user = {:user} && organization = {:org} && (role = 'owner' || role = 'admin')",
		dbx.Params{"user": re.Auth.Id, "org": orgID},
	)
	if err != nil || membership == nil {
		return "", re.ForbiddenError("owner or admin of the active organization required", nil)
	}

	return orgID, nil
}
