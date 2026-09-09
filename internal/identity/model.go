package identity

import (
	"encoding/json"
	"time"
)

// Entity is the conserved identity. Mutable domain attributes belong in Assertions.
type Entity struct {
	ID         string              `json:"id"`
	Kind       string              `json:"kind"`
	CreatedAt  time.Time           `json:"created_at"`
	References []ExternalReference `json:"references,omitempty"`
	Assertions  []Assertion         `json:"assertions,omitempty"`
}

// ExternalReference points to a representation managed outside ICMN.
type ExternalReference struct {
	SourceSystem string    `json:"source_system"`
	ObjectType   string    `json:"object_type"`
	SourceKey    string    `json:"source_key"`
	URI          string    `json:"uri,omitempty"`
	ObservedAt   time.Time `json:"observed_at"`
}

// Assertion records one domain's contextual statement about an identity.
type Assertion struct {
	ID         string          `json:"id"`
	Domain     string          `json:"domain"`
	Predicate  string          `json:"predicate"`
	Value      json.RawMessage `json:"value"`
	ValidFrom  time.Time       `json:"valid_from"`
	ValidTo    *time.Time      `json:"valid_to,omitempty"`
	Provenance Provenance      `json:"provenance"`
	RecordedAt time.Time       `json:"recorded_at"`
}

type Provenance struct {
	Source string `json:"source"`
	Actor  string `json:"actor,omitempty"`
	Method string `json:"method,omitempty"`
}

type CreateEntity struct {
	Kind string `json:"kind"`
}
