// Package openapi resolves the endpoints of an OpenAPI spec into (path, method)
// targets, so a scenario can name an endpoint in the target's own contract
// (IR-01). It parses the spec's `paths` section with the YAML library rather
// than depending on a full OpenAPI parser — see ADR-0008 for what that does and
// does not cover ($ref indirection and schema-derived bodies are not handled).
package openapi

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// Endpoint is one path/method the spec declares.
type Endpoint struct {
	Path   string
	Method string
}

// httpMethods are the keys under a path item that name an operation; anything
// else (parameters, summary, servers, …) is not an endpoint.
var httpMethods = map[string]bool{
	"get": true, "post": true, "put": true, "delete": true,
	"patch": true, "head": true, "options": true, "trace": true,
}

// Load reads an OpenAPI spec (YAML or JSON, which the YAML parser also reads) and
// returns its endpoints, sorted by path then method for deterministic output.
func Load(path string) ([]Endpoint, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- operator-supplied spec path
	if err != nil {
		return nil, fmt.Errorf("openapi: read %s: %w", path, err)
	}

	var doc struct {
		Paths map[string]map[string]yaml.Node `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("openapi: parse %s: %w", path, err)
	}
	if len(doc.Paths) == 0 {
		return nil, fmt.Errorf("openapi: %s declares no paths", path)
	}

	var eps []Endpoint
	for p, item := range doc.Paths {
		for method := range item {
			if httpMethods[strings.ToLower(method)] {
				eps = append(eps, Endpoint{Path: p, Method: strings.ToUpper(method)})
			}
		}
	}
	slices.SortFunc(eps, func(a, b Endpoint) int {
		if a.Path != b.Path {
			return strings.Compare(a.Path, b.Path)
		}
		return strings.Compare(a.Method, b.Method)
	})
	return eps, nil
}

// Has reports whether the endpoints include the given method and path, so a run
// can validate a scenario's target against the imported spec.
func Has(eps []Endpoint, method, path string) bool {
	for _, e := range eps {
		if e.Method == strings.ToUpper(method) && e.Path == path {
			return true
		}
	}
	return false
}
