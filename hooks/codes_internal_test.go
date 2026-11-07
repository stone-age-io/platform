package hooks

import (
	"regexp"
	"strings"
	"testing"
)

// generatedShape is what GenerateCode must always produce. Written out rather
// than built from codeSymbols, so an alphabet change must change this test too.
var generatedShape = regexp.MustCompile(`^([A-Z]{1,4}-)?[3-9A-HJ-NP-Y]{3}-[3-9A-HJ-NP-Y]{3}$`)

// migrations.CodePattern, restated to keep this test inside the hooks package.
// A generated code the field rejects would fail its own save.
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

// About half of all raw draws fail the chunk rule, so a regression that skipped
// it fails within a handful of these draws.
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

// A broken source (constant, or a short cycle) repeats hundreds of times. One
// repeat is not that failure: 2,000 honest draws from ~2.1e8 codes repeat once
// about 1% of the time, so failing on it is flaky. Three or more has odds of
// about 1 in 7 million.
func TestGenerateCodeIsNotRepetitive(t *testing.T) {
	seen := map[string]bool{}
	repeats := 0
	for range 2000 {
		code, err := GenerateCode("")
		if err != nil {
			t.Fatal(err)
		}
		if seen[code] {
			repeats++
		}
		seen[code] = true
	}
	if repeats > 2 {
		t.Fatalf("GenerateCode repeated %d codes within 2000 draws", repeats)
	}
}

func TestTypePrefixPattern(t *testing.T) {
	for _, ok := range []string{"C", "CA", "CAM", "BLDG"} {
		if !TypePrefixPattern.MatchString(ok) {
			t.Errorf("prefix %q should be accepted", ok)
		}
	}
	// A digit would blur where the prefix ends and the random part begins.
	for _, bad := range []string{"ca", "C4", "CA-", "BLDGS", ""} {
		if TypePrefixPattern.MatchString(bad) {
			t.Errorf("prefix %q should be refused", bad)
		}
	}
}
