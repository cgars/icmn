package httpapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/cgars/icmn/internal/httpapi"
	"github.com/cgars/icmn/internal/identity"
)

type errorResponse struct {
	Error string `json:"error"`
}

func TestHealth(t *testing.T) {
	res := request(t, httpapi.New(identity.NewMemoryStore()), http.MethodGet, "/healthz", nil)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	var got map[string]string
	decodeResponse(t, res, &got)
	if got["status"] != "ok" {
		t.Fatalf("status field = %q, want ok", got["status"])
	}
}

func TestCreateAndRetrieveEntity(t *testing.T) {
	h := httpapi.New(identity.NewMemoryStore())
	created := createEntity(t, h, "organization")
	if created.Kind != "organization" || !strings.HasPrefix(created.ID, "ent_") || created.CreatedAt.IsZero() {
		t.Fatalf("unexpected entity: %+v", created)
	}

	res := request(t, h, http.MethodGet, "/v1/entities/"+created.ID, nil)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	var got identity.Entity
	decodeResponse(t, res, &got)
	if got.ID != created.ID || got.Kind != created.Kind || !got.CreatedAt.Equal(created.CreatedAt) {
		t.Fatalf("retrieved entity = %+v, want %+v", got, created)
	}
}

func TestInvalidRequests(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "malformed JSON", body: `{"kind":`},
		{name: "missing kind", body: `{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := request(t, httpapi.New(identity.NewMemoryStore()), http.MethodPost, "/v1/entities", strings.NewReader(tt.body))
			assertError(t, res, http.StatusBadRequest)
		})
	}
}

func TestUnknownEntity(t *testing.T) {
	res := request(t, httpapi.New(identity.NewMemoryStore()), http.MethodGet, "/v1/entities/ent_00000000000000000000000000000000", nil)
	assertError(t, res, http.StatusNotFound)
}

func TestAddExternalReference(t *testing.T) {
	h := httpapi.New(identity.NewMemoryStore())
	e := createEntity(t, h, "organization")
	body := `{"source_system":"crm","object_type":"account","source_key":"42","uri":"https://crm.example/accounts/42"}`
	res := request(t, h, http.MethodPost, "/v1/entities/"+e.ID+"/references", strings.NewReader(body))
	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", res.Code, http.StatusCreated, res.Body.String())
	}
	var got identity.Entity
	decodeResponse(t, res, &got)
	if len(got.References) != 1 {
		t.Fatalf("references = %d, want 1", len(got.References))
	}
	ref := got.References[0]
	if ref.SourceSystem != "crm" || ref.ObjectType != "account" || ref.SourceKey != "42" || ref.ObservedAt.IsZero() {
		t.Fatalf("unexpected reference: %+v", ref)
	}
}

func TestTypedReferenceConflict(t *testing.T) {
	h := httpapi.New(identity.NewMemoryStore())
	one := createEntity(t, h, "organization")
	two := createEntity(t, h, "organization")
	body := []byte(`{"source_system":"crm","object_type":"account","source_key":"42"}`)
	first := request(t, h, http.MethodPost, "/v1/entities/"+one.ID+"/references", bytes.NewReader(body))
	if first.Code != http.StatusCreated {
		t.Fatalf("first status = %d, want %d", first.Code, http.StatusCreated)
	}
	conflict := request(t, h, http.MethodPost, "/v1/entities/"+two.ID+"/references", bytes.NewReader(body))
	assertError(t, conflict, http.StatusConflict)
}

func TestContradictoryDomainAssertionsCoexist(t *testing.T) {
	h := httpapi.New(identity.NewMemoryStore())
	e := createEntity(t, h, "organization")
	for _, assertion := range []string{
		`{"domain":"finance","predicate":"customer-status","value":"delinquent","valid_from":"2026-01-01T00:00:00Z","provenance":{"source":"ledger"}}`,
		`{"domain":"sales","predicate":"customer-status","value":"strategic","valid_from":"2026-01-01T00:00:00Z","provenance":{"source":"crm","actor":"sales-ops","method":"review"}}`,
	} {
		res := request(t, h, http.MethodPost, "/v1/entities/"+e.ID+"/assertions", strings.NewReader(assertion))
		if res.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d: %s", res.Code, http.StatusCreated, res.Body.String())
		}
	}

	res := request(t, h, http.MethodGet, "/v1/entities/"+e.ID, nil)
	var got identity.Entity
	decodeResponse(t, res, &got)
	if len(got.Assertions) != 2 {
		t.Fatalf("assertions = %d, want 2", len(got.Assertions))
	}
	if got.Assertions[0].Domain != "finance" || string(got.Assertions[0].Value) != `"delinquent"` || got.Assertions[1].Domain != "sales" || string(got.Assertions[1].Value) != `"strategic"` {
		t.Fatalf("assertions lost domain disagreement: %+v", got.Assertions)
	}
	for _, assertion := range got.Assertions {
		if !strings.HasPrefix(assertion.ID, "ast_") || assertion.RecordedAt.IsZero() || assertion.Provenance.Source == "" {
			t.Fatalf("incomplete assertion: %+v", assertion)
		}
	}
}

func TestRejectsBodyLargerThanOneMiB(t *testing.T) {
	body := `{"kind":"` + strings.Repeat("x", 1<<20) + `"}`
	res := request(t, httpapi.New(identity.NewMemoryStore()), http.MethodPost, "/v1/entities", strings.NewReader(body))
	assertError(t, res, http.StatusBadRequest)
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
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/healthz"},
		{http.MethodPost, "/v1/entities"},
		{http.MethodGet, "/v1/entities/{id}"},
		{http.MethodPost, "/v1/entities/{id}/references"},
		{http.MethodPost, "/v1/entities/{id}/assertions"},
	} {
		operations, ok := contract.Paths[route.path]
		if !ok {
			t.Errorf("contract missing path %s", route.path)
			continue
		}
		if _, ok := operations[strings.ToLower(route.method)]; !ok {
			t.Errorf("contract missing %s %s", route.method, route.path)
		}
	}
}

func createEntity(t *testing.T, h http.Handler, kind string) identity.Entity {
	t.Helper()
	res := request(t, h, http.MethodPost, "/v1/entities", strings.NewReader(`{"kind":"`+kind+`"}`))
	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", res.Code, http.StatusCreated, res.Body.String())
	}
	var got identity.Entity
	decodeResponse(t, res, &got)
	return got
}

func request(t *testing.T, h http.Handler, method, target string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == nil {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, body)
		req.Header.Set("Content-Type", "application/json")
	}
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	return res
}

func decodeResponse(t *testing.T, res *httptest.ResponseRecorder, dst any) {
	t.Helper()
	if got := res.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	if err := json.NewDecoder(res.Body).Decode(dst); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func assertError(t *testing.T, res *httptest.ResponseRecorder, status int) {
	t.Helper()
	if res.Code != status {
		t.Fatalf("status = %d, want %d: %s", res.Code, status, res.Body.String())
	}
	var got errorResponse
	decodeResponse(t, res, &got)
	if got.Error == "" {
		t.Fatal("error response has an empty error message")
	}
}
