package hooks

import (
	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"

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
// and admins perform on their own overlay, neither of which a PocketBase API
// rule can express.
//
// Rotation is a route, not a branch on the operator-only `nebula_ca.updateRule`
// (whose comment points here), for the reason nats_account_routes.go is: a rule
// cannot say "this one field and nothing else", and a deny-list on the
// collection holding the mesh's trust anchor would silently re-open every
// field added later. The switch below IS the allowlist: one field, three values.
//
// The tenant may rotate because pb-nebula interlocks it: `prepare` is
// reversible, `commit` is idempotent, and `finish` refuses while any active host
// still holds a certificate from the outgoing CA. The dangerous part is the WAIT
// between steps, and that belongs to whoever operates the devices. The tenant
// still cannot write key material, set a validity, or name another org's CA:
// the CA comes from the caller's active organization, as in
// resolveOwnOrgNatsAccount.
//
// The audit is a route because it needs a Nebula certificate parsed, which the
// browser cannot do; pbnebula.HostCertNetworkIsStale is the one predicate for
// the /32 defect (certificates signed before pb-nebula v0.3.0). It is READ-ONLY
// with deliberately no bulk fix: re-signing moves the fingerprint that
// `pki.blocklist` revokes, so a sweep would rewrite every peer config. The fix
// is `renew`, one host at a time.
func RegisterNebulaRoutes(app *pocketbase.PocketBase, opts NebulaRoutesOptions) {
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.Router.POST("/api/org/nebula-ca/rotate", func(re *core.RequestEvent) error {
			var body struct {
				Step string `json:"step"`
			}
			if err := re.BindBody(&body); err != nil {
				return re.BadRequestError("invalid request body", err)
			}

			// The allowlist. pb-nebula validates the TRANSITION; this only
			// guarantees the record gets one of the three verbs.
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
				// pb-nebula's message names the out-of-order step or the host
				// blocking a finish, so surface it.
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
// Mirrors resolveOwnOrgNatsAccount, including not distinguishing "not a member"
// from "insufficient role". The organization comes from the authenticated
// record, never the request.
func resolveOwnOrgNebulaCA(re *core.RequestEvent, opts NebulaRoutesOptions) (*core.Record, error) {
	orgID, _, err := requireMembership(re, opts.MembershipCollection, rolesOrgManager)
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
// Inactive hosts are excluded: an inactive host is revoked by fingerprint, so
// re-signing it would silently un-revoke it, and badging one invites that.
// pb-nebula's own sweeps skip them too.
//
// A host whose certificate or network cannot be read is omitted, not reported:
// it is not known to be wrong, and the library's bootstrap audit logs it.
func auditOwnOrgHostCerts(re *core.RequestEvent, opts NebulaRoutesOptions) ([]string, error) {
	orgID, _, err := requireMembership(re, opts.MembershipCollection, rolesOrgManager)
	if err != nil {
		return nil, err
	}

	hosts, err := re.App.FindAllRecords(opts.NebulaHostCollection,
		dbx.HashExp{"organization": orgID, "active": true})
	if err != nil {
		return nil, re.InternalServerError("could not list Nebula hosts", err)
	}

	// One lookup per network rather than per host.
	cidrs := map[string]string{}

	// Never nil: a JSON null would read as "the audit did not run".
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

// Both routes here gate on rolesOrgManager via requireMembership
// (hooks/org_context.go), matching nebula_hosts' own read rules, which stop at
// owner/admin.
