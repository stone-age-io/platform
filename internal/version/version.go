// Package version holds the build version shared by both binaries.
package version

import (
	"fmt"
	"runtime/debug"
	"strings"
)

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

// DependencyLines renders one indented "name version" line per module path, for
// use in a multi-line `--version` string. Names are the last path segment, and
// the version column is aligned.
//
// It exists because the module version is the only "stamp" a Go library can
// carry: there is no ldflags equivalent for a dependency, so the build info is
// the single source of truth for which pb-nats a given binary was compiled
// against. That is also the argument for tagging those repos -- an untagged
// module reads back as v0.0.0-20260822174619-50fca4306606, which names a commit
// nobody can map to a release, while a tagged one reads v0.1.0.
func DependencyLines(paths ...string) string {
	width := 0
	for _, p := range paths {
		if n := len(shortName(p)); n > width {
			width = n
		}
	}

	var b strings.Builder
	for _, p := range paths {
		fmt.Fprintf(&b, "  %-*s  %s\n", width, shortName(p), Dependency(p))
	}
	return b.String()
}

// shortName is the last segment of a module path: the name a human uses.
func shortName(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}
