package hooks

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// ThingRoutesOptions names the collections involved so a deployment that renamed
// them via config.yaml still works.
type ThingRoutesOptions struct {
	ThingCollection         string
	OrgCollection           string
	MembershipCollection    string
	NatsUserCollection      string
	NatsAccountCollection   string
	NatsRoleCollection      string
	NebulaHostCollection    string
	NebulaNetworkCollection string
}

// provisionMode mirrors the three choices the form offers per identity.
const (
	modeAuto = "auto"
	modeLink = "link"
	modeNone = "none"
)

type createThingRequest struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Code        string         `json:"code"`
	Type        string         `json:"type"`
	Location    string         `json:"location"`
	Metadata    map[string]any `json:"metadata"`

	Nats struct {
		Mode   string `json:"mode"`
		UserID string `json:"user_id"` // link
		RoleID string `json:"role_id"` // auto
	} `json:"nats"`

	Nebula struct {
		Mode      string `json:"mode"`
		HostID    string `json:"host_id"`    // link
		NetworkID string `json:"network_id"` // auto
		OverlayIP string `json:"overlay_ip"` // auto
	} `json:"nebula"`
}

// RegisterThingRoutes adds POST /api/org/things: create a Thing and, optionally,
// mint its NATS identity and Nebula host in ONE transaction.
//
// This replaces three unguarded client calls. The console used to create the
// nats_users record, then the nebula_hosts record, then the Thing — three
// independent requests with no rollback, so a failure on the third left an
// orphaned NATS identity holding a signed credential and a Nebula host holding an
// allocated overlay IP, with nothing referencing either. The browser also had to
// know the whole provisioning recipe (which account, which default role, what
// email shape), and it got one part wrong: it never set `active`, so every Thing
// created through the console landed with active = false and could not
// authenticate at all once things.authRule became `active = true`.
//
// Atomicity is real, not decorative. PocketBase defers the *AfterCreateSuccess
// hooks to transaction completion (core/db.go: txInfo.OnComplete), and pb-nats
// mints the user JWT and publishes to NATS on OnRecordAfterCreateSuccess. So a
// rollback here means pb-nats never signed anything and never published — the
// failure mode is "nothing happened", not "NATS knows about a user PocketBase
// forgot".
//
// The route exists for a second reason the rules cannot cover: it needs TWO
// authority levels for one operation. A member may create inventory but not
// attach an identity to it (things.createRule freezes nats_user/nebula_host for
// the member branch), and there is no way to express "you may call this endpoint,
// but the identity half of it is admin-only" in a create rule. Here it is a role
// check per section.
//
// SECURITY: this route writes with app.Save(), which bypasses API rules entirely.
// Every check the rules would have applied is therefore restated below —
// organization comes from the caller's own record and never from the body, and a
// linked identity is verified to belong to that same organization. Do not assume
// a rule is still protecting anything on this path.
func RegisterThingRoutes(app *pocketbase.PocketBase, opts ThingRoutesOptions) {
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.Router.POST("/api/org/things", func(re *core.RequestEvent) error {
			var body createThingRequest
			if err := re.BindBody(&body); err != nil {
				return re.BadRequestError("invalid request body", err)
			}

			body.Name = strings.TrimSpace(body.Name)
			body.Code = strings.TrimSpace(body.Code)
			if body.Name == "" {
				return re.BadRequestError("name is required", nil)
			}
			if body.Code == "" {
				return re.BadRequestError("code is required", nil)
			}

			// Modes default to "none" so an absent block provisions nothing. A
			// mode we don't recognise is rejected rather than treated as none —
			// the same reason the account-key route rejects unknown actions.
			natsMode := defaultMode(body.Nats.Mode)
			nebulaMode := defaultMode(body.Nebula.Mode)
			if natsMode == "" {
				return re.BadRequestError("nats.mode must be auto, link, or none", nil)
			}
			if nebulaMode == "" {
				return re.BadRequestError("nebula.mode must be auto, link, or none", nil)
			}

			orgID, role, err := resolveInventoryRole(re, opts)
			if err != nil {
				return err
			}

			// The identity half is owner/admin only. This is the same boundary
			// things.createRule draws by freezing nats_user/nebula_host for
			// members, restated because app.Save() below skips that rule.
			wantsIdentity := natsMode != modeNone || nebulaMode != modeNone
			if wantsIdentity && role != "owner" && role != "admin" {
				return re.ForbiddenError("attaching a NATS or Nebula identity requires owner or admin", nil)
			}

			if err := assertThingCodeFree(re, opts, orgID, body.Code); err != nil {
				return err
			}

			orgSlug, err := orgSlugFor(re, opts, orgID)
			if err != nil {
				return err
			}

			thingPassword, err := randomSecret(16)
			if err != nil {
				return re.InternalServerError("failed to generate credential", err)
			}

			var created *core.Record

			txErr := re.App.RunInTransaction(func(txApp core.App) error {
				natsUserID, err := resolveNatsUser(re, txApp, opts, natsMode, body, orgID, orgSlug)
				if err != nil {
					return err
				}

				nebulaHostID, err := resolveNebulaHost(re, txApp, opts, nebulaMode, body, orgID, orgSlug)
				if err != nil {
					return err
				}

				col, err := txApp.FindCollectionByNameOrId(opts.ThingCollection)
				if err != nil {
					return re.InternalServerError("thing collection not found", err)
				}

				thing := core.NewRecord(col)
				thing.Set("name", body.Name)
				thing.Set("description", body.Description)
				thing.Set("code", body.Code)
				thing.Set("email", fmt.Sprintf("%s@%s.thing.local", body.Code, orgSlug))
				thing.Set("emailVisibility", true)
				thing.SetPassword(thingPassword)
				thing.Set("organization", orgID)
				// The client used to omit this, which left the Thing unable to
				// authenticate: things.authRule is `active = true` and a bool
				// column has no schema default, so an unset flag reads false.
				thing.Set("active", true)
				if body.Type != "" {
					thing.Set("type", body.Type)
				}
				if body.Location != "" {
					thing.Set("location", body.Location)
				}
				if body.Metadata != nil {
					thing.Set("metadata", body.Metadata)
				}
				if natsUserID != "" {
					thing.Set("nats_user", natsUserID)
				}
				if nebulaHostID != "" {
					thing.Set("nebula_host", nebulaHostID)
				}

				if err := txApp.Save(thing); err != nil {
					return re.BadRequestError("failed to create thing", err)
				}
				created = thing
				return nil
			})
			if txErr != nil {
				return txErr
			}

			// The password is returned exactly once — PocketBase stores only its
			// hash, so this response is the only chance to record it.
			return re.JSON(200, map[string]any{
				"id":       created.Id,
				"code":     created.GetString("code"),
				"email":    created.GetString("email"),
				"password": thingPassword,
			})
		}).Bind(apis.RequireAuth("users"))

		return se.Next()
	})
}

