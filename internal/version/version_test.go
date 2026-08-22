package version

import (
	"strings"
	"testing"
)

// A module that is not a dependency reads back as "unknown", which is what
// makes this assertion deterministic: the point under test is the formatting,
// not the lookup.
func TestDependencyLinesAlignsTheVersionColumn(t *testing.T) {
	got := DependencyLines(
		"example.com/x/short",
		"example.com/x/a-much-longer-name",
	)

	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want one line per path, got %d:\n%s", len(lines), got)
	}

	// Every line indents by two and the versions start at the same column, so a
	// multi-line --version reads as a table rather than ragged text.
	first := strings.Index(lines[0], "unknown")
	for i, line := range lines {
		if !strings.HasPrefix(line, "  ") {
			t.Errorf("line %d is not indented: %q", i, line)
		}
		if col := strings.Index(line, "unknown"); col != first {
			t.Errorf("line %d starts its version at column %d, want %d: %q", i, col, first, line)
		}
	}

	if !strings.Contains(lines[0], "short") || strings.Contains(lines[0], "example.com") {
		t.Errorf("want the last path segment as the label, got %q", lines[0])
	}
}
