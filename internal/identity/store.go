package identity

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotFound = errors.New("identity not found")
	ErrConflict = errors.New("reference already belongs to another identity")
	ErrInvalid  = errors.New("invalid input")
)

type Store interface {
	Create(context.Context, CreateEntity) (Entity, error)
	Get(context.Context, string) (Entity, error)
	AddReference(context.Context, string, ExternalReference) (Entity, error)
	AddAssertion(context.Context, string, Assertion) (Entity, error)
}

type MemoryStore struct {
	mu         sync.RWMutex
	entities   map[string]Entity
	references map[string]string
	now        func() time.Time
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		entities:   make(map[string]Entity),
		references: make(map[string]string),
		now:        time.Now,
	}
}

func (s *MemoryStore) Create(_ context.Context, in CreateEntity) (Entity, error) {
	if strings.TrimSpace(in.Kind) == "" {
		return Entity{}, fmt.Errorf("%w: kind is required", ErrInvalid)
	}
	e := Entity{ID: newID("ent"), Kind: in.Kind, CreatedAt: s.now().UTC()}
	s.mu.Lock()
	s.entities[e.ID] = e
	s.mu.Unlock()
	return e, nil
}

func (s *MemoryStore) Get(_ context.Context, id string) (Entity, error) {
	s.mu.RLock()
	e, ok := s.entities[id]
	s.mu.RUnlock()
	if !ok {
		return Entity{}, ErrNotFound
	}
	return clone(e), nil
}

func (s *MemoryStore) AddReference(_ context.Context, id string, ref ExternalReference) (Entity, error) {
	if strings.TrimSpace(ref.SourceSystem) == "" || strings.TrimSpace(ref.ObjectType) == "" || strings.TrimSpace(ref.SourceKey) == "" {
		return Entity{}, fmt.Errorf("%w: source_system, object_type, and source_key are required", ErrInvalid)
	}
	key := ref.SourceSystem + "\x00" + ref.ObjectType + "\x00" + ref.SourceKey
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entities[id]
	if !ok {
		return Entity{}, ErrNotFound
	}
	if owner, exists := s.references[key]; exists && owner != id {
		return Entity{}, ErrConflict
	}
	if ref.ObservedAt.IsZero() {
		ref.ObservedAt = s.now().UTC()
	} else {
		ref.ObservedAt = ref.ObservedAt.UTC()
	}
	s.references[key] = id
	e.References = append(e.References, ref)
	s.entities[id] = e
	return clone(e), nil
}

func (s *MemoryStore) AddAssertion(_ context.Context, id string, assertion Assertion) (Entity, error) {
	if strings.TrimSpace(assertion.Domain) == "" || strings.TrimSpace(assertion.Predicate) == "" || len(assertion.Value) == 0 || strings.TrimSpace(assertion.Provenance.Source) == "" {
		return Entity{}, fmt.Errorf("%w: domain, predicate, value, and provenance.source are required", ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entities[id]
	if !ok {
		return Entity{}, ErrNotFound
	}
	now := s.now().UTC()
	assertion.ID = newID("ast")
	assertion.RecordedAt = now
	if assertion.ValidFrom.IsZero() {
		assertion.ValidFrom = now
	} else {
		assertion.ValidFrom = assertion.ValidFrom.UTC()
	}
	if assertion.ValidTo != nil {
		validTo := assertion.ValidTo.UTC()
		assertion.ValidTo = &validTo
	}
	e.Assertions = append(e.Assertions, assertion)
	s.entities[id] = e
	return clone(e), nil
}

func newID(prefix string) string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return prefix + "_" + hex.EncodeToString(b)
}

func clone(e Entity) Entity {
	e.References = append([]ExternalReference(nil), e.References...)
	e.Assertions = append([]Assertion(nil), e.Assertions...)
	return e
}
