//go:build e2e

// Package e2e drives the built stone-age binary the way a customer runs it and
// asks the real NATS server and the real Nebula code whether the result works.
// See plans/e2e-suite.md.
//
//	go test -tags e2e -count=1 ./e2e/
//	E2E_KEEP=1 go test -tags e2e -count=1 ./e2e/   # keep the temp dir and server log
package e2e

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

const (
	suEmail    = "su@e2e.test"
	suPassword = "e2e-password-123"
)

// platform is the one server every test in the package shares. Each test makes
// its own organization, so tests do not see each other's records.
var platform *Platform

func TestMain(m *testing.M) {
	work, err := os.MkdirTemp("", "stone-age-e2e-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	p, err := startPlatform(work)
	if err != nil {
		fmt.Fprintln(os.Stderr, "e2e: platform did not start:", err)
		if p != nil {
			p.stop()
			p.dumpLog()
		}
		os.Exit(1)
	}
	platform = p

	code := m.Run()

	p.stop()
	if code != 0 {
		p.dumpLog()
	}
	if os.Getenv("E2E_KEEP") == "1" {
		fmt.Println("e2e: kept", work)
	} else {
		os.RemoveAll(work)
	}
	os.Exit(code)
}

// Platform is one running stone-age process with an embedded NATS server.
type Platform struct {
	Dir     string
	API     string // http://127.0.0.1:<port>/api
	NatsURL string // nats://127.0.0.1:<port>
	SUToken string // superuser token: API rules do not apply

	bin     string
	logPath string
	cmd     *exec.Cmd
}

func startPlatform(work string) (*Platform, error) {
	p := &Platform{
		Dir:     work,
		bin:     filepath.Join(work, "stone-age"+exeSuffix()), // .exe on Windows (see exeSuffix)
		logPath: filepath.Join(work, "server.log"),
	}

	build := exec.Command("go", "build", "-o", p.bin, ".")
	build.Dir = ".."
	if out, err := build.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("go build: %v\n%s", err, out)
	}

	ports, err := freePorts(4)
	if err != nil {
		return nil, err
	}
	httpPort, natsPort, wsPort, leafPort := ports[0], ports[1], ports[2], ports[3]
	p.API = fmt.Sprintf("http://127.0.0.1:%d/api", httpPort)
	p.NatsURL = fmt.Sprintf("nats://127.0.0.1:%d", natsPort)

	natsDir := filepath.Join(work, "nats-config")
	env := append(os.Environ(),
		"STONE_AGE_BOOTSTRAP_PASSWORD="+suPassword,
		"STONE_AGE_NATS_LOG_TO_CONSOLE=true",
		"STONE_AGE_NATS_SERVER_URL="+p.NatsURL,
		"STONE_AGE_NATS_EMBEDDED_CONFIG="+filepath.Join(natsDir, "nats.conf"),
		fmt.Sprintf("STONE_AGE_NATS_LEAF_URL=nats-leaf://127.0.0.1:%d", leafPort),
	)
	dirFlag := "--dir=" + filepath.Join(work, "pb_data")

	// The first-run path a customer takes.
	if err := p.run(env, dirFlag, "init", "--email", suEmail, "--operator-org", "E2E Operator"); err != nil {
		return nil, err
	}
	// init always exports the default ports, so export again onto free ones.
	// Same operator and account JWTs; only the listeners in nats.conf change.
	if err := p.run(env, dirFlag, "nats", "export", "--output", natsDir,
		"--port", fmt.Sprint(natsPort),
		"--websocket-port", fmt.Sprint(wsPort),
		"--leafnode-port", fmt.Sprint(leafPort),
	); err != nil {
		return nil, err
	}

	logFile, err := os.Create(p.logPath)
	if err != nil {
		return nil, err
	}
	p.cmd = exec.Command(p.bin, dirFlag, "--dev=false", "serve", "--nats", "--http", fmt.Sprintf("127.0.0.1:%d", httpPort))
	p.cmd.Dir = work
	p.cmd.Env = env
	p.cmd.Stdout = logFile
	p.cmd.Stderr = logFile
	if err := p.cmd.Start(); err != nil {
		return nil, err
	}

	if err := p.waitReady(60 * time.Second); err != nil {
		return p, err
	}

	var auth struct {
		Token string `json:"token"`
	}
	status, body, err := p.request("POST", "/collections/_superusers/auth-with-password", "",
		map[string]string{"identity": suEmail, "password": suPassword}, &auth)
	if err != nil || status != 200 || auth.Token == "" {
		return p, fmt.Errorf("superuser auth: status %d, err %v, body %s", status, err, body)
	}
	p.SUToken = auth.Token
	return p, nil
}

