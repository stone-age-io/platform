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
// server sends unprompted on connect.
//
// WHY THIS AND NOT A CLIENT CONNECTION. Reachability and trust are two
// different failures with two different fixes, and a single authenticated
// connect conflates them: "connection refused" (nothing is listening, or the
// port is wrong) and "authorization violation" (the server is up but does not
// trust this platform's operator) both surface as a failed Connect. Reading the
// unauthenticated greeting answers the first question on its own, and
// CheckCreds below answers the second — so a report can say which one is
// actually broken.
//
// It also costs nothing: every NATS server writes INFO before any client
// authentication happens, so this needs no credential and creates no session.
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
	// A TLS-only listener, or anything that is not a NATS server, will not send
	// this. Say so plainly rather than reporting a JSON parse error.
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
// This is the check that catches the platform's nastiest silent failure: the
// operator JWT baked into the running nats-server's config no longer matches
// the operator in this database (someone regenerated one, or `nats export` was
// run against a different install, or an old nats.conf outlived a redeploy).
// Every account claim the Control Plane publishes then fails, no organization's
// NATS account ever reaches the server, and nothing in the console looks wrong
// — new devices simply cannot connect, days later, for no stated reason.
//
// It takes the creds as a string rather than a path because the credential
// lives in a database column, not on disk. nkeys parses the same decorated
// format either way.
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
		// One attempt, then give up. This is a diagnostic: a client that
		// retries in the background would report success for a server it only
		// reached on the third try, and would keep a connection alive after the
		// check returned.
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
//
// Both halves failing get one message, because they have one cause and one fix:
// the creds_file column does not hold a well-formed credential and must be
// re-minted. Splitting them would also mislead — nkeys treats any undecorated
// text as a raw JWT, so arbitrary garbage reliably passes the JWT parse and
// fails at the seed, and an error saying "no usable seed" would send the reader
// looking for a truncated key rather than a wrong column.
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
	// url.Parse needs a scheme to find the host; a bare "localhost:4222" is a
	// legal thing to have in a config file, so tolerate it.
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
