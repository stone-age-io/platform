package hooks

import (
	"time"

	"ganigeorgiev.com/pocketbase/pocketbase/core"

	"platform/internal/health"
	"platform/internal/metrics"
)

// Exposed for hooks_test, which must be an external test package because
// internal/testutil imports platform/hooks. Test-only, so nothing that ships is
// widened.

type CertSummary = certSummary

const (
	CertKindCA       = certKindCA
	CertKindHost     = certKindHost
	CertExpiryWindow = certExpiryWindow
	CAExpiryWindow   = caExpiryWindow
	CertCheckName    = "nebula_cert_expiry"
)

func NebulaCertSummaries(app core.App, opts ObservabilityOptions, now time.Time) ([]CertSummary, error) {
	return nebulaCertSummaries(app, opts, now)
}

func RegisterPlatformChecks(app core.App, reg *health.Registry, opts ObservabilityOptions) {
	registerPlatformChecks(app, reg, opts)
}

func RegisterPlatformMetrics(app core.App, set *metrics.Set, opts ObservabilityOptions) {
	registerPlatformMetrics(app, set, opts)
}

// The shared org-context guard and its role sets, so org_context_test.go can
// test the refusals for routes not yet written, which the authz script cannot.
var (
	RolesOrgManager = rolesOrgManager
	RolesInventory  = rolesInventory
)

func RequireMembership(re *core.RequestEvent, membershipCollection string, roles []string) (string, string, error) {
	return requireMembership(re, membershipCollection, roles)
}

// Inventory events. The sweep and the platform-events identity run from
// OnServe, which no test reaches.
var (
	InventoryFields     = inventoryFields
	InventorySubject    = inventorySubject
	InventoryImportName = inventoryImportName
)

func InventoryExportName(orgID string) string { return inventoryExportName(orgID) }

func SweepInventoryExports(app core.App, opts InventoryEventsOptions) {
	sweepInventoryExports(app, opts)
}

func EnsurePlatformEventsUser(app core.App, opts InventoryEventsOptions) (string, error) {
	return ensurePlatformEventsUser(app, opts)
}
