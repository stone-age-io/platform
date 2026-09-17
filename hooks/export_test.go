package hooks

import (
	"time"

	"github.com/pocketbase/pocketbase/core"

	"platform/internal/health"
	"platform/internal/metrics"
)

// Exposed for hooks_test, which must be an EXTERNAL test package: it needs
// internal/testutil for a real database, and testutil imports platform/hooks.
//
// Test-only by construction -- an _test.go file is not compiled into the
// binary -- so this widens nothing that ships. Same shape as
// internal/demoseed/export_test.go.

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

// The shared org-context guard and the two role sets every route gates on.
// Exposed so org_context_test.go can exercise the refusals directly: the authz
// script covers them over HTTP, but only for the routes that exist today, and
// the point of this helper is the route someone writes next.
var (
	RolesOrgManager = rolesOrgManager
	RolesInventory  = rolesInventory
)

func RequireMembership(re *core.RequestEvent, membershipCollection string, roles []string) (string, string, error) {
	return requireMembership(re, membershipCollection, roles)
}
