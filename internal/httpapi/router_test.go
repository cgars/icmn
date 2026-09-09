package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cgars/icmn/internal/identity"
)

func TestCreateAndReadEntity(t *testing.T) {
	h := New(identity.NewMemoryStore())
	req := httptest.NewRequest(http.MethodPost, "/v1/entities", strings.NewReader(`{"kind":"organization"}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"id":"ent_`) {
		t.Fatalf("missing generated identity: %s", res.Body.String())
	}
}
