package ports

import "net/http"

// Requester is the sole boundary outbound load may cross. The real adapter
// (internal/adapters/httpclient) wires BlastGuard in as middleware around Do,
// so no request reaches the network without passing a guard decision (SR-06).
// Core code is handed a Requester and has no way to construct an unguarded one;
// tests substitute a fake with canned responses.
type Requester interface {
	Do(req *http.Request) (*http.Response, error)
}
