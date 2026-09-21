package openapi

import "testing"

func TestLoadExtractsPathMethodPairs(t *testing.T) {
	eps, err := Load("testdata/api.yaml")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	// sorted by path then method; the `parameters` key is not a method.
	want := []Endpoint{
		{Path: "/users", Method: "GET"},
		{Path: "/users", Method: "POST"},
		{Path: "/users/{id}", Method: "GET"},
	}
	if len(eps) != len(want) {
		t.Fatalf("got %d endpoints %+v, want %d", len(eps), eps, len(want))
	}
	for i := range want {
		if eps[i] != want[i] {
			t.Errorf("endpoint %d = %+v, want %+v", i, eps[i], want[i])
		}
	}
}

func TestHas(t *testing.T) {
	eps, _ := Load("testdata/api.yaml")
	if !Has(eps, "GET", "/users") {
		t.Error("Has(GET /users) = false, want true")
	}
	if Has(eps, "DELETE", "/users") {
		t.Error("Has(DELETE /users) = true, want false")
	}
}
