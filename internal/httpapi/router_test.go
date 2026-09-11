package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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

func TestEmptyEntityListIsJSONArray(t *testing.T) {
	res := request(t, httpapi.New(identity.NewMemoryStore()), http.MethodGet, "/v1/entities", nil)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"items":[]`) {
		t.Fatalf("status=%d body=%s, want items array", res.Code, res.Body.String())
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

func TestJSONBodyIsCompleteAndWithinLimitAcrossPOSTEndpoints(t *testing.T) {
	endpoints := []struct {
		name string
		path func(identity.Entity) string
		body string
	}{
		{name: "create entity", path: func(identity.Entity) string { return "/v1/entities" }, body: `{"kind":"organization"}`},
		{name: "add reference", path: func(e identity.Entity) string { return "/v1/entities/" + e.ID + "/references" }, body: `{"source_system":"crm","object_type":"account","source_key":"42"}`},
		{name: "add assertion", path: func(e identity.Entity) string { return "/v1/entities/" + e.ID + "/assertions" }, body: `{"domain":"finance","predicate":"status","value":"active","provenance":{"source":"ledger"}}`},
	}
	rejectedSuffixes := []struct {
		name   string
		suffix func(string) string
	}{
		{name: "trailing garbage", suffix: func(string) string { return `garbage` }},
		{name: "second JSON value", suffix: func(string) string { return `{}` }},
		{name: "trailing whitespace over limit", suffix: func(body string) string {
			return strings.Repeat(" ", (1<<20)-len(body)+1)
		}},
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint.name, func(t *testing.T) {
			for _, rejected := range rejectedSuffixes {
				t.Run(rejected.name, func(t *testing.T) {
					store, entity := preparedCountingStore(t)
					res := request(t, httpapi.New(store), http.MethodPost, endpoint.path(entity), strings.NewReader(endpoint.body+rejected.suffix(endpoint.body)))
					assertError(t, res, http.StatusBadRequest)
					if got := store.mutations(); got != 0 {
						t.Fatalf("store mutations = %d, want 0", got)
					}
				})
			}

			t.Run("valid trailing whitespace", func(t *testing.T) {
				store, entity := preparedCountingStore(t)
				res := request(t, httpapi.New(store), http.MethodPost, endpoint.path(entity), strings.NewReader(endpoint.body+" \n\t"))
				if res.Code != http.StatusCreated {
					t.Fatalf("status = %d, want %d: %s", res.Code, http.StatusCreated, res.Body.String())
				}
				if got := store.mutations(); got != 1 {
					t.Fatalf("store mutations = %d, want 1", got)
				}
			})
		})
	}
}

func TestSuppliedTimestampsAreSerializedInUTC(t *testing.T) {
	h := httpapi.New(identity.NewMemoryStore())
	e := createEntity(t, h, "organization")

	refBody := `{"source_system":"crm","object_type":"account","source_key":"42","observed_at":"2026-09-09T12:00:00+02:00"}`
	refResponse := request(t, h, http.MethodPost, "/v1/entities/"+e.ID+"/references", strings.NewReader(refBody))
	assertEntityTimestampStrings(t, refResponse, "2026-09-09T10:00:00Z", "", "", false)

	assertionBody := `{"domain":"finance","predicate":"status","value":"active","valid_from":"2026-09-09T12:00:00+02:00","valid_to":"2026-09-09T13:00:00+02:00","provenance":{"source":"ledger"}}`
	assertionResponse := request(t, h, http.MethodPost, "/v1/entities/"+e.ID+"/assertions", strings.NewReader(assertionBody))
	assertEntityTimestampStrings(t, assertionResponse, "2026-09-09T10:00:00Z", "2026-09-09T10:00:00Z", "2026-09-09T11:00:00Z", true)

	withoutValidTo := `{"domain":"sales","predicate":"status","value":"active","valid_from":"2026-09-09T12:30:00+02:00","provenance":{"source":"crm"}}`
	optionalResponse := request(t, h, http.MethodPost, "/v1/entities/"+e.ID+"/assertions", strings.NewReader(withoutValidTo))
	assertLastAssertionOmitsValidTo(t, optionalResponse, "2026-09-09T10:30:00Z")

	retrieved := request(t, h, http.MethodGet, "/v1/entities/"+e.ID, nil)
	body := retrieved.Body.Bytes()
	assertEntityTimestampStrings(t, responseWithBody(retrieved, body), "2026-09-09T10:00:00Z", "2026-09-09T10:00:00Z", "2026-09-09T11:00:00Z", true)
	assertLastAssertionOmitsValidTo(t, responseWithBody(retrieved, body), "2026-09-09T10:30:00Z")
}

type countingStore struct {
	identity.Store
	creates, references, assertions int
}

func (s *countingStore) Create(ctx context.Context, in identity.CreateEntity) (identity.Entity, error) {
	s.creates++
	return s.Store.Create(ctx, in)
}

func (s *countingStore) AddReference(ctx context.Context, id string, in identity.ExternalReference) (identity.Entity, error) {
	s.references++
	return s.Store.AddReference(ctx, id, in)
}

func (s *countingStore) AddAssertion(ctx context.Context, id string, in identity.Assertion) (identity.Entity, error) {
	s.assertions++
	return s.Store.AddAssertion(ctx, id, in)
}

func (s *countingStore) mutations() int { return s.creates + s.references + s.assertions }

func preparedCountingStore(t *testing.T) (*countingStore, identity.Entity) {
	t.Helper()
	base := identity.NewMemoryStore()
	entity, err := base.Create(context.Background(), identity.CreateEntity{Kind: "organization"})
	if err != nil {
		t.Fatal(err)
	}
	return &countingStore{Store: base}, entity
}

func assertEntityTimestampStrings(t *testing.T, res *httptest.ResponseRecorder, observedAt, validFrom, validTo string, wantAssertion bool) {
	t.Helper()
	if res.Code < 200 || res.Code >= 300 {
		t.Fatalf("status = %d, want success: %s", res.Code, res.Body.String())
	}
	var got struct {
		References []struct {
			ObservedAt string `json:"observed_at"`
		} `json:"references"`
		Assertions []struct {
			ValidFrom string  `json:"valid_from"`
			ValidTo   *string `json:"valid_to"`
		} `json:"assertions"`
	}
	decodeResponse(t, res, &got)
	if len(got.References) != 1 || got.References[0].ObservedAt != observedAt {
		t.Fatalf("observed_at = %v, want %q", got.References, observedAt)
	}
	if !wantAssertion {
		if len(got.Assertions) != 0 {
			t.Fatalf("assertions = %v, want none", got.Assertions)
		}
		return
	}
	if len(got.Assertions) < 1 || got.Assertions[0].ValidFrom != validFrom || got.Assertions[0].ValidTo == nil || *got.Assertions[0].ValidTo != validTo {
		t.Fatalf("assertion timestamps = %v, want valid_from %q and valid_to %q", got.Assertions, validFrom, validTo)
	}
}

func assertLastAssertionOmitsValidTo(t *testing.T, res *httptest.ResponseRecorder, validFrom string) {
	t.Helper()
	if res.Code < 200 || res.Code >= 300 {
		t.Fatalf("status = %d, want success: %s", res.Code, res.Body.String())
	}
	var got struct {
		Assertions []struct {
			ValidFrom string  `json:"valid_from"`
			ValidTo   *string `json:"valid_to"`
		} `json:"assertions"`
	}
	decodeResponse(t, res, &got)
	if len(got.Assertions) == 0 {
		t.Fatal("response has no assertions")
	}
	last := got.Assertions[len(got.Assertions)-1]
	if last.ValidFrom != validFrom || last.ValidTo != nil {
		t.Fatalf("last assertion = %+v, want valid_from %q and omitted valid_to", last, validFrom)
	}
}

func responseWithBody(original *httptest.ResponseRecorder, body []byte) *httptest.ResponseRecorder {
	copy := httptest.NewRecorder()
	copy.Code = original.Code
	copy.HeaderMap = original.Header().Clone()
	copy.Body.Write(body)
	return copy
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

func TestListLookupAndIdempotencyContract(t *testing.T) {
	h := httpapi.New(identity.NewMemoryStore())
	body := `{"kind":"organization"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/entities", strings.NewReader(body))
	req.Header.Set("Idempotency-Key", "create-fictional")
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	var created identity.Entity
	decodeResponse(t, res, &created)
	req = httptest.NewRequest(http.MethodPost, "/v1/entities", strings.NewReader(body))
	req.Header.Set("Idempotency-Key", "create-fictional")
	res = httptest.NewRecorder()
	h.ServeHTTP(res, req)
	var replay identity.Entity
	decodeResponse(t, res, &replay)
	if replay.ID != created.ID {
		t.Fatalf("replay id=%s want %s", replay.ID, created.ID)
	}
	ref := `{"source_system":"crm","object_type":"account","source_key":"fictional-42"}`
	res = request(t, h, http.MethodPost, "/v1/entities/"+created.ID+"/references", strings.NewReader(ref))
	if res.Code != http.StatusCreated {
		t.Fatal(res.Body.String())
	}
	res = request(t, h, http.MethodGet, "/v1/entities/by-reference?source_system=crm&object_type=account&source_key=fictional-42", nil)
	var found identity.Entity
	decodeResponse(t, res, &found)
	if found.ID != created.ID {
		t.Fatalf("found=%+v", found)
	}
	res = request(t, h, http.MethodGet, "/v1/entities?limit=1", nil)
	var page identity.EntityPage
	decodeResponse(t, res, &page)
	if len(page.Items) != 1 {
		t.Fatalf("page=%+v", page)
	}
}
