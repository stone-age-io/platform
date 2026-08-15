// Package natsd runs a NATS server inside the Control Plane process.
//
// The server is an ordinary nats-server. It reads the same nats.conf that
// `stone-age nats export` writes, and behaves exactly as it would if you ran
// `nats-server -c nats.conf` yourself. There is no embedded-only mode, no
// options derived in Go, and no second way for an account JWT to reach it: the
// NATS support library still publishes account claims over $SYS exactly as it
// does to an external server.
//
// That sameness is the point. It keeps one code path in the library, keeps the
// config a real file you can read and edit, and makes moving to an external
// server a config change rather than a migration.
//
// Deliberately not supported: clustering the Control Plane itself. Run one
// embedded server. If you need more than one node, add external ones with a
// cluster block, or turn the embedded server off entirely.
package natsd

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"sync"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
)

const (
	// readyTimeout is how long to wait for the server to accept connections
	// before giving up. Generous: JetStream recovers its store on startup, and
	// a large store on slow disk is not an error.
	readyTimeout = 30 * time.Second

	// defaultConfigDir is the output directory the docs use for `nats export`.
	defaultConfigDir = "./nats-config"

	// DefaultConfigFile is the config --nats loads unless --nats-config says
	// otherwise. It is what `nats export --output ./nats-config/` produces.
	DefaultConfigFile = defaultConfigDir + "/nats.conf"
)

// Server wraps an embedded nats-server.
type Server struct {
	ns *natsserver.Server
}

// Start loads confPath, starts a NATS server from it, and waits for it to
// accept connections.
//
// clientURL is the address this Control Plane is configured to dial
// (nats.server_url). It is not used to connect — it is checked against the port
// the server actually listens on, because the two disagreeing is the one
// misconfiguration that leaves a process talking to itself and failing.
//
// The returned error is intended to be fatal to startup. A Control Plane asked
// for an embedded server that cannot start should say so and stop, not run on
// in a state where every credential it issues points at nothing.
func Start(confPath, clientURL string) (*Server, error) {
	if _, err := os.Stat(confPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf(
				"NATS config not found at %s\n"+
					"       Generate it first:  ./stone-age nats export --output %s\n"+
					"       Or point at an existing config with --nats-config",
				confPath, defaultConfigDir)
		}
		return nil, fmt.Errorf("cannot read NATS config %s: %w", confPath, err)
	}

	opts, err := natsserver.ProcessConfigFile(confPath)
	if err != nil {
		return nil, fmt.Errorf("invalid NATS config %s: %w", confPath, err)
	}

	// PocketBase owns SIGINT/SIGTERM. Without this nats-server installs its own
	// handlers and the two fight over shutdown.
	opts.NoSigs = true

	ns, err := natsserver.NewServer(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create NATS server: %w", err)
	}

	// Route NATS output through the platform logger so there is one log stream.
	// Set before Start so nothing is written anywhere else first.
	//
	// A `log_file` in nats.conf is not honoured as a result. That is a known
	// trade for having one place to look when something goes wrong.
	logger := newLogger()
	ns.SetLogger(logger, opts.Debug, opts.Trace)

	go ns.Start()

	if err := waitReady(ns, logger); err != nil {
		ns.Shutdown()
		ns.WaitForShutdown()
		return nil, err
	}

	if err := checkPortsAgree(ns.ClientURL(), clientURL); err != nil {
		ns.Shutdown()
		ns.WaitForShutdown()
		return nil, err
	}

	log.Printf("✅ Embedded NATS server listening on %s (config: %s)", ns.ClientURL(), confPath)
	return &Server{ns: ns}, nil
}

// waitReady blocks until the server accepts connections, or fails.
//
// It polls rather than calling ReadyForConnections(readyTimeout) once, because
// a server can log a fatal and never become ready — a JetStream store it cannot
// open, a system account it will not accept — and waiting out the full timeout
// to then report a recorded fatal turns a one-second failure into a thirty
// second one. nats-server's Start() returns after a fatal instead of panicking,
// so the recorded message is the only diagnosis available.
func waitReady(ns *natsserver.Server, logger *logger) error {
	deadline := time.Now().Add(readyTimeout)
	for {
		if ns.ReadyForConnections(250 * time.Millisecond) {
			// A server can accept connections while a subsystem the config
			// asked for failed. Refuse it: JetStream silently missing is worse
			// than not starting, because the twins just never persist.
			if reason := logger.lastFatal(); reason != "" {
				return fmt.Errorf("embedded NATS server started but reported a fatal error: %s", reason)
			}
			return nil
		}
		if reason := logger.lastFatal(); reason != "" {
			return fmt.Errorf("embedded NATS server failed to start: %s", reason)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("embedded NATS server did not become ready within %s", readyTimeout)
		}
	}
}

// checkPortsAgree fails when the embedded server listens on a different port
// from the one this Control Plane dials.
//
// Only the port is compared. Hosts differ legitimately all the time — the
// server binds 0.0.0.0 while the configured URL names a hostname — but a port
// mismatch cannot be anything except a mistake, and the symptom is miserable to
// diagnose: the publisher sits in bootstrap mode forever, queueing account
// claims for a server running in its own process.
func checkPortsAgree(listenURL, configuredURL string) error {
	listenPort, err := portOf(listenURL)
	if err != nil {
		return fmt.Errorf("cannot parse embedded server address %q: %w", listenURL, err)
	}
	configuredPort, err := portOf(configuredURL)
	if err != nil {
		// Not our business to validate nats.server_url beyond this check.
		return nil
	}
	if listenPort == configuredPort {
		return nil
	}
	return fmt.Errorf(
		"embedded NATS server listens on port %s but nats.server_url is %s\n"+
			"       Nothing in this process would ever reach it. Make them agree:\n"+
			"       set nats.server_url to port %s, or re-export with --port %s",
		listenPort, configuredURL, listenPort, configuredPort)
}

func portOf(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	if p := u.Port(); p != "" {
		return p, nil
	}
	if u.Host == "" {
		return "", fmt.Errorf("no host in %q", rawURL)
	}
	return "4222", nil // NATS default when the URL omits a port
}

// Stop shuts the server down and waits for it to finish.
func (s *Server) Stop() {
	if s == nil || s.ns == nil {
		return
	}
	log.Println("ℹ️ Shutting down embedded NATS server")
	s.ns.Shutdown()
	s.ns.WaitForShutdown()
}

// logger adapts nats-server's Logger onto the standard library log package,
// which is what the rest of the binary uses.
//
// Fatalf deliberately does not exit. nats-server calls it and then returns from
// Start, so recording the message lets Start report a real reason instead of a
// bare timeout.
type logger struct {
	mu    sync.Mutex
	fatal string
}

func newLogger() *logger { return &logger{} }

func (l *logger) lastFatal() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.fatal
}

func (l *logger) Noticef(format string, v ...any) { log.Printf("nats: "+format, v...) }
func (l *logger) Warnf(format string, v ...any)   { log.Printf("nats: ⚠️  "+format, v...) }
func (l *logger) Errorf(format string, v ...any)  { log.Printf("nats: ❌ "+format, v...) }
func (l *logger) Debugf(format string, v ...any)  { log.Printf("nats: "+format, v...) }
func (l *logger) Tracef(format string, v ...any)  { log.Printf("nats: "+format, v...) }

func (l *logger) Fatalf(format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	l.mu.Lock()
	l.fatal = msg
	l.mu.Unlock()
	log.Printf("nats: ❌ %s", msg)
}
