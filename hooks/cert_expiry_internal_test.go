package hooks

import (
	"testing"
	"time"
)

// Each kind must name its own window: a CA counted against 90 days must not be
// reported as "within 30 days".
func TestExpiringPhraseNamesEachKindsOwnWindow(t *testing.T) {
	sums := []certSummary{
		{Kind: certKindCA, Expiring: 1, Window: caExpiryWindow},
		{Kind: certKindHost, Expiring: 2, Window: certExpiryWindow},
	}
	got := certExpiringPhrase(sums)
	want := "1 CA within 90 days, 2 host certificates within 30 days"
	if got != want {
		t.Fatalf("certExpiringPhrase = %q, want %q", got, want)
	}

	if got := certExpiringPhrase([]certSummary{{Kind: certKindHost, Window: time.Hour}}); got != "none" {
		t.Fatalf("nothing expiring should read %q, got %q", "none", got)
	}
}
