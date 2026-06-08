package hooks

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// leafNodeRoleName is the per-organization NATS role minted for edge leaf nodes.
// A leaf node runs a single NATS identity that serves the leaf-remote connection,
// rule-router, and leaf-sync, so the role is broad within the account (allow-all
// publish/subscribe, which also covers $JS.API.> and $KV.>). The account boundary
// is the tenant isolation; the edge box is the trust boundary.
const leafNodeRoleName = "leaf-node"

// LeafNodeProvisioningOptions captures the collections involved in provisioning a
// leaf node's NATS identity. Kept as a plain struct so the caller (main.go) owns
// the config-loading story, mirroring OrgProvisioningOptions.
type LeafNodeProvisioningOptions struct {
	LeafNodeCollection    string
	NatsAccountCollection string
	NatsUserCollection    string
	NatsRoleCollection    string
}

// RegisterLeafNodeProvisioning attaches a hook that auto-provisions a single NATS
// user for every newly-created leaf node, linking it back onto the leaf node
// record. A leaf node is "a special thing": one nats_user, minted server-side so
// the UI form and any tooling share one provisioning path.
//
// Mirrors RegisterOrgProvisioning: sequential saves, errors logged (not fatal) so
// a provisioning hiccup never rejects the already-created leaf node record — it
// can be retried by re-saving the record once the cause is fixed.
func RegisterLeafNodeProvisioning(app *pocketbase.PocketBase, opts LeafNodeProvisioningOptions) {
	app.OnRecordAfterCreateSuccess(opts.LeafNodeCollection).BindFunc(func(e *core.RecordEvent) error {
		leaf := e.Record

		// Idempotency: skip if a NATS user is already linked.
		if leaf.GetString("nats_user") != "" {
			return e.Next()
		}

		orgID := leaf.GetString("organization")
		code := leaf.GetString("code")
		if orgID == "" || code == "" {
			log.Printf("⚠️ Leaf node '%s' missing organization/code; skipping NATS provisioning", leaf.Id)
			return e.Next()
		}

		log.Printf("🔗 Leaf node '%s' created, provisioning NATS identity...", code)

		account, err := e.App.FindFirstRecordByFilter(opts.NatsAccountCollection,
			"organization = {:org} && active = true", map[string]any{"org": orgID})
		if err != nil {
			log.Printf("❌ No active NATS account for organization %s; cannot provision leaf node: %v", orgID, err)
			return e.Next()
		}

		roleID, err := ensureLeafNodeRole(e.App, opts.NatsRoleCollection, orgID)
		if err != nil {
			log.Printf("❌ Failed to ensure leaf-node NATS role: %v", err)
			return e.Next()
		}

		natsCol, err := e.App.FindCollectionByNameOrId(opts.NatsUserCollection)
		if err != nil {
			log.Printf("❌ Failed to find NATS user collection: %v", err)
			return e.Next()
		}

		pw, err := randomSecret(32)
		if err != nil {
			log.Printf("❌ Failed to generate credential: %v", err)
			return e.Next()
		}

		natsUser := core.NewRecord(natsCol)
		natsUser.Set("nats_username", "leaf-"+code)
		// email is the only globally-unique constraint on nats_users; derive it
		// from the leaf node id so it never collides across orgs/codes.
		natsUser.Set("email", fmt.Sprintf("leaf-%s@leaf.local", leaf.Id))
		natsUser.Set("emailVisibility", true)
		natsUser.SetPassword(pw)
		natsUser.Set("account_id", account.Id)
		natsUser.Set("role_id", roleID)
		natsUser.Set("organization", orgID)
		natsUser.Set("active", true)
		if err := e.App.Save(natsUser); err != nil {
			log.Printf("❌ Failed to create NATS user for leaf node '%s': %v", code, err)
			return e.Next()
		}

		leaf.Set("nats_user", natsUser.Id)
		if err := e.App.Save(leaf); err != nil {
			log.Printf("❌ Failed to link NATS user to leaf node '%s': %v", code, err)
			return e.Next()
		}

		log.Printf("✅ Provisioned NATS user 'leaf-%s' for leaf node '%s'", code, code)
		return e.Next()
	})
}

// ensureLeafNodeRole finds (or creates) the per-organization broad role used by
// leaf nodes, returning its id.
func ensureLeafNodeRole(app core.App, roleCollection, orgID string) (string, error) {
	existing, _ := app.FindFirstRecordByFilter(roleCollection,
		"organization = {:org} && name = {:name}",
		map[string]any{"org": orgID, "name": leafNodeRoleName})
	if existing != nil {
		return existing.Id, nil
	}

	col, err := app.FindCollectionByNameOrId(roleCollection)
	if err != nil {
		return "", err
	}

	role := core.NewRecord(col)
	role.Set("name", leafNodeRoleName)
	role.Set("description", "Edge leaf node: allow-all within its own NATS account (leaf remote + rule-router + leaf-sync).")
	role.Set("organization", orgID)
	role.Set("is_default", false)
	role.Set("max_subscriptions", -1)
	role.Set("max_data", -1)
	role.Set("max_payload", -1)
	role.Set("publish_permissions", []string{">"})
	role.Set("subscribe_permissions", []string{">"})
	role.Set("publish_deny_permissions", []string{})
	role.Set("subscribe_deny_permissions", []string{})
	if err := app.Save(role); err != nil {
		return "", err
	}
	return role.Id, nil
}

// randomSecret returns a hex-encoded cryptographically-random string of nBytes
// of entropy (used for the leaf node's NATS user password, which is never used
// for login — the user authenticates to NATS via its generated JWT/creds).
func randomSecret(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
