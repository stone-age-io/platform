package hooks

import (
	"net/url"
	"strings"
	"testing"
)

// The invitation link is the product of this feature; its shape is checked here
// as a string.
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

// The token makes an invitation unguessable: pin that it is URL-safe and not a
// constant.
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

		// The invites schema caps `token` at 64 characters; 32 bytes encode to 43.
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
