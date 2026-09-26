package hooks

import (
	"testing"
	"time"
)

// The readiness warning once said "expiring within 30 days" for a CA counted
// against its 90-day window, so an operator reading "30" could believe there was
// more slack than the check had actually measured -- or less. Each kind has to
// name its own window.
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
