package hooks

import (
	"regexp"
	"strings"
	"testing"
)

// generatedShape is what GenerateCode must always produce: an optional prefix
// of one to four capitals, then two three-symbol chunks from the 30-symbol
// alphabet. Written out here rather than built from codeSymbols, so a change to
// the alphabet has to change this test on purpose.
var generatedShape = regexp.MustCompile(`^([A-Z]{1,4}-)?[3-9A-HJ-NP-Y]{3}-[3-9A-HJ-NP-Y]{3}$`)

// The codePattern every inventory code has to satisfy (migrations.CodePattern,
// restated to keep this test inside the hooks package). A generated code the
// field rejects would fail its own save.
var inventoryCodePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,62}$`)

func TestAlphabetDropsTheLookAlikes(t *testing.T) {
	if len(codeLetters) != 23 || len(codeDigits) != 7 {
		t.Fatalf("alphabet is %d letters + %d digits, want 23 + 7", len(codeLetters), len(codeDigits))
	}
	for _, c := range "0O1I2Z" {
		if strings.ContainsRune(codeSymbols, c) {
			t.Errorf("alphabet contains %q, which reads as its look-alike", c)
		}
	}
}

// Enough draws that a broken chunk rule shows up. About half of all raw draws
// fail it, so a regression that skipped the check would fail within a handful.
func TestGenerateCodeShape(t *testing.T) {
	for _, prefix := range []string{"", "CA", "BLDG"} {
		for range 2000 {
			code, err := GenerateCode(prefix)
			if err != nil {
				t.Fatal(err)
			}
			if !generatedShape.MatchString(code) {
				t.Fatalf("GenerateCode(%q) = %q, not PFX-XXX-XXX", prefix, code)
			}
			if !inventoryCodePattern.MatchString(code) {
				t.Fatalf("GenerateCode(%q) = %q, which the code field would refuse", prefix, code)
			}
			if prefix != "" && !strings.HasPrefix(code, prefix+"-") {
				t.Fatalf("GenerateCode(%q) = %q, prefix missing", prefix, code)
			}

			chunks := strings.Split(strings.TrimPrefix(code, prefix+"-"), "-")
			if prefix == "" {
				chunks = strings.Split(code, "-")
			}
			for _, chunk := range chunks {
				if !strings.ContainsAny(chunk, codeLetters) || !strings.ContainsAny(chunk, codeDigits) {
					t.Fatalf("chunk %q of %q lacks a letter or a digit", chunk, code)
				}
			}
		}
	}
}

// Random means neighbours differ. Two thousand draws colliding would mean the
// source is not random at all, which is the failure that turns a mistyped code
// back into somebody else's record.
func TestGenerateCodeIsNotRepetitive(t *testing.T) {
	seen := map[string]bool{}
	for range 2000 {
		code, err := GenerateCode("")
		if err != nil {
			t.Fatal(err)
		}
		if seen[code] {
			t.Fatalf("GenerateCode repeated %q within 2000 draws", code)
		}
		seen[code] = true
	}
}

func TestTypePrefixPattern(t *testing.T) {
	for _, ok := range []string{"C", "CA", "CAM", "BLDG"} {
		if !TypePrefixPattern.MatchString(ok) {
			t.Errorf("prefix %q should be accepted", ok)
		}
	}
	// Lowercase, digits, a separator, too long, empty. A digit is the one worth
	// pinning: it would blur where the prefix ends and the random part begins.
	for _, bad := range []string{"ca", "C4", "CA-", "BLDGS", ""} {
		if TypePrefixPattern.MatchString(bad) {
			t.Errorf("prefix %q should be refused", bad)
		}
	}
}
