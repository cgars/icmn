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

func TestIdempotentReplayAndConflict(t *testing.T) {
	s := NewMemoryStore()
	ctx := WithIdempotencyKey(context.Background(), "create-1")
	first, err := s.Create(ctx, CreateEntity{Kind: "organization"})
	if err != nil {
		t.Fatal(err)
	}
	replay, err := s.Create(ctx, CreateEntity{Kind: "organization"})
	if err != nil || replay.ID != first.ID {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	if _, err = s.Create(ctx, CreateEntity{Kind: "person"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("want conflict, got %v", err)
	}
}
func TestListIsStableAndReferenceLookupIsTyped(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	a, _ := s.Create(ctx, CreateEntity{Kind: "organization"})
	b, _ := s.Create(ctx, CreateEntity{Kind: "organization"})
	ref := ExternalReference{SourceSystem: "crm", ObjectType: "account", SourceKey: "42"}
	s.AddReference(ctx, a.ID, ref)
	got, err := s.FindByReference(ctx, ref)
	if err != nil || got.ID != a.ID {
		t.Fatalf("lookup=%+v err=%v", got, err)
	}
	first, _ := s.List(ctx, Page{Limit: 1})
	if len(first.Items) != 1 || first.NextCursor == "" {
		t.Fatalf("first=%+v", first)
	}
	second, _ := s.List(ctx, Page{Limit: 1, After: first.NextCursor})
	if len(second.Items) != 1 || second.Items[0].ID == first.Items[0].ID {
		t.Fatalf("second=%+v", second)
	}
	_ = b
}
