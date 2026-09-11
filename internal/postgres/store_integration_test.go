package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/cgars/icmn/internal/identity"
	"os"
	"sync"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("ICMN_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("ICMN_TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	s, err := Open(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"idempotency", "outbox", "audit_events", "assertions", "external_references", "entities"} {
		if _, err := s.db.Exec("DELETE FROM " + table); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func TestDurabilityAndIndependentAssertions(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	e, err := s.Create(ctx, identity.CreateEntity{Kind: "organization"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.AddAssertion(ctx, e.ID, identity.Assertion{Domain: "finance", Predicate: "status", Value: json.RawMessage(`{"state":"due"}`), Provenance: identity.Provenance{Source: "fictional-ledger"}})
	if err != nil {
		t.Fatal(err)
	}
	url := os.Getenv("ICMN_TEST_DATABASE_URL")
	s.Close()
	s, err = Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, e.ID)
	if err != nil || len(got.Assertions) != 1 || string(got.Assertions[0].Value) != `{"state": "due"}` {
		t.Fatalf("after reopen: %+v %v", got, err)
	}
}
func TestConcurrentReferenceUniqueness(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	a, _ := s.Create(ctx, identity.CreateEntity{Kind: "organization"})
	b, _ := s.Create(ctx, identity.CreateEntity{Kind: "organization"})
	ref := identity.ExternalReference{SourceSystem: "crm", ObjectType: "account", SourceKey: "42"}
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, id := range []string{a.ID, b.ID} {
		wg.Add(1)
		go func(id string) { defer wg.Done(); <-start; _, err := s.AddReference(ctx, id, ref); errs <- err }(id)
	}
	close(start)
	wg.Wait()
	close(errs)
	success, conflicts := 0, 0
	for err := range errs {
		if err == nil {
			success++
		} else if errors.Is(err, identity.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatalf("success=%d conflicts=%d", success, conflicts)
	}
}
func TestIdempotencyAuditOutboxAndPagination(t *testing.T) {
	ctx := identity.WithIdempotencyKey(context.Background(), "same-command")
	s := testStore(t)
	first, err := s.Create(ctx, identity.CreateEntity{Kind: "organization"})
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.Create(ctx, identity.CreateEntity{Kind: "organization"})
	if err != nil || again.ID != first.ID {
		t.Fatalf("replay=%+v %v", again, err)
	}
	_, err = s.Create(ctx, identity.CreateEntity{Kind: "person"})
	if !errors.Is(err, identity.ErrConflict) {
		t.Fatalf("want conflict, got %v", err)
	}
	s.Create(context.Background(), identity.CreateEntity{Kind: "organization"})
	page, err := s.List(context.Background(), identity.Page{Limit: 1})
	if err != nil || len(page.Items) != 1 || page.NextCursor == "" {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	next, _ := s.List(context.Background(), identity.Page{Limit: 10, After: page.NextCursor})
	if len(next.Items) != 1 || next.Items[0].ID <= page.Items[0].ID {
		t.Fatalf("next=%+v", next)
	}
	var audits, outbox int
	s.db.QueryRow(`SELECT count(*) FROM audit_events`).Scan(&audits)
	s.db.QueryRow(`SELECT count(*) FROM outbox`).Scan(&outbox)
	if audits != 2 || outbox != 2 {
		t.Fatalf("audit=%d outbox=%d", audits, outbox)
	}
	messages, err := s.ClaimOutbox(context.Background(), 10, time.Now().Add(time.Minute))
	if err != nil || len(messages) != 2 {
		t.Fatalf("messages=%d err=%v", len(messages), err)
	}
	if err = s.AcknowledgeOutbox(context.Background(), messages[0].ID); err != nil {
		t.Fatal(err)
	}
}

func TestIdempotentReplayForReferenceAndAssertion(t *testing.T) {
	s := testStore(t)
	baseCtx := context.Background()
	e, err := s.Create(baseCtx, identity.CreateEntity{Kind: "organization"})
	if err != nil {
		t.Fatal(err)
	}

	refCtx := identity.WithIdempotencyKey(baseCtx, "same-reference-command")
	ref := identity.ExternalReference{SourceSystem: "crm", ObjectType: "account", SourceKey: "42"}
	firstReference, err := s.AddReference(refCtx, e.ID, ref)
	if err != nil {
		t.Fatal(err)
	}
	replayedReference, err := s.AddReference(refCtx, e.ID, ref)
	if err != nil {
		t.Fatal(err)
	}
	if replayedReference.ID != firstReference.ID || len(replayedReference.References) != len(firstReference.References) {
		t.Fatalf("reference replay mismatch: first=%+v replay=%+v", firstReference, replayedReference)
	}

	assertionCtx := identity.WithIdempotencyKey(baseCtx, "same-assertion-command")
	assertion := identity.Assertion{
		Domain:     "finance",
		Predicate:  "status",
		Value:      json.RawMessage(`{"state":"due"}`),
		Provenance: identity.Provenance{Source: "fictional-ledger"},
	}
	firstAssertion, err := s.AddAssertion(assertionCtx, e.ID, assertion)
	if err != nil {
		t.Fatal(err)
	}
	replayedAssertion, err := s.AddAssertion(assertionCtx, e.ID, assertion)
	if err != nil {
		t.Fatal(err)
	}
	if replayedAssertion.ID != firstAssertion.ID || len(replayedAssertion.Assertions) != len(firstAssertion.Assertions) {
		t.Fatalf("assertion replay mismatch: first=%+v replay=%+v", firstAssertion, replayedAssertion)
	}
}

func TestFailedMutationRollsBackBusinessAuditAndOutbox(t *testing.T) {
	s := testStore(t)
	ctx := identity.WithIdempotencyKey(context.Background(), "missing-entity-command")
	_, err := s.AddAssertion(ctx, "ent_00000000000000000000000000000000", identity.Assertion{Domain: "finance", Predicate: "status", Value: json.RawMessage(`"due"`), Provenance: identity.Provenance{Source: "fictional-ledger"}})
	if !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("want not found, got %v", err)
	}
	for _, table := range []string{"assertions", "audit_events", "outbox", "idempotency"} {
		var count int
		if err := s.db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("%s rows=%d, want rollback", table, count)
		}
	}
}

func TestConcurrentIdempotentReplayReturnsOneIdentity(t *testing.T) {
	s := testStore(t)
	ctx := identity.WithIdempotencyKey(context.Background(), "concurrent-create")
	start := make(chan struct{})
	results := make(chan identity.Entity, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			e, err := s.Create(ctx, identity.CreateEntity{Kind: "organization"})
			results <- e
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var id string
	for e := range results {
		if id == "" {
			id = e.ID
		} else if e.ID != id {
			t.Fatalf("ids differ: %s and %s", id, e.ID)
		}
	}
	var count int
	s.db.QueryRow(`SELECT count(*) FROM entities`).Scan(&count)
	if count != 1 {
		t.Fatalf("entities=%d, want 1", count)
	}
}
