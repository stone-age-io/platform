package hooks

import (
	"regexp"
	"testing"
)

// The same pattern schema.json puts on organizations.code. Slugify may return
// values this rejects, but never one that passes and is not the code meant,
// because a code is immutable once saved.
var pattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,30}$`)

func TestSlugify(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"lowercases", "Acme", "acme"},
		{"spaces become one hyphen", "Acme Industries", "acme-industries"},
		{"runs collapse", "Acme   --  Industries", "acme-industries"},
		{"punctuation is separator", "Acme, Inc.", "acme-inc"},
		{"leading junk is dropped", "  ...Acme", "acme"},
		{"trailing junk is dropped", "Acme, Inc.  ", "acme-inc"},
		{"digits survive", "Site 42 North", "site-42-north"},
		{"a leading digit is kept -- the pattern permits one", "3M", "3m"},
		{"non-ascii is a separator, not a transliteration", "Café Beta", "caf-beta"},
		{"the two reserved names derive to themselves", "System", "system"},
		{"operator too", "Operator", "operator"},

		// Deliberately invalid outputs. The hook refuses these rather than
		// inventing a prefix, because the guess would be permanent.
		{"nothing usable yields empty", "!!!", ""},
		{"a single letter is too short for the pattern", "X", "x"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Slugify(tc.in); got != tc.want {
				t.Errorf("Slugify(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// A derived code has to satisfy the field pattern, length ceiling included, or
// the save fails with a validation error instead of the hook's message.
func TestSlugifyTruncatesWithoutLeavingATrailingHyphen(t *testing.T) {
	// 40 characters, with the 31st and 32nd both non-alphanumeric, so a naive
	// truncation would end on a hyphen and fail the pattern.
	got := Slugify("abcdefghijklmnopqrstuvwxyzabcd  efghijkl")

	if len(got) > OrgCodeMaxLen {
		t.Fatalf("Slugify returned %d chars, want <= %d: %q", len(got), OrgCodeMaxLen, got)
	}
	if !pattern.MatchString(got) {
		t.Errorf("truncated slug %q does not match %s", got, pattern)
	}
}

// The two cases the hook is expected to reject. Pinned so a change to Slugify
// that repairs them is a deliberate decision. A leading digit ("3m") is valid.
func TestSlugifyDoesNotInventAValidCode(t *testing.T) {
	for _, in := range []string{"!!!", "X"} {
		if got := Slugify(in); pattern.MatchString(got) {
			t.Errorf("Slugify(%q) = %q, which passes the pattern — the hook can no longer refuse it", in, got)
		}
	}
}
