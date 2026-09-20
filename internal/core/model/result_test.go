package model

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestResultJSONSchemaFieldNames(t *testing.T) {
	r := Result{
		Target:    TargetEcho{BaseURL: "http://localhost:8080", Tier: TierLab, Caps: Caps{MaxConcurrency: 50, MaxDuration: time.Minute}},
		Scenario:  "sustained-baseline",
		Profile:   "sustained",
		StartedAt: time.Unix(0, 0).UTC(),
		Aborted:   false,
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	got := string(b)
	for _, key := range []string{`"base_url"`, `"tier"`, `"max_concurrency"`, `"scenario"`, `"profile"`, `"aborted"`} {
		if !strings.Contains(got, key) {
			t.Errorf("result.json missing schema key %s in %s", key, got)
		}
	}
}

func TestResultRoundTrip(t *testing.T) {
	r := Result{Target: TargetEcho{Tier: TierStaging}, Scenario: "s", Aborted: true, AbortReason: "cap exceeded"}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var back Result
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if back.Target.Tier != TierStaging || back.Scenario != "s" || !back.Aborted || back.AbortReason != "cap exceeded" {
		t.Errorf("round-trip lost data: %+v", back)
	}
}
