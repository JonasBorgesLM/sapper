// Package httpclient is the real ports.Requester adapter: a thin wrapper around
// net/http.Client that enforces BlastGuard on every request before it is sent.
// Its only way to reach the network is Do, and Do always admits through the
// guard first — there is no other path and no unguarded constructor, so it is
// structurally impossible to generate load that bypasses the guard (SR-06).
package httpclient

import (
	"fmt"
	"net/http"

	"github.com/JonasBorgesLM/sapper/internal/core/blastguard"
	"github.com/JonasBorgesLM/sapper/internal/ports"
)

var _ ports.Requester = (*Client)(nil)

// Client is the production ports.Requester. Every request goes through
// BlastGuard.Acquire before the network.
//
// Two things belong here but land with the issues that make them testable and
// give them a source of truth: re-checking each redirect hop against the guard
// (net/http follows redirects internally without returning to Do) becomes
// meaningful with the tier/host gate (SR-01), and a per-request timeout is
// configured by the generator (a hung request otherwise pins a worker until the
// run's max-duration cap fires).
type Client struct {
	guard      *blastguard.BlastGuard
	httpClient *http.Client
}

// New builds a guarded Client. guard must not be nil — it is the whole point of
// this adapter, so a nil guard is a programming error, not a runtime condition.
// If httpClient is nil a plain *http.Client is used.
func New(guard *blastguard.BlastGuard, httpClient *http.Client) *Client {
	if guard == nil {
		panic("httpclient: guard must not be nil — there is no unguarded client (SR-06)")
	}
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	return &Client{guard: guard, httpClient: httpClient}
}

// Do admits the request through BlastGuard, then delegates to net/http. A
// blocked request returns before dialing and never reaches the network.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	release, err := c.guard.Acquire()
	if err != nil {
		return nil, fmt.Errorf("httpclient: request blocked: %w", err)
	}
	defer release()
	// Not an SSRF sink: the URL is the operator's own --config/--scenario input,
	// and where a request may go is governed by BlastGuard's tier gate (SR-01),
	// not by gosec's taint analysis. This is a load tool whose entire purpose is
	// to call the target the operator authorised.
	return c.httpClient.Do(req) // #nosec G704

}
