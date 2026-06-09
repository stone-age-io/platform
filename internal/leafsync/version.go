package leafsync

// Version is the leaf-sync build version. It is stamped at build time via
//
//	-ldflags "-X platform/internal/leafsync.Version=$(git describe --tags --always --dirty)"
//
// and defaults to "dev" for plain `go build`/`go run`. It surfaces in the CLI
// `--version` output and in each heartbeat payload.
var Version = "dev"
