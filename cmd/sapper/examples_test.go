package main

import (
	"testing"

	"github.com/JonasBorgesLM/sapper/internal/adapters/config"
	"github.com/JonasBorgesLM/sapper/internal/core/scenario"
)

// The shipped example config and scenarios must always load, so a copy-paste
// starting point is never broken. Paths are relative to this package directory.
func TestShippedExamplesLoad(t *testing.T) {
	if _, err := config.Load("../../configs/sapper.example.yaml"); err != nil {
		t.Errorf("configs/sapper.example.yaml does not load: %v", err)
	}
	for _, name := range []string{"sustained", "ramp-up", "spike", "soak"} {
		if _, err := scenario.Load("../../scenarios/" + name + ".yaml"); err != nil {
			t.Errorf("scenarios/%s.yaml does not load: %v", name, err)
		}
	}
}
