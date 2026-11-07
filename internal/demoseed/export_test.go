package demoseed

// export_test.go exposes the fixture tables to the external test package
// (demoseed_test) without widening the real API; it compiles only under
// `go test`. The fixture tests stay external because they share the NATS
// subject matcher in seed_test.go.

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
