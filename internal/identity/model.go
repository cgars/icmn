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
	Assertions []Assertion         `json:"assertions,omitempty"`
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

// Page describes a stable, creation-ordered page. After is the last ID from
// the previous page; using a cursor avoids records moving between pages.
type Page struct {
	Limit int
	After string
}

type EntityPage struct {
	Items      []Entity `json:"items"`
	NextCursor string   `json:"next_cursor,omitempty"`
}

type AuditEvent struct {
	ID         string          `json:"id"`
	EntityID   string          `json:"entity_id"`
	Type       string          `json:"type"`
	OccurredAt time.Time       `json:"occurred_at"`
	Details    json.RawMessage `json:"details"`
}

type OutboxMessage struct {
	ID          string          `json:"id"`
	Topic       string          `json:"topic"`
	Payload     json.RawMessage `json:"payload"`
	OccurredAt  time.Time       `json:"occurred_at"`
	Attempts    int             `json:"attempts"`
	AvailableAt time.Time       `json:"available_at"`
}
