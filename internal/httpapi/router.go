package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/cgars/icmn/internal/identity"
)

type API struct{ store identity.Store }

func New(store identity.Store) http.Handler {
	a := &API{store: store}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.health)
	mux.HandleFunc("POST /v1/entities", a.createEntity)
	mux.HandleFunc("GET /v1/entities/{id}", a.getEntity)
	mux.HandleFunc("POST /v1/entities/{id}/references", a.addReference)
	mux.HandleFunc("POST /v1/entities/{id}/assertions", a.addAssertion)
	return mux
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) createEntity(w http.ResponseWriter, r *http.Request) {
	var in identity.CreateEntity
	if !decode(w, r, &in) {
		return
	}
	e, err := a.store.Create(r.Context(), in)
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
	e, err := a.store.AddReference(r.Context(), r.PathValue("id"), in)
	respond(w, e, err, http.StatusCreated)
}

func (a *API) addAssertion(w http.ResponseWriter, r *http.Request) {
	var in identity.Assertion
	if !decode(w, r, &in) {
		return
	}
	e, err := a.store.AddAssertion(r.Context(), r.PathValue("id"), in)
	respond(w, e, err, http.StatusCreated)
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
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
