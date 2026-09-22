package probe

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractFieldDotPath(t *testing.T) {
	body := []byte(`{"memstats":{"Alloc":123456,"HeapObjects":42},"goroutines":17}`)
	cases := []struct {
		path string
		want float64
	}{
		{"goroutines", 17},
		{"memstats.Alloc", 123456},
		{"memstats.HeapObjects", 42},
	}
	for _, c := range cases {
		got, err := extractField(body, c.path)
		if err != nil {
			t.Errorf("extractField(%q) error = %v", c.path, err)
			continue
		}
		if got != c.want {
			t.Errorf("extractField(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestExtractFieldErrors(t *testing.T) {
	body := []byte(`{"a":{"b":1},"s":"text"}`)
	for _, path := range []string{"missing", "a.missing", "s", "a.b.c"} {
		if _, err := extractField(body, path); err == nil {
			t.Errorf("extractField(%q) = nil error, want an error", path)
		}
	}
}

func TestProbeSamplesEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"goroutines":25}`))
	}))
	defer srv.Close()

	v, err := Sample(srv.URL, "goroutines")
	if err != nil {
		t.Fatalf("Sample() error = %v", err)
	}
	if v != 25 {
		t.Errorf("Sample() = %v, want 25", v)
	}
}
