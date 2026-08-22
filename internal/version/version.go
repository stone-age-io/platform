// Package version holds the build version shared by both binaries.
package version

import "runtime/debug"

// Version is stamped at build time via
//
//	-ldflags "-X platform/internal/version.Version=$(git describe --tags --always --dirty)"
//
// and defaults to "dev" for a plain `go build` or `go run`.
//
// One variable for both binaries, deliberately: `stone-age` and `leaf-sync` are
// built from the same tree and released from the same tag, so two stamps would
// only mean two chances to forget one -- and a `leaf-sync` reporting "dev" in its
// heartbeat while the server reports v0.3.1 is a support call about nothing.
//
// It surfaces in `--version` on both commands and in every leaf-sync heartbeat.
var Version = "dev"

// Dependency returns the module version of the given import path as recorded in
// the build info, or "unknown" if this binary was not built as a module.
//
// It exists because PocketBase's own pocketbase.Version is only stamped in
// official PocketBase releases -- in a build that imports it as a library it
// reads "(untracked)", so `--version` was reporting nothing useful about the
// half of the stack that decides authorization semantics. The API rules in
// schema.json are the entire enforcement layer, and how they are evaluated is
// PocketBase's business, so which PocketBase this binary carries is the second
// fact any bug report needs.
func Dependency(path string) string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, dep := range info.Deps {
		if dep.Path == path {
			return dep.Version
		}
	}
	return "unknown"
}
