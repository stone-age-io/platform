package hooks

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"ganigeorgiev.com/pocketbase/dbx"
	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/apis"
	"ganigeorgiev.com/pocketbase/pocketbase/core"
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
// mint its NATS identity and Nebula host in ONE transaction, so a failure leaves
// no orphaned identity holding a signed credential or an allocated overlay IP.
//
// The atomicity is real: PocketBase defers *AfterCreateSuccess hooks to
// transaction completion (core/db.go: txInfo.OnComplete), and pb-nats mints and
// publishes there, so a rollback means pb-nats never signed or published.
//
// It is a route because it needs TWO authority levels for one operation: a
// member may create inventory but not attach an identity to it (things.createRule
// freezes nats_user/nebula_host for members), which no create rule can express.
//
// SECURITY: this route writes with app.Save(), which bypasses every API rule, so
// each check is restated below. The organization comes from the caller's own
// record, never the body, and a linked identity must belong to that organization.
func RegisterThingRoutes(app *pocketbase.PocketBase, opts ThingRoutesOptions) {
	// Only an owner or admin ever knows a Thing's password, the same line
	// things.manageRule draws. A member who knew it could sign in as the Thing
	// after an admin attached an identity, and read its creds_file or host key.
	// The route below withholds the generated one; this covers the record API,
	// whose create admits members and must carry a password, so no rule can
	// refuse the field. Replacing it here makes PocketBase skip its
	// passwordConfirm check (forms/record_upsert.go, syncPasswordFields).
	app.OnRecordCreateRequest(opts.ThingCollection).BindFunc(func(e *core.RecordRequestEvent) error {
		if e.HasSuperuserAuth() {
			return e.Next()
		}
		if _, _, err := requireMembership(e.RequestEvent, opts.MembershipCollection, rolesOrgManager); err != nil {
			pw, err := randomSecret(16)
			if err != nil {
				return e.InternalServerError("failed to generate credential", err)
			}
			e.Record.SetPassword(pw)
		}
		return e.Next()
	})

	// The other half: a Thing's password is never reset by email. PocketBase
	// serves request-password-reset to anyone because things keep password auth
	// for the agent, and a member chooses a Thing's email on create, so a member
	// could reset the password to their own mailbox and sign in as the Thing.
	app.OnRecordRequestPasswordResetRequest(opts.ThingCollection).BindFunc(func(e *core.RecordRequestPasswordResetRequestEvent) error {
		return e.ForbiddenError("Things do not use password reset. An owner or admin sets a Thing's password.", nil)
	})

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

			// An absent mode is "none"; an unrecognised one is rejected, not
			// treated as none.
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

			// The identity half is owner/admin only: things.createRule's boundary,
			// restated because app.Save() below skips that rule.
			wantsIdentity := natsMode != modeNone || nebulaMode != modeNone
			if wantsIdentity && role != "owner" && role != "admin" {
				return re.ForbiddenError("attaching a NATS or Nebula identity requires owner or admin", nil)
			}

			// A blank code is generated here, not left to the create hook in
			// codes.go: the Thing's synthetic email and its NATS username are
			// both built from the code before anything is saved.
			if body.Code == "" {
				prefix, err := typePrefix(re.App, thingTypesCollection, body.Type, orgID)
				if err != nil {
					return re.BadRequestError(err.Error(), nil)
				}
				body.Code, err = NewUniqueCode(re.App, orgID, prefix)
				if err != nil {
					return re.InternalServerError("failed to generate a code", err)
				}
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
				// things.authRule is `active = true` and a bool column has no
				// schema default, so an unset flag cannot authenticate.
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

			// txApp.Save() skips the activity request hooks, so the create is
			// recorded here, after the transaction, so the feed never describes a
			// Thing that did not land.
			RecordActivity(re.App, re, orgID, ActionProvisioned, "thing",
				created.Id, created.GetString("name"))

			resp := map[string]any{
				"id":    created.Id,
				"code":  created.GetString("code"),
				"email": created.GetString("email"),
			}
			// The password is returned exactly once (PocketBase stores only its
			// hash), and to owner/admin only, for the reason on the create hook
			// above. A member's Thing gets its password from an owner or admin.
			if role == "owner" || role == "admin" {
				resp["password"] = thingPassword
			}
			return re.JSON(200, resp)
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

// resolveInventoryRole returns the caller's active organization and role,
// refusing anyone without an inventory-writing role. The organization comes from
// the authenticated record, never the request.
func resolveInventoryRole(re *core.RequestEvent, opts ThingRoutesOptions) (string, string, error) {
	return requireMembership(re, opts.MembershipCollection, rolesInventory)
}

// assertThingCodeFree rejects a duplicate code up front with a readable message
// instead of a driver constraint error. Case is ignored, matching the unique
// index (ADR 0003).
func assertThingCodeFree(re *core.RequestEvent, opts ThingRoutesOptions, orgID, code string) error {
	existing, err := findFirst(re.App,
		opts.ThingCollection,
		"organization = {:org} && code:lower = {:code}",
		dbx.Params{"org": orgID, "code": strings.ToLower(code)},
	)
	if err != nil {
		return re.InternalServerError("failed to check the thing code", err)
	}
	if existing != nil {
		return re.BadRequestError(fmt.Sprintf("a thing with code %q already exists in this organization", code), nil)
	}
	return nil
}

// orgSlugFor returns the organization's code, the domain half of the synthetic
// emails this route mints for a Thing and its NATS / Nebula identities. It must
// be the stored code, never a slug of the name (ADR 0002 in platform-docs): a
// name is mutable and two names can slug alike, while organizations.code is
// globally unique and frozen.
func orgSlugFor(re *core.RequestEvent, opts ThingRoutesOptions, orgID string) (string, error) {
	org, err := re.App.FindRecordById(opts.OrgCollection, orgID)
	if err != nil {
		return "", re.NotFoundError("active organization not found", err)
	}
	code := org.GetString("code")
	if code == "" {
		// Unreachable through the record API (the create hook and the backfill
		// assign codes). Refusing beats minting "DOOR-1@.thing.local".
		return "", re.BadRequestError(
			"the active organization has no code; set one before creating Things", nil)
	}
	return code, nil
}

// resolveNatsUser returns the nats_users id to link, minting one under "auto".
// Under "link" the target must live in the caller's organization; without that
// check this route is a cross-tenant credential-theft path, since a Thing can
// read the credential of its own linked identity.
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

	// Scoped to the account, not global: a NATS user name only has to be unique
	// within its account (the JWT identity is the public key), and a global
	// check would let one tenant's names block another's. There is no unique
	// index on the column, so this check is the whole policy.
	existing, err := findFirst(txApp,
		opts.NatsUserCollection,
		"account_id = {:acct} && nats_username = {:u}",
		dbx.Params{"acct": account.Id, "u": body.Code},
	)
	if err != nil {
		return "", re.InternalServerError("failed to check the NATS username", err)
	}
	if existing != nil {
		return "", re.BadRequestError(fmt.Sprintf("NATS username %q is already used in this organization", body.Code), nil)
	}

	// The synthetic email has a global unique index. The domain half cannot
	// collide, but the local half can if assertThingCodeFree was bypassed, so
	// this says what happened instead of a bare "failed to create" from Save().
	email := fmt.Sprintf("%s@%s.nats.local", body.Code, orgSlug)
	clash, err := findFirst(txApp,
		opts.NatsUserCollection,
		"email = {:e}",
		dbx.Params{"e": email},
	)
	if err != nil {
		return "", re.InternalServerError("failed to check the NATS identity email", err)
	}
	if clash != nil {
		return "", re.BadRequestError(fmt.Sprintf(
			"identifier %q is already in use — a Thing with code %q already has a NATS identity in this organization",
			email, body.Code), nil)
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
	u.Set("email", email)
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

// resolveNebulaHost mirrors resolveNatsUser for the Nebula side. The overlay IP
// is caller-supplied; this route does not allocate one.
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

	existing, err := findFirst(txApp,
		opts.NebulaHostCollection,
		"network_id = {:n} && (hostname = {:h} || overlay_ip = {:ip})",
		dbx.Params{"n": body.Nebula.NetworkID, "h": body.Code, "ip": body.Nebula.OverlayIP},
	)
	if err != nil {
		return "", re.InternalServerError("failed to check the Nebula hostname and overlay IP", err)
	}
	if existing != nil {
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

// randomSecret returns nBytes of crypto/rand entropy, hex-encoded. Used for
// PocketBase passwords nobody types; a NATS identity authenticates to the bus
// with its signed JWT, not with this.
func randomSecret(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
