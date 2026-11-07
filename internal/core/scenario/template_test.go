package scenario

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "s.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

const base = `
name: t
profile: {type: sustained, concurrency: 1, duration: 1s}
`

func TestRenderSubstitutesEveryOccurrence(t *testing.T) {
	got := Render("/a/{{uuid}}/{{ seq }}?k={{uuid}}", Vars{UUID: "U", Seq: 7})
	if got != "/a/U/7?k=U" {
		t.Errorf("Render() = %q", got)
	}
}

func TestNewUUIDIsV4AndDistinct(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		u := NewUUID()
		if !re.MatchString(u) {
			t.Fatalf("NewUUID() = %q, not a v4 UUID", u)
		}
		if seen[u] {
			t.Fatalf("NewUUID() repeated %q", u)
		}
		seen[u] = true
	}
}

func TestLoadBodyMapIsJSON(t *testing.T) {
	s, err := Load(write(t, base+`
request:
  method: POST
  body: {id: "bet-{{uuid}}", amount: 100}
slos: {status_absent: 500}
`))
	if err != nil {
		t.Fatal(err)
	}
	if !s.Request.Body.JSON || s.Request.Body.Text != `{"amount":100,"id":"bet-{{uuid}}"}` {
		t.Errorf("Body = %+v", s.Request.Body)
	}
}

func TestLoadBodyStringIsSentAsWritten(t *testing.T) {
	s, err := Load(write(t, base+`
request: {method: POST, body: "a=1&b={{seq}}"}
slos: {status_absent: 500}
`))
	if err != nil {
		t.Fatal(err)
	}
	if s.Request.Body.JSON || s.Request.Body.Text != "a=1&b={{seq}}" {
		t.Errorf("Body = %+v", s.Request.Body)
	}
}

func TestLoadRejectsBodyOfWrongKind(t *testing.T) {
	if _, err := Load(write(t, base+"request: {body: 12}\nslos: {status_absent: 500}\n")); err == nil {
		t.Fatal("Load() accepted a numeric body")
	}
}

// A typo must fail the load: sent literally it would make every request
// identical, which an idempotent target answers once and then deduplicates.
func TestLoadRejectsUnknownTemplateVariable(t *testing.T) {
	for name, req := range map[string]string{
		"body":   `{body: {id: "{{uid}}"}}`,
		"path":   `{path: "/x/{{nope}}"}`,
		"header": `{headers: {Idempotency-Key: "{{UUID}}"}}`,
	} {
		_, err := Load(write(t, base+"request: "+req+"\nslos: {status_absent: 500}\n"))
		if err == nil || !strings.Contains(err.Error(), "unknown template variable") {
			t.Errorf("%s: error = %v, want an unknown template variable", name, err)
		}
	}
}

func TestLoadStatusSLOs(t *testing.T) {
	s, err := Load(write(t, base+`
slos:
  status_absent: 500
  status_fraction_under: {status: 503, fraction: 0.05}
`))
	if err != nil {
		t.Fatal(err)
	}
	if *s.SLOs.StatusAbsent != 500 || s.SLOs.StatusFractionUnder.Status != 503 || s.SLOs.StatusFractionUnder.Fraction != 0.05 {
		t.Errorf("SLOs = %+v", s.SLOs)
	}
}

// Each new SLO alone satisfies "at least one SLO" — Empty() must know them.
func TestEmptyKnowsTheStatusSLOs(t *testing.T) {
	c := 500
	if (SLOs{StatusAbsent: &c}).Empty() || (SLOs{StatusFractionUnder: &StatusFraction{}}).Empty() {
		t.Error("Empty() = true for a declared status SLO")
	}
}

func TestLoadRejectsBadStatusSLOValues(t *testing.T) {
	for _, slos := range []string{
		"{status_absent: 42}",
		"{status_fraction_under: {status: 99, fraction: 0.1}}",
		"{status_fraction_under: {status: 503, fraction: 0}}",
		"{status_fraction_under: {status: 503, fraction: 1.5}}",
	} {
		if _, err := Load(write(t, base+"slos: "+slos+"\n")); err == nil {
			t.Errorf("Load() accepted slos %s", slos)
		}
	}
}