// defaultMode normalises an absent mode to "none" and returns "" for anything
// unrecognised, so the caller can reject it explicitly.
func defaultMode(m string) string {
	switch m {
	case "":
		return modeNone
	case modeAuto, modeLink, modeNone:
		return m
	default:
		return ""
	}
}

// resolveInventoryRole returns the caller's active organization and their role in
// it, refusing anyone without an inventory-writing role. The organization comes
// from the authenticated record, never the request, so a caller cannot create a
// Thing in someone else's tenant.
func resolveInventoryRole(re *core.RequestEvent, opts ThingRoutesOptions) (string, string, error) {
	if re.Auth == nil || re.Auth.Collection().Name != "users" {
		return "", "", re.UnauthorizedError("user authentication required", nil)
	}

	orgID := re.Auth.GetString("current_organization")
	if orgID == "" {
		return "", "", re.BadRequestError("no active organization selected", nil)
	}

	membership, err := re.App.FindFirstRecordByFilter(
		opts.MembershipCollection,
		"user = {:user} && organization = {:org} && (role = 'owner' || role = 'admin' || role = 'member')",
		dbx.Params{"user": re.Auth.Id, "org": orgID},
	)
	if err != nil || membership == nil {
		// Same shape as an API-rule rejection: don't distinguish "not a member"
		// from "insufficient role".
		return "", "", re.ForbiddenError("an inventory role in the active organization is required", nil)
	}

	return orgID, membership.GetString("role"), nil
}

