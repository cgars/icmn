package identity

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestReferencesAreUniqueAcrossIdentities(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	one, _ := s.Create(ctx, CreateEntity{Kind: "organization"})
	two, _ := s.Create(ctx, CreateEntity{Kind: "organization"})
	ref := ExternalReference{SourceSystem: "crm", ObjectType: "account", SourceKey: "42"}

	if _, err := s.AddReference(ctx, one.ID, ref); err != nil {
		t.Fatalf("first reference: %v", err)
	}
	if _, err := s.AddReference(ctx, two.ID, ref); !errors.Is(err, ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
}

func TestConflictingAssertionsCanCoexist(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	e, _ := s.Create(ctx, CreateEntity{Kind: "organization"})
	for _, domain := range []string{"finance", "sales"} {
		_, err := s.AddAssertion(ctx, e.ID, Assertion{
			Domain: domain, Predicate: "customer-status", Value: json.RawMessage(`"active"`),
			Provenance: Provenance{Source: domain + "-system"},
		})
		if err != nil {
			t.Fatalf("add assertion for %s: %v", domain, err)
		}
	}
	got, _ := s.Get(ctx, e.ID)
	if len(got.Assertions) != 2 {
		t.Fatalf("want 2 assertions, got %d", len(got.Assertions))
	}
}
