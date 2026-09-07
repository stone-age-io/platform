package demoseed

// export_test.go exposes the fixture tables to the EXTERNAL test package
// (demoseed_test) without widening the package's real API. The file is compiled
// only under `go test`, so nothing here reaches a build of the binary.
//
// The alternative was moving the fixture-driven tests into an in-package test
// file, but the assertions they need -- requirePermission, composeSubject,
// subjectMatches -- live in seed_test.go and are shared with tests that check
// role permissions directly, so splitting them would mean duplicating the NATS
// subject matcher. One alias file is cheaper than two copies of that.

type (
	RoleFixture      = roleFixture
	OperationFixture = operationFixture
	ThingTypeFixture = thingTypeFixture
)

// The fixture tables, read-only from the tests' point of view.
var (
	ThingTypeFixtures = thingTypes
	OperationFixtures = operations
	RoleTemplates     = roleTemplates
)