// assertThingCodeFree rejects a duplicate code up front so the caller gets a
// readable message instead of a constraint violation surfaced from the driver.
func assertThingCodeFree(re *core.RequestEvent, opts ThingRoutesOptions, orgID, code string) error {
	existing, _ := re.App.FindFirstRecordByFilter(
		opts.ThingCollection,
		"organization = {:org} && code = {:code}",
		dbx.Params{"org": orgID, "code": code},
	)
	if existing != nil {
		return re.BadRequestError(fmt.Sprintf("a thing with code %q already exists in this organization", code), nil)
	}
	return nil
}

var slugNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// orgSlugFor derives the email-domain slug from the organization name, matching
// what the console used to build client-side so existing records keep their shape.
func orgSlugFor(re *core.RequestEvent, opts ThingRoutesOptions, orgID string) (string, error) {
	org, err := re.App.FindRecordById(opts.OrgCollection, orgID)
	if err != nil {
		return "", re.NotFoundError("active organization not found", err)
	}
	slug := slugNonAlnum.ReplaceAllString(strings.ToLower(org.GetString("name")), "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "org"
	}
	return slug, nil
}

// resolveNatsUser returns the nats_users id to link, minting one under "auto".
// Under "link" the target is verified to live in the caller's organization —
// without that check this route would be a cross-tenant credential-theft path,
// since a Thing can read the credential of its own linked identity.
func resolveNatsUser(
	re *core.RequestEvent, txApp core.App, opts ThingRoutesOptions,
	mode string, body createThingRequest, orgID, orgSlug string,
) (string, error) {
	switch mode {
	case modeNone:
		return "", nil

	case modeLink:
		if body.Nats.UserID == "" {
			return "", re.BadRequestError("nats.user_id is required when nats.mode is link", nil)
		}
		rec, err := txApp.FindRecordById(opts.NatsUserCollection, body.Nats.UserID)
		if err != nil || rec.GetString("organization") != orgID {
			return "", re.BadRequestError("nats.user_id is not a NATS identity in this organization", nil)
		}
		return rec.Id, nil
	}

	// auto
	account, err := txApp.FindFirstRecordByFilter(
		opts.NatsAccountCollection,
		"organization = {:org} && active = true",
		dbx.Params{"org": orgID},
	)
	if err != nil || account == nil {
		return "", re.BadRequestError("no active NATS account for this organization", nil)
	}

	roleID := body.Nats.RoleID
	if roleID == "" {
		def, err := txApp.FindFirstRecordByFilter(
			opts.NatsRoleCollection,
			"organization = {:org} && is_default = true",
			dbx.Params{"org": orgID},
		)
		if err != nil || def == nil {
			return "", re.BadRequestError("no default NATS role for this organization; pass nats.role_id", nil)
		}
		roleID = def.Id
	} else {
		role, err := txApp.FindRecordById(opts.NatsRoleCollection, roleID)
		if err != nil || role.GetString("organization") != orgID {
			return "", re.BadRequestError("nats.role_id is not a role in this organization", nil)
		}
	}

	if existing, _ := txApp.FindFirstRecordByFilter(
		opts.NatsUserCollection,
		"nats_username = {:u}",
		dbx.Params{"u": body.Code},
	); existing != nil {
		return "", re.BadRequestError(fmt.Sprintf("NATS username %q is already taken", body.Code), nil)
	}

	col, err := txApp.FindCollectionByNameOrId(opts.NatsUserCollection)
	if err != nil {
		return "", re.InternalServerError("nats user collection not found", err)
	}
	pw, err := randomSecret(32)
	if err != nil {
		return "", re.InternalServerError("failed to generate credential", err)
	}

	u := core.NewRecord(col)
	u.Set("nats_username", body.Code)
	u.Set("email", fmt.Sprintf("%s@%s.nats.local", body.Code, orgSlug))
	u.Set("emailVisibility", true)
	u.SetPassword(pw)
	u.Set("account_id", account.Id)
	u.Set("role_id", roleID)
	u.Set("organization", orgID)
	u.Set("active", true)
	if err := txApp.Save(u); err != nil {
		return "", re.BadRequestError("failed to create NATS identity", err)
	}
	return u.Id, nil
}

