package hooks

import (
	"net/url"
	"strings"
	"testing"
)

// The invitation link is the entire product of this feature -- everything else
// in hooks/invites.go exists to put a working one in somebody's inbox -- and it
// is the one part no integration test can check, because nothing in the suite
// sends mail. So it is checked here, as a string.
func TestInviteLink(t *testing.T) {
	const token = "abc-123_XYZ"

	t.Run("carries the token and the address", func(t *testing.T) {
		raw := inviteLink("https://console.example.test", token, "invitee@example.test")

		u, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("not a URL: %v", err)
		}
		if u.Path != "/accept-invite" {
			t.Errorf("path = %q, want /accept-invite (the console route, ui/src/router/index.ts)", u.Path)
		}
		if got := u.Query().Get("token"); got != token {
			t.Errorf("token = %q, want %q", got, token)
		}
		// The absorbed template sent only the token, while AcceptInviteView reads
		// `email` to prefill the registration form an invitee without an account
		// has to fill in first -- so that field was always blank and every invitee
		// retyped an address the system already knew.
		if got := u.Query().Get("email"); got != "invitee@example.test" {
			t.Errorf("email = %q, want invitee@example.test -- AcceptInviteView prefills the register form from this", got)
		}
	})

	t.Run("no trailing slash doubled", func(t *testing.T) {
		raw := inviteLink("https://console.example.test/", token, "a@b.test")
		if strings.Contains(raw, "test//accept-invite") {
			t.Errorf("doubled slash in %q -- Meta.AppURL is operator-entered and often has a trailing slash", raw)
		}
	})

	t.Run("omits a blank address rather than sending email=", func(t *testing.T) {
		raw := inviteLink("https://console.example.test", token, "")
		if strings.Contains(raw, "email=") {
			t.Errorf("blank email should be omitted, got %q", raw)
		}
	})
}

// generateInviteToken is what makes an invitation unguessable, so the two
// properties worth pinning are that it is URL-safe and that it is not a constant.
func TestGenerateInviteToken(t *testing.T) {
	seen := make(map[string]bool, 64)

	for i := 0; i < 64; i++ {
		token, err := generateInviteToken()
		if err != nil {
			t.Fatalf("generateInviteToken: %v", err)
		}

		if seen[token] {
			t.Fatalf("duplicate token %q after %d draws", token, i)
		}
		seen[token] = true

		// RawURLEncoding, so no padding to survive a query string. The invites
		// schema caps `token` at 64 characters; 32 random bytes encode to 43.
		if strings.ContainsAny(token, "=+/") {
			t.Errorf("token %q is not URL-safe -- must be RawURLEncoding", token)
		}
		if len(token) != 43 {
			t.Errorf("token length = %d, want 43 (32 bytes, unpadded base64url)", len(token))
		}
		if url.QueryEscape(token) != token {
			t.Errorf("token %q changes under query escaping", token)
		}
	}
}
