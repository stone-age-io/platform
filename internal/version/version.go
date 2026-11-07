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
// and defaults to "dev" for a plain `go build` or `go run`. It surfaces in
// `stone-age --version` and in the readiness report.
var Version = "dev"

// Dependency returns the module version of the given import path as recorded in
// the build info, or "unknown" if this binary was not built as a module.
//
// pocketbase.Version reads "(untracked)" when PocketBase is imported as a
// library, and PocketBase evaluates the API rules that are the whole
// enforcement layer, so a bug report needs its version from the build info.
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
// A dependency has no ldflags stamp, so the build info is the only record of
// which pb-nats a binary carries. Tag those repos: an untagged module reads
// back as a pseudo-version nobody can map to a release.
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
