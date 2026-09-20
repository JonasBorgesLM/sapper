package model

// Tier is the environment classification of a target, declared in config. It is
// the first line of BlastGuard's authorization (SR-01): Sapper refuses to emit
// load against a target whose tier is not one of the known values, and treats
// TierProduction specially (SR-02 — a separate flag and interactive confirm,
// enforced in the guard, not here).
type Tier string

const (
	TierLab        Tier = "lab"
	TierStaging    Tier = "staging"
	TierAuthorized Tier = "authorized"
	TierProduction Tier = "production"
)

// Valid reports whether t is one of the known tiers. An unknown or empty tier
// is refused at config load — there is no default (SR-01).
func (t Tier) Valid() bool {
	switch t {
	case TierLab, TierStaging, TierAuthorized, TierProduction:
		return true
	default:
		return false
	}
}

// IsProduction reports whether t is the production tier, which the guard gates
// behind an explicit flag and interactive confirmation (SR-02).
func (t Tier) IsProduction() bool {
	return t == TierProduction
}
