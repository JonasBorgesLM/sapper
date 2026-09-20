package blastguard

import (
	"testing"

	"github.com/JonasBorgesLM/sapper/internal/core/model"
)

func TestNewRejectsUnknownTier(t *testing.T) {
	if _, err := New("bogus"); err == nil {
		t.Fatal("New(\"bogus\") error = nil, want an error for an unknown tier (SR-01)")
	}
}

func TestNewRejectsEmptyTier(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatal("New(\"\") error = nil, want an error — there is no default tier (SR-01)")
	}
}

func TestNewAllowsAuthorizedTiers(t *testing.T) {
	for _, tier := range []model.Tier{model.TierLab, model.TierStaging, model.TierAuthorized} {
		g, err := New(tier)
		if err != nil {
			t.Fatalf("New(%q) error = %v, want nil", tier, err)
		}
		if _, err := g.Acquire(); err != nil {
			t.Errorf("New(%q) then Acquire() error = %v, want an admitting guard", tier, err)
		}
	}
}

// SR-02: production needs the explicit opt-in AND interactive confirmation.
func TestNewProductionRequiresApproval(t *testing.T) {
	if _, err := New(model.TierProduction); err == nil {
		t.Fatal("New(production) with no approval = nil, want refusal (SR-02)")
	}
}

func TestNewProductionRefusedWhenNonInteractive(t *testing.T) {
	// A nil confirmer models a non-interactive context (CI): there is no way to
	// obtain human confirmation, so production must be refused.
	if _, err := New(model.TierProduction, WithProductionApproval(nil)); err == nil {
		t.Fatal("New(production) with a nil confirmer = nil, want refusal in a non-interactive context (SR-02)")
	}
}

func TestNewProductionRefusedWhenConfirmationDeclined(t *testing.T) {
	declined := func() (bool, error) { return false, nil }
	if _, err := New(model.TierProduction, WithProductionApproval(declined)); err == nil {
		t.Fatal("New(production) with a declined confirmation = nil, want refusal (SR-02)")
	}
}

func TestNewProductionAllowedWhenConfirmed(t *testing.T) {
	confirmed := func() (bool, error) { return true, nil }
	g, err := New(model.TierProduction, WithProductionApproval(confirmed))
	if err != nil {
		t.Fatalf("New(production) with confirmation error = %v, want nil", err)
	}
	if _, err := g.Acquire(); err != nil {
		t.Errorf("Acquire() error = %v, want an admitting guard", err)
	}
}
