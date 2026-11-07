// Package natsd runs a NATS server inside the Control Plane process.
//
// It is an ordinary nats-server reading the nats.conf that `stone-age nats
// export` writes, exactly as `nats-server -c nats.conf` would. There is no
// embedded-only mode and no options derived in Go, and account claims still
// arrive over $SYS. That keeps one code path and makes moving to an external
// server a config change.
//
// A `cluster` block is honoured like any other directive, so a Control Plane can
// be one node of a cluster of external nats-servers
// (TestEmbeddedServerClustersWithAPeer). Restarting the platform then takes that
// node down with it; run the bus as a separate process (the default) where that
// is the wrong trade.
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
	// readyTimeout is generous because JetStream recovers its store on startup,
	// and a large store on slow disk is not an error.
	readyTimeout = 30 * time.Second

	// defaultConfigDir is the output directory the docs use for `nats export`.
	defaultConfigDir = "./nats-config"

	// DefaultConfigFile is the config --nats loads unless --nats-config says
	// otherwise.
	DefaultConfigFile = defaultConfigDir + "/nats.conf"
)

// Server wraps an embedded nats-server.
type Server struct {
	ns *natsserver.Server
}

// Start loads confPath, starts a NATS server from it, and waits for it to
// accept connections.
//
// clientURL is the address the calling process dials. It is only checked against
// the port the server listens on, since a mismatch leaves the process unable to
// reach its own server. clientURLSetting names the config key it came from
// (`nats.server_url` in the Control Plane, `nats.local_url` on an edge), for
// the error message.
//
// The returned error is meant to be fatal to startup.
func Start(confPath, clientURL, clientURLSetting string) (*Server, error) {
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

	// PocketBase owns SIGINT/SIGTERM; otherwise the two fight over shutdown.
	opts.NoSigs = true

	ns, err := natsserver.NewServer(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create NATS server: %w", err)
	}

	// One log stream: set before Start so nothing is written elsewhere first.
	// A `log_file` in nats.conf is therefore not honoured.
	logger := newLogger()
	ns.SetLogger(logger, opts.Debug, opts.Trace)

	go ns.Start()

	if err := waitReady(ns, logger); err != nil {
		ns.Shutdown()
		ns.WaitForShutdown()
		return nil, err
	}

	if err := checkPortsAgree(ns.ClientURL(), clientURL, clientURLSetting, confPath); err != nil {
		ns.Shutdown()
		ns.WaitForShutdown()
		return nil, err
	}

	log.Printf("✅ Embedded NATS server listening on %s (config: %s)", ns.ClientURL(), confPath)
	return &Server{ns: ns}, nil
}

// waitReady blocks until the server accepts connections, or fails.
//
// It polls rather than waiting out readyTimeout, because a server can log a
// fatal and never become ready, and nats-server's Start() then just returns. The
// recorded fatal is the only diagnosis, and it is reported at once.
func waitReady(ns *natsserver.Server, logger *logger) error {
	deadline := time.Now().Add(readyTimeout)
	for {
		if ns.ReadyForConnections(250 * time.Millisecond) {
			// Ready but with a failed subsystem: refuse, since a silently
			// missing JetStream means twins never persist.
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
// from the one the calling process dials.
//
// Only the port is compared: hosts differ legitimately (the server binds
// 0.0.0.0, the URL names a hostname), but a port mismatch is always a mistake.
func checkPortsAgree(listenURL, configuredURL, setting, confPath string) error {
	listenPort, err := portOf(listenURL)
	if err != nil {
		return fmt.Errorf("cannot parse embedded server address %q: %w", listenURL, err)
	}
	configuredPort, err := portOf(configuredURL)
	if err != nil {
		// Validating the caller's URL is not this check's job.
		return nil
	}
	if listenPort == configuredPort {
		return nil
	}
	return fmt.Errorf(
		"embedded NATS server listens on port %s but %s is %s\n"+
			"       Nothing in this process would ever reach it. Make them agree:\n"+
			"       set %s to port %s, or set `port: %s` in %s",
		listenPort, setting, configuredURL, setting, listenPort, configuredPort, confPath)
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

// Stats is the embedded server's own view of itself, for the metrics endpoint.
//
// Everything here is server-level, not account-level, so it needs no credential
// inside a tenant's account and the Control Plane may publish it.
//
// Leafnodes counts leaf connections across every account: a capacity number,
// not per-tenant availability. A tenant asks about its own sites through
// $SYS.REQ.ACCOUNT.PING.CONNZ.
type Stats struct {
	Connections int
	Leafnodes   int
	// Routes is peer connections in the same cluster, non-zero when nats.conf
	// has a `cluster` block (TestEmbeddedServerClustersWithAPeer).
	Routes        int
	SlowConsumers int64
	InMsgs        int64
	OutMsgs       int64
	InBytes       int64
	OutBytes      int64

	JetStreamEnabled bool
	JetStreamMemory  uint64
	JetStreamStore   uint64
}

// Stats reports the embedded server's counters. The second return is false when
// no embedded server is running, so the caller can leave the metrics out rather
// than export zeros that look like an idle server.
func (s *Server) Stats() (*Stats, bool) {
	if s == nil || s.ns == nil {
		return nil, false
	}
	vz, err := s.ns.Varz(nil)
	if err != nil || vz == nil {
		return nil, false
	}
	st := &Stats{
		Connections:   vz.Connections,
		Leafnodes:     vz.Leafs,
		Routes:        vz.Routes,
		SlowConsumers: vz.SlowConsumers,
		InMsgs:        vz.InMsgs,
		OutMsgs:       vz.OutMsgs,
		InBytes:       vz.InBytes,
		OutBytes:      vz.OutBytes,
	}
	if js := vz.JetStream.Stats; js != nil {
		st.JetStreamEnabled = true
		st.JetStreamMemory = js.Memory
		st.JetStreamStore = js.Store
	}
	return st, true
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

// logger adapts nats-server's Logger onto the standard library log package.
// Fatalf does not exit: nats-server calls it and then returns from Start, so the
// recorded message lets Start report a real reason instead of a timeout.
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
