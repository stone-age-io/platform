package health

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
)

// ServerInfo is the subset of a NATS server's INFO greeting worth reporting.
type ServerInfo struct {
	ServerName string `json:"server_name"`
	Version    string `json:"version"`
	JetStream  bool   `json:"jetstream"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
}

// DialInfo opens a TCP connection to a NATS server and reads the INFO line the
// server sends unprompted on connect, before any authentication.
//
// Not a client connection: an authenticated connect fails the same way whether
// nothing is listening or the server distrusts the operator. This answers
// reachability alone, and CheckCreds answers trust, so a report can say which
// is broken.
func DialInfo(ctx context.Context, serverURL string, timeout time.Duration) (*ServerInfo, error) {
	addr, err := hostPort(serverURL)
	if err != nil {
		return nil, err
	}

	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetReadDeadline(dl)
	} else {
		_ = conn.SetReadDeadline(time.Now().Add(timeout))
	}

	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("connected to %s but read no INFO greeting: %w", addr, err)
	}
	// A TLS-only listener or a non-NATS service sends no INFO. Say so rather
	// than report a JSON parse error.
	if !strings.HasPrefix(line, "INFO ") {
		return nil, fmt.Errorf("%s answered but did not send a NATS INFO greeting (is it a NATS server, or TLS-only?)", addr)
	}

	var info ServerInfo
	if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "INFO ")), &info); err != nil {
		return nil, fmt.Errorf("could not parse INFO from %s: %w", addr, err)
	}
	return &info, nil
}

// CheckCreds connects to serverURL with the contents of a .creds file and
// reports whether the server accepted the credential.
//
// It catches a nats-server whose configured operator no longer matches this
// database's: every account claim then fails to publish while the console
// looks fine. Creds are a string because they live in a database column.
func CheckCreds(serverURL, creds string, timeout time.Duration) error {
	jwt, kp, err := parseCreds(creds)
	if err != nil {
		return err
	}
	defer kp.Wipe()

	nc, err := nats.Connect(serverURL,
		nats.UserJWT(
			func() (string, error) { return jwt, nil },
			func(nonce []byte) ([]byte, error) { return kp.Sign(nonce) },
		),
		nats.Name("stone-age-readiness"),
		nats.Timeout(timeout),
		// One attempt: a diagnostic must not retry or outlive the check.
		nats.MaxReconnects(0),
		nats.NoReconnect(),
	)
	if err != nil {
		return err
	}
	nc.Close()
	return nil
}

// parseCreds pulls the JWT and the signing key out of a decorated .creds file.
// Both failures get one message: the fix is the same (re-mint), and nkeys
// accepts any undecorated text as a JWT, so garbage fails at the seed and a
// seed-specific error would mislead.
func parseCreds(creds string) (string, nkeys.KeyPair, error) {
	const problem = "not a valid .creds file"

	jwt, jwtErr := nkeys.ParseDecoratedJWT([]byte(creds))
	kp, kpErr := nkeys.ParseDecoratedNKey([]byte(creds))

	switch {
	case jwtErr != nil:
		return "", nil, fmt.Errorf("%s: %w", problem, jwtErr)
	case kpErr != nil:
		return "", nil, fmt.Errorf("%s: %w", problem, kpErr)
	case strings.TrimSpace(jwt) == "":
		return "", nil, fmt.Errorf("%s: it contains no JWT", problem)
	}
	return jwt, kp, nil
}

// hostPort turns a nats:// URL into a dialable host:port, defaulting the port
// to 4222 the way every NATS client does.
func hostPort(rawURL string) (string, error) {
	// url.Parse needs a scheme to find the host; tolerate "localhost:4222".
	if !strings.Contains(rawURL, "://") {
		rawURL = "nats://" + rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid NATS URL %q: %w", rawURL, err)
	}
	if u.Host == "" {
		return "", fmt.Errorf("invalid NATS URL %q: no host", rawURL)
	}
	if u.Port() == "" {
		return net.JoinHostPort(u.Hostname(), "4222"), nil
	}
	return u.Host, nil
}
