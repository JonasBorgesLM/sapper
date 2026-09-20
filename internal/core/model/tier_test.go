package model

import "testing"

func TestTierValid(t *testing.T) {
	valid := []Tier{TierLab, TierStaging, TierAuthorized, TierProduction}
	for _, tier := range valid {
		if !tier.Valid() {
			t.Errorf("Tier(%q).Valid() = false, want true", tier)
		}
	}
}

func TestTierInvalid(t *testing.T) {
	invalid := []Tier{"", "prod", "dev", "LAB", "authorised"}
	for _, tier := range invalid {
		if tier.Valid() {
			t.Errorf("Tier(%q).Valid() = true, want false", tier)
		}
	}
}

func TestTierIsProduction(t *testing.T) {
	if !TierProduction.IsProduction() {
		t.Error("TierProduction.IsProduction() = false, want true")
	}
	for _, tier := range []Tier{TierLab, TierStaging, TierAuthorized} {
		if tier.IsProduction() {
			t.Errorf("Tier(%q).IsProduction() = true, want false", tier)
		}
	}
}
