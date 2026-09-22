// Package ports declares the interfaces the core depends on, so the core can be
// tested against fakes with no real network. Its one interface today is
// Requester, the sole boundary outbound load crosses — the only implementation
// reaching the network is the guarded client in internal/adapters/httpclient,
// which is what makes "no load bypasses the guard" structural (SR-06).
package ports