// run runs one short-lived command against the platform's data directory.
func (p *Platform) run(env []string, args ...string) error {
	cmd := exec.Command(p.bin, args...)
	cmd.Dir = p.Dir
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("stone-age %v: %v\n%s", args, err, out)
	}
	return nil
}

// waitReady waits for /api/ready to answer 200. It answers 503 only on a
// failed check, so 200 means every check is ok or merely warning.
func (p *Platform) waitReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	last := "no answer"
	for time.Now().Before(deadline) {
		if p.cmd.ProcessState != nil {
			return fmt.Errorf("serve exited: %v", p.cmd.ProcessState)
		}
		resp, err := http.Get(p.API + "/ready")
		if err == nil {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return nil
			}
			last = fmt.Sprintf("%d %s", resp.StatusCode, b)
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("/api/ready never answered 200 (last: %s)", last)
}

func (p *Platform) stop() {
	if p.cmd == nil || p.cmd.Process == nil {
		return
	}
	_ = p.cmd.Process.Signal(os.Interrupt)
	done := make(chan struct{})
	go func() { _ = p.cmd.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		_ = p.cmd.Process.Kill()
		<-done
	}
}

// dumpLog prints the tail of the server log, which is where the reason for a
// failure usually is.
func (p *Platform) dumpLog() {
	f, err := os.Open(p.logPath)
	if err != nil {
		return
	}
	defer f.Close()
	var lines []string
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 1024*1024), 1024*1024)
	for s.Scan() {
		lines = append(lines, s.Text())
	}
	if len(lines) > 150 {
		lines = lines[len(lines)-150:]
	}
	fmt.Fprintln(os.Stderr, "--- server log (tail) ---")
	for _, l := range lines {
		fmt.Fprintln(os.Stderr, l)
	}
}

// request sends a JSON request and decodes a JSON answer into out (if non-nil).
// It returns the status and the raw body so a failure can say what came back.
func (p *Platform) request(method, path, token string, body, out any) (int, []byte, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, p.API+path, r)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	if out != nil && resp.StatusCode < 300 {
		if err := json.Unmarshal(raw, out); err != nil {
			return resp.StatusCode, raw, fmt.Errorf("decode: %w", err)
		}
	}
	return resp.StatusCode, raw, nil
}

// mustRequest is request for tests: anything but wantStatus fails the test.
func (p *Platform) mustRequest(t *testing.T, wantStatus int, method, path, token string, body, out any) {
	t.Helper()
	status, raw, err := p.request(method, path, token, body, out)
	if err != nil || status != wantStatus {
		t.Fatalf("%s %s: status %d (want %d), err %v, body %s", method, path, status, wantStatus, err, raw)
	}
}

// eventually retries fn until it returns nil or the timeout passes. pb-nats
// publishes claims after a debounce, so most bus assertions need it.
func eventually(t *testing.T, timeout time.Duration, fn func() error) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var err error
	for {
		if err = fn(); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("not true after %s: %v", timeout, err)
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// holds is the negative of eventually: fn must keep returning nil for the whole
// window. Use it where the bug is something coming back after a delay, such as
// an account republished after pb-nats's 3s debounce.
func holds(t *testing.T, window time.Duration, fn func() error) {
	t.Helper()
	deadline := time.Now().Add(window)
	for time.Now().Before(deadline) {
		if err := fn(); err != nil {
			t.Fatalf("stopped holding: %v", err)
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// freePorts asks the kernel for n free TCP ports. They are released before use,
// so another process could take one in between; on a CI runner that is rare
// enough not to engineer around.
func freePorts(n int) ([]int, error) {
	var ports []int
	var listeners []net.Listener
	defer func() {
		for _, l := range listeners {
			l.Close()
		}
	}()
	for i := 0; i < n; i++ {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, err
		}
		listeners = append(listeners, l)
		ports = append(ports, l.Addr().(*net.TCPAddr).Port)
	}
	return ports, nil
}

// exeSuffix is ".exe" on Windows: go build adds it, and exec.Command looks for
// it, so a bare name is not found there.
func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}
