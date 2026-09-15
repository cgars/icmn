package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
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
	FindByReference(context.Context, ExternalReference) (Entity, error)
	List(context.Context, Page) (EntityPage, error)
}

type idempotencyContextKey struct{}

// WithIdempotencyKey attaches a caller-scoped command key. Reusing a key with
// the same command returns its original result; different input conflicts.
func WithIdempotencyKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, idempotencyContextKey{}, strings.TrimSpace(key))
}

func IdempotencyKey(ctx context.Context) string {
	key, _ := ctx.Value(idempotencyContextKey{}).(string)
	return key
}

type MemoryStore struct {
	mu         sync.RWMutex
	entities   map[string]Entity
	references map[string]string
	now        func() time.Time
	idempotent map[string]memoryCommand
}

type memoryCommand struct {
	hash   [32]byte
	entity Entity
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		entities:   make(map[string]Entity),
		references: make(map[string]string),
		now:        time.Now,
		idempotent: make(map[string]memoryCommand),
	}
}

func (s *MemoryStore) Create(ctx context.Context, in CreateEntity) (Entity, error) {
	if strings.TrimSpace(in.Kind) == "" {
		return Entity{}, fmt.Errorf("%w: kind is required", ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	hash := commandHash("create_entity", in)
	if replay, found, err := s.replay(ctx, hash); found || err != nil {
		return replay, err
	}
	e := Entity{ID: newID("ent"), Kind: in.Kind, CreatedAt: s.now().UTC()}
	s.entities[e.ID] = e
	s.remember(ctx, hash, e)
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

func (s *MemoryStore) AddReference(ctx context.Context, id string, ref ExternalReference) (Entity, error) {
	if strings.TrimSpace(ref.SourceSystem) == "" || strings.TrimSpace(ref.ObjectType) == "" || strings.TrimSpace(ref.SourceKey) == "" {
		return Entity{}, fmt.Errorf("%w: source_system, object_type, and source_key are required", ErrInvalid)
	}
	key := ref.SourceSystem + "\x00" + ref.ObjectType + "\x00" + ref.SourceKey
	s.mu.Lock()
	defer s.mu.Unlock()
	hash := commandHash("add_reference:"+id, ref)
	if replay, found, err := s.replay(ctx, hash); found || err != nil {
		return replay, err
	}
	e, ok := s.entities[id]
	if !ok {
		return Entity{}, ErrNotFound
	}
	if owner, exists := s.references[key]; exists && owner != id {
		return Entity{}, ErrConflict
	} else if exists {
		return clone(e), nil
	}
	if ref.ObservedAt.IsZero() {
		ref.ObservedAt = s.now().UTC()
	} else {
		ref.ObservedAt = ref.ObservedAt.UTC()
	}
	s.references[key] = id
	e.References = append(e.References, ref)
	s.entities[id] = e
	s.remember(ctx, hash, e)
	return clone(e), nil
}

func (s *MemoryStore) AddAssertion(ctx context.Context, id string, assertion Assertion) (Entity, error) {
	if strings.TrimSpace(assertion.Domain) == "" || strings.TrimSpace(assertion.Predicate) == "" || len(assertion.Value) == 0 || strings.TrimSpace(assertion.Provenance.Source) == "" {
		return Entity{}, fmt.Errorf("%w: domain, predicate, value, and provenance.source are required", ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	hash := commandHash("add_assertion:"+id, assertion)
	if replay, found, err := s.replay(ctx, hash); found || err != nil {
		return replay, err
	}
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
	s.remember(ctx, hash, e)
	return clone(e), nil
}

func commandHash(name string, in any) [32]byte {
	raw, _ := json.Marshal(in)
	return sha256.Sum256(append([]byte(name+"\x00"), raw...))
}
func (s *MemoryStore) replay(ctx context.Context, hash [32]byte) (Entity, bool, error) {
	key := IdempotencyKey(ctx)
	if key == "" {
		return Entity{}, false, nil
	}
	old, ok := s.idempotent[key]
	if !ok {
		return Entity{}, false, nil
	}
	if old.hash != hash {
		return Entity{}, true, fmt.Errorf("%w: idempotency key was used for a different command", ErrConflict)
	}
	return clone(old.entity), true, nil
}
func (s *MemoryStore) remember(ctx context.Context, hash [32]byte, e Entity) {
	if key := IdempotencyKey(ctx); key != "" {
		s.idempotent[key] = memoryCommand{hash: hash, entity: clone(e)}
	}
}

func (s *MemoryStore) FindByReference(_ context.Context, ref ExternalReference) (Entity, error) {
	key := ref.SourceSystem + "\x00" + ref.ObjectType + "\x00" + ref.SourceKey
	s.mu.RLock()
	id, ok := s.references[key]
	e := s.entities[id]
	s.mu.RUnlock()
	if !ok {
		return Entity{}, ErrNotFound
	}
	return clone(e), nil
}

func (s *MemoryStore) List(_ context.Context, page Page) (EntityPage, error) {
	if page.Limit <= 0 {
		page.Limit = 50
	}
	if page.Limit > 200 {
		return EntityPage{}, fmt.Errorf("%w: limit must not exceed 200", ErrInvalid)
	}
	s.mu.RLock()
	items := make([]Entity, 0, len(s.entities))
	for _, e := range s.entities {
		if e.ID > page.After {
			items = append(items, clone(e))
		}
	}
	s.mu.RUnlock()
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	result := EntityPage{Items: items}
	if len(items) > page.Limit {
		result.Items = items[:page.Limit]
		result.NextCursor = result.Items[len(result.Items)-1].ID
	}
	return result, nil
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