// resolveNebulaHost mirrors resolveNatsUser for the Nebula side. The overlay IP is
// operator-supplied, as it was in the form — this route does not allocate one.
func resolveNebulaHost(
	re *core.RequestEvent, txApp core.App, opts ThingRoutesOptions,
	mode string, body createThingRequest, orgID, orgSlug string,
) (string, error) {
	switch mode {
	case modeNone:
		return "", nil

	case modeLink:
		if body.Nebula.HostID == "" {
			return "", re.BadRequestError("nebula.host_id is required when nebula.mode is link", nil)
		}
		rec, err := txApp.FindRecordById(opts.NebulaHostCollection, body.Nebula.HostID)
		if err != nil || rec.GetString("organization") != orgID {
			return "", re.BadRequestError("nebula.host_id is not a Nebula host in this organization", nil)
		}
		return rec.Id, nil
	}

	// auto
	if body.Nebula.NetworkID == "" {
		return "", re.BadRequestError("nebula.network_id is required when nebula.mode is auto", nil)
	}
	if body.Nebula.OverlayIP == "" {
		return "", re.BadRequestError("nebula.overlay_ip is required when nebula.mode is auto", nil)
	}
	network, err := txApp.FindRecordById(opts.NebulaNetworkCollection, body.Nebula.NetworkID)
	if err != nil || network.GetString("organization") != orgID {
		return "", re.BadRequestError("nebula.network_id is not a network in this organization", nil)
	}

	if existing, _ := txApp.FindFirstRecordByFilter(
		opts.NebulaHostCollection,
		"network_id = {:n} && (hostname = {:h} || overlay_ip = {:ip})",
		dbx.Params{"n": body.Nebula.NetworkID, "h": body.Code, "ip": body.Nebula.OverlayIP},
	); existing != nil {
		return "", re.BadRequestError(
			fmt.Sprintf("hostname %q or overlay IP %q is already used on that network", body.Code, body.Nebula.OverlayIP), nil)
	}

	col, err := txApp.FindCollectionByNameOrId(opts.NebulaHostCollection)
	if err != nil {
		return "", re.InternalServerError("nebula host collection not found", err)
	}
	pw, err := randomSecret(32)
	if err != nil {
		return "", re.InternalServerError("failed to generate credential", err)
	}

	h := core.NewRecord(col)
	h.Set("hostname", body.Code)
	h.Set("email", fmt.Sprintf("%s@%s.nebula.local", body.Code, orgSlug))
	h.Set("emailVisibility", true)
	h.SetPassword(pw)
	h.Set("network_id", body.Nebula.NetworkID)
	h.Set("overlay_ip", body.Nebula.OverlayIP)
	h.Set("organization", orgID)
	h.Set("active", true)
	if err := txApp.Save(h); err != nil {
		return "", re.BadRequestError("failed to create Nebula host", err)
	}
	return h.Id, nil
}
