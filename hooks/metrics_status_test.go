package hooks

import (
	"errors"
	"testing"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/tools/router"
)

// An API-rule rejection must be counted as a 4xx.
//
// This is the bug the split-out function exists to pin. PocketBase's router runs
// its ErrorHandler AFTER the middleware chain unwinds, so a handler that returns
// an error leaves the tracked status at 0 by the time the metrics middleware
// sees it. The previous code answered "5xx" for every such case -- and EVERY
// authorization rejection arrives that way: 400 on a denied create, 404 on a
// denied update, 401/403 from RequireAuth. The 4xx bucket sat near-empty while a
// 5xx alert fired on a platform doing exactly its job.
//
// The statuses below are this platform's own documented conventions, which is
// why they are the cases worth naming.
func TestStatusClassCountsRuleRejectionsAsClientErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		err    error
		want   string
	}{
		{
			name: "denied create (PocketBase answers 400)",
			err:  apis.NewBadRequestError("", nil),
			want: "4xx",
		},
		{
			name: "denied update (PocketBase answers 404, not 403, deliberately)",
			err:  apis.NewNotFoundError("", nil),
			want: "4xx",
		},
		{
			name: "unauthenticated (RequireAuth)",
			err:  apis.NewUnauthorizedError("", nil),
			want: "4xx",
		},
		{
			name: "forbidden",
			err:  apis.NewForbiddenError("", nil),
			want: "4xx",
		},
		{
			name: "a genuine server error is still 5xx",
			err:  apis.NewInternalServerError("", nil),
			want: "5xx",
		},
		{
			name:   "a written status wins over the error",
			status: 201,
			err:    nil,
			want:   "2xx",
		},
		{
			name:   "no status and no error is not a guess",
			status: 0,
			err:    nil,
			want:   "unknown",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := statusClassFor(tc.status, tc.err); got != tc.want {
				t.Errorf("statusClassFor(%d, %v) = %q, want %q", tc.status, tc.err, got, tc.want)
			}
		})
	}
}

// A plain error carries no status of its own. PocketBase's ErrorHandler resolves
// it through the same ToApiError call used here, which generalises it to 400 --
// so this asserts the bucket matches what the client is actually sent, rather
// than what a reader might assume an unclassified error means.
func TestStatusClassMatchesWhatTheRouterWouldSend(t *testing.T) {
	plain := errors.New("something went wrong")

	sent := router.ToApiError(plain).Status
	want := "4xx"
	if sent >= 500 {
		want = "5xx"
	}

	if got := statusClassFor(0, plain); got != want {
		t.Errorf("statusClassFor(0, plain error) = %q, but the router would send %d (%s)", got, sent, want)
	}
}
