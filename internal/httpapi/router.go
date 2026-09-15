package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/cgars/icmn/internal/identity"
)

type API struct{ store identity.Store }

func New(store identity.Store) http.Handler {
	a := &API{store: store}
	mux := http.NewServeMux()
	registerRoutes(mux, a)
	return mux
}

type routeRegistrar interface {
	HandleFunc(string, func(http.ResponseWriter, *http.Request))
}

func registerRoutes(mux routeRegistrar, a *API) {
	mux.HandleFunc("GET /healthz", a.health)
	mux.HandleFunc("POST /v1/entities", a.createEntity)
	mux.HandleFunc("GET /v1/entities", a.listEntities)
	mux.HandleFunc("GET /v1/entities/by-reference", a.findByReference)
	mux.HandleFunc("GET /v1/entities/{id}", a.getEntity)
	mux.HandleFunc("POST /v1/entities/{id}/references", a.addReference)
	mux.HandleFunc("POST /v1/entities/{id}/assertions", a.addAssertion)
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) createEntity(w http.ResponseWriter, r *http.Request) {
	var in identity.CreateEntity
	if !decode(w, r, &in) {
		return
	}
	e, err := a.store.Create(commandContext(r), in)
	respond(w, e, err, http.StatusCreated)
}

func (a *API) getEntity(w http.ResponseWriter, r *http.Request) {
	e, err := a.store.Get(r.Context(), r.PathValue("id"))
	respond(w, e, err, http.StatusOK)
}

func (a *API) addReference(w http.ResponseWriter, r *http.Request) {
	var in identity.ExternalReference
	if !decode(w, r, &in) {
		return
	}
	e, err := a.store.AddReference(commandContext(r), r.PathValue("id"), in)
	respond(w, e, err, http.StatusCreated)
}

func (a *API) addAssertion(w http.ResponseWriter, r *http.Request) {
	var in identity.Assertion
	if !decode(w, r, &in) {
		return
	}
	e, err := a.store.AddAssertion(commandContext(r), r.PathValue("id"), in)
	respond(w, e, err, http.StatusCreated)
}

func commandContext(r *http.Request) context.Context {
	return identity.WithIdempotencyKey(r.Context(), r.Header.Get("Idempotency-Key"))
}

func (a *API) listEntities(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			respond(w, nil, fmt.Errorf("%w: limit must be an integer", identity.ErrInvalid), 0)
			return
		}
		limit = n
	}
	page, err := a.store.List(r.Context(), identity.Page{Limit: limit, After: r.URL.Query().Get("after")})
	respond(w, page, err, http.StatusOK)
}
func (a *API) findByReference(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ref := identity.ExternalReference{SourceSystem: q.Get("source_system"), ObjectType: q.Get("object_type"), SourceKey: q.Get("source_key")}
	if ref.SourceSystem == "" || ref.ObjectType == "" || ref.SourceKey == "" {
		respond(w, nil, fmt.Errorf("%w: source_system, object_type, and source_key are required", identity.ErrInvalid), 0)
		return
	}
	e, err := a.store.FindByReference(r.Context(), ref)
	respond(w, e, err, http.StatusOK)
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return false
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("request body must contain exactly one JSON value")
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return false
	}
	return true
}

func respond(w http.ResponseWriter, value any, err error, success int) {
	if err == nil {
		writeJSON(w, success, value)
		return
	}
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, identity.ErrInvalid):
		status = http.StatusBadRequest
	case errors.Is(err, identity.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, identity.ErrConflict):
		status = http.StatusConflict
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
