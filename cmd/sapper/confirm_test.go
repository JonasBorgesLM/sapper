package main

import (
	"strings"
	"testing"
)

func TestConfirmProduction(t *testing.T) {
	// M2: not a terminal (CI, piped, redirected) — refuse even if the input says
	// "yes", so `echo yes | sapper run` cannot clear the production gate.
	if ok, _ := confirmProduction(strings.NewReader("yes\n"), false); ok {
		t.Error("confirmProduction(yes, isTTY=false) = true; must refuse without a terminal (M2)")
	}
	// At a terminal, an explicit "yes" confirms; anything else declines.
	if ok, _ := confirmProduction(strings.NewReader("yes\n"), true); !ok {
		t.Error("confirmProduction(yes, isTTY=true) = false; want true")
	}
	if ok, _ := confirmProduction(strings.NewReader("no\n"), true); ok {
		t.Error("confirmProduction(no, isTTY=true) = true; want false")
	}
}
