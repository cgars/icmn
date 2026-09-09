package httpapi

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/cgars/icmn/internal/identity"
)

type recordingRegistrar struct {
	patterns []string
}

func (r *recordingRegistrar) HandleFunc(pattern string, _ func(http.ResponseWriter, *http.Request)) {
	r.patterns = append(r.patterns, pattern)
}

func TestOpenAPIContractCoversRegisteredRoutes(t *testing.T) {
	raw, err := os.ReadFile("../../api/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var contract struct {
		OpenAPI string                                `json:"openapi"`
		Paths   map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(raw, &contract); err != nil {
		t.Fatalf("parse OpenAPI JSON: %v", err)
	}
	if !strings.HasPrefix(contract.OpenAPI, "3.1.") {
		t.Fatalf("openapi = %q, want 3.1.x", contract.OpenAPI)
	}

	registrar := &recordingRegistrar{}
	registerRoutes(registrar, &API{store: identity.NewMemoryStore()})
	if len(registrar.patterns) == 0 {
		t.Fatal("HTTP transport registered no routes")
	}
	for _, pattern := range registrar.patterns {
		method, path, ok := strings.Cut(pattern, " ")
		if !ok {
			t.Errorf("registered pattern %q does not name a method", pattern)
			continue
		}
		operations, ok := contract.Paths[path]
		if !ok {
			t.Errorf("contract missing registered path %s", path)
			continue
		}
		if _, ok := operations[strings.ToLower(method)]; !ok {
			t.Errorf("contract missing registered route %s", pattern)
		}
	}
}
