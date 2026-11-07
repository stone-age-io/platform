// Package health is the readiness-check engine behind `stone-age serve`.
//
// The same package lives in the agent repo, copied on purpose rather than
// extracted into a shared module: two small copies that drift are cheaper than
// a third repo to version, and the two processes check different things.
//
// PocketBase's /api/health only says the HTTP server is listening, which stays
// true through every failure that matters here (an untrusted operator,
// `bootstrap` before `migrate up`). These checks target what is silently wrong,
// not liveness.
//
// A check must be answerable first-hand by the process running it. The Control
// Plane holds no credential in any tenant's account, so it cannot read an org
// KV. Do not give it one to improve a check: that would make the credential
// issuer a data-plane participant in every tenant's bus.
package health

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// State is a single check's verdict. Only Fail makes the process unready.
//
//   - Warn is running but misconfigured (at-rest encryption off). Failing on it
//     would refuse to serve stock dev deployments.
//   - Skipped means the check does not apply here. OK would claim a check ran.
type State string

const (
	StateOK      State = "ok"
	StateWarn    State = "warn"
	StateFail    State = "fail"
	StateSkipped State = "skipped"
)

// severity orders states so a report can take the worst one. Skipped ranks
// below OK: it is the absence of an answer, never a better one.
func (s State) severity() int {
	switch s {
	case StateFail:
		return 3
	case StateWarn:
		return 2
	case StateOK:
		return 1
	default:
		return 0
	}
}

// Result is one check's outcome. Every non-OK result should carry a Fix: the
// exact command for someone who just deployed and does not know the bootstrap
// order.
type Result struct {
	Name   string `json:"name"`
	State  State  `json:"state"`
	Detail string `json:"detail,omitempty"`
	Fix    string `json:"fix,omitempty"`
	// Took is how long this check ran, filled in by Registry.Run.
	Took string `json:"took,omitempty"`
}

// OK, Warn, Fail and Skip build results. The registry supplies the name.
func OK(detail string) Result   { return Result{State: StateOK, Detail: detail} }
func Skip(detail string) Result { return Result{State: StateSkipped, Detail: detail} }

func Warn(detail, fix string) Result {
	return Result{State: StateWarn, Detail: detail, Fix: fix}
}

func Fail(detail, fix string) Result {
	return Result{State: StateFail, Detail: detail, Fix: fix}
}

// Func is a single check. It must respect ctx's deadline, or it can hang a
// readiness probe.
type Func func(ctx context.Context) Result

// Registry holds the checks a process runs. The zero value is usable.
type Registry struct {
	mu     sync.Mutex
	names  []string
	checks map[string]Func
}

// Register adds a check. The same name twice replaces the first, so one label
// never covers two checks.
func (r *Registry) Register(name string, fn Func) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.checks == nil {
		r.checks = make(map[string]Func)
	}
	if _, dup := r.checks[name]; !dup {
		r.names = append(r.names, name)
	}
	r.checks[name] = fn
}

// Report is the whole answer: one Result per registered check, plus the summary
// fields a probe or a human reads first.
type Report struct {
	// Ready is false if and only if some check failed.
	Ready bool `json:"ready"`
	// State is the worst state across all checks.
	State   State     `json:"state"`
	Version string    `json:"version"`
	Uptime  string    `json:"uptime"`
	Took    string    `json:"took"`
	Checked time.Time `json:"checked"`
	Checks  []Result  `json:"checks"`
}

// Run executes every registered check concurrently, so a probe waits for the
// slowest check rather than the sum. Each gets the caller's context; give it a
// deadline. A panicking check is reported as a failure, not a crash.
func (r *Registry) Run(ctx context.Context, version string, uptime time.Duration) *Report {
	r.mu.Lock()
	names := append([]string(nil), r.names...)
	checks := make(map[string]Func, len(r.checks))
	for k, v := range r.checks {
		checks[k] = v
	}
	r.mu.Unlock()

	started := time.Now()
	results := make([]Result, len(names))

	var wg sync.WaitGroup
	for i, name := range names {
		wg.Add(1)
		go func(i int, name string, fn Func) {
			defer wg.Done()
			checkStart := time.Now()
			res := runSafely(ctx, fn)
			res.Name = name
			res.Took = time.Since(checkStart).Round(time.Millisecond).String()
			results[i] = res
		}(i, name, checks[name])
	}
	wg.Wait()

	// Sorted so the output is stable between calls.
	sort.Slice(results, func(i, j int) bool { return results[i].Name < results[j].Name })

	// Seeded from the first result, not StateOK: skipped ranks below OK, so an
	// all-skipped report must not come back "ok". Only an empty registry is OK.
	worst := StateOK
	if len(results) > 0 {
		worst = results[0].State
	}
	ready := true
	for _, res := range results {
		if res.State.severity() > worst.severity() {
			worst = res.State
		}
		if res.State == StateFail {
			ready = false
		}
	}

	return &Report{
		Ready:   ready,
		State:   worst,
		Version: version,
		Uptime:  uptime.Round(time.Second).String(),
		Checked: started.UTC(),
		Took:    time.Since(started).Round(time.Millisecond).String(),
		Checks:  results,
	}
}

// runSafely turns a panic inside a check into a failed result.
func runSafely(ctx context.Context, fn Func) (res Result) {
	defer func() {
		if p := recover(); p != nil {
			res = Fail(
				fmt.Sprintf("check panicked: %v", p),
				"This is a bug in the check itself, not necessarily in the thing it inspects. Please report it.",
			)
		}
	}()
	if fn == nil {
		return Skip("no check registered")
	}
	return fn(ctx)
}
