package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cgars/icmn/internal/identity"
	"net/url"
	"os"
	"reflect"
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

func TestIdempotentGeneratedFieldsAndConcurrentRetries(t *testing.T) {
	s := testStore(t)
	base := context.Background()
	entity, err := s.Create(base, identity.CreateEntity{Kind: "organization"})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		key       string
		call      func(context.Context) (identity.Entity, error)
		different func(context.Context) error
		table     string
	}{
		{
			name: "create", key: "generated-create", table: "entities",
			call: func(ctx context.Context) (identity.Entity, error) {
				return s.Create(ctx, identity.CreateEntity{Kind: "person"})
			},
			different: func(ctx context.Context) error {
				_, err := s.Create(ctx, identity.CreateEntity{Kind: "device"})
				return err
			},
		},
		{
			name: "reference with omitted observed time", key: "generated-reference", table: "external_references",
			call: func(ctx context.Context) (identity.Entity, error) {
				return s.AddReference(ctx, entity.ID, identity.ExternalReference{SourceSystem: "crm", ObjectType: "account", SourceKey: "42"})
			},
			different: func(ctx context.Context) error {
				_, err := s.AddReference(ctx, entity.ID, identity.ExternalReference{SourceSystem: "crm", ObjectType: "account", SourceKey: "43"})
				return err
			},
		},
		{
			name: "assertion with omitted times", key: "generated-assertion", table: "assertions",
			call: func(ctx context.Context) (identity.Entity, error) {
				return s.AddAssertion(ctx, entity.ID, identity.Assertion{Domain: "finance", Predicate: "status", Value: json.RawMessage(`"due"`), Provenance: identity.Provenance{Source: "ledger"}})
			},
			different: func(ctx context.Context) error {
				_, err := s.AddAssertion(ctx, entity.ID, identity.Assertion{Domain: "finance", Predicate: "status", Value: json.RawMessage(`"paid"`), Provenance: identity.Provenance{Source: "ledger"}})
				return err
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := identity.WithIdempotencyKey(base, tt.key)
			before := tableCounts(t, s)
			start := make(chan struct{})
			results := make(chan identity.Entity, 2)
			errs := make(chan error, 2)
			for range 2 {
				go func() { <-start; got, err := tt.call(ctx); results <- got; errs <- err }()
			}
			close(start)
			first, second := <-results, <-results
			for range 2 {
				if err := <-errs; err != nil {
					t.Fatal(err)
				}
			}
			if !reflect.DeepEqual(first, second) {
				t.Fatalf("replays differ:\n%+v\n%+v", first, second)
			}
			after := tableCounts(t, s)
			if after[tt.table] != before[tt.table]+1 || after["audit_events"] != before["audit_events"]+1 || after["outbox"] != before["outbox"]+1 || after["idempotency"] != before["idempotency"]+1 {
				t.Fatalf("before=%v after=%v", before, after)
			}
			if err := tt.different(ctx); !errors.Is(err, identity.ErrConflict) {
				t.Fatalf("different input: %v", err)
			}
			if got := tableCounts(t, s); !reflect.DeepEqual(got, after) {
				t.Fatalf("conflict changed counts: before=%v after=%v", after, got)
			}
		})
	}
}

func TestReferenceReattachAndEmptyPagesMatchMemoryBehavior(t *testing.T) {
	s := testStore(t)
	empty, err := s.List(context.Background(), identity.Page{})
	if err != nil || empty.Items == nil || len(empty.Items) != 0 {
		t.Fatalf("empty=%+v err=%v", empty, err)
	}
	e, _ := s.Create(context.Background(), identity.CreateEntity{Kind: "organization"})
	ref := identity.ExternalReference{SourceSystem: "crm", ObjectType: "account", SourceKey: "same"}
	first, err := s.AddReference(context.Background(), e.ID, ref)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.AddReference(context.Background(), e.ID, ref)
	if err != nil || len(again.References) != 1 || !again.References[0].ObservedAt.Equal(first.References[0].ObservedAt) {
		t.Fatalf("again=%+v err=%v", again, err)
	}
	var audits, outbox int
	s.db.QueryRow(`SELECT count(*) FROM audit_events WHERE event_type='reference.added'`).Scan(&audits)
	s.db.QueryRow(`SELECT count(*) FROM outbox WHERE topic='reference.added'`).Scan(&outbox)
	if audits != 1 || outbox != 1 {
		t.Fatalf("audits=%d outbox=%d", audits, outbox)
	}
	exhausted, _ := s.List(context.Background(), identity.Page{After: "zzzz"})
	if exhausted.Items == nil {
		t.Fatal("exhausted page items is nil")
	}
}

func tableCounts(t *testing.T, s *Store) map[string]int {
	t.Helper()
	counts := map[string]int{}
	for _, table := range []string{"entities", "external_references", "assertions", "audit_events", "outbox", "idempotency"} {
		var count int
		if err := s.db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		counts[table] = count
	}
	return counts
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

func TestOutboxLeaseCompetitionAndExpiredRecovery(t *testing.T) {
	s := testStore(t)
	created, err := s.Create(context.Background(), identity.CreateEntity{Kind: "organization"})
	if err != nil {
		t.Fatal(err)
	}
	_ = created
	lease := time.Now().Add(time.Minute)
	start := make(chan struct{})
	results := make(chan []identity.OutboxMessage, 2)
	for range 2 {
		go func() {
			<-start
			messages, err := s.ClaimOutbox(context.Background(), 1, lease)
			if err != nil {
				t.Error(err)
			}
			results <- messages
		}()
	}
	close(start)
	a, b := <-results, <-results
	if len(a)+len(b) != 1 {
		t.Fatalf("workers claimed %d messages, want 1", len(a)+len(b))
	}
	claimed := a
	if len(claimed) == 0 {
		claimed = b
	}
	if claimed[0].Attempts != 1 {
		t.Fatalf("attempts=%d", claimed[0].Attempts)
	}
	active, err := s.ClaimOutbox(context.Background(), 1, lease)
	if err != nil || len(active) != 0 {
		t.Fatalf("active lease claimed=%v err=%v", active, err)
	}
	if _, err := s.db.Exec(`UPDATE outbox SET available_at=now()-interval '1 second' WHERE id=$1`, claimed[0].ID); err != nil {
		t.Fatal(err)
	}
	recovered, err := s.ClaimOutbox(context.Background(), 1, time.Now().Add(time.Minute))
	if err != nil || len(recovered) != 1 || recovered[0].ID != claimed[0].ID || recovered[0].Attempts != 2 {
		t.Fatalf("recovered=%+v err=%v", recovered, err)
	}
}

func TestLateWriteFailuresRollbackWholeCommand(t *testing.T) {
	for _, failingTable := range []string{"audit_events", "outbox"} {
		t.Run(failingTable, func(t *testing.T) {
			s := testStore(t)
			function := "fail_" + failingTable
			if _, err := s.db.Exec(fmt.Sprintf(`CREATE OR REPLACE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected late write failure'; END $$`, function)); err != nil {
				t.Fatal(err)
			}
			if _, err := s.db.Exec(fmt.Sprintf(`CREATE TRIGGER %s BEFORE INSERT ON %s FOR EACH ROW EXECUTE FUNCTION %s()`, function, failingTable, function)); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _, _ = s.db.Exec(fmt.Sprintf(`DROP FUNCTION IF EXISTS %s() CASCADE`, function)) })
			ctx := identity.WithIdempotencyKey(context.Background(), "rollback-"+failingTable)
			if _, err := s.Create(ctx, identity.CreateEntity{Kind: "organization"}); err == nil {
				t.Fatal("want injected failure")
			}
			counts := tableCounts(t, s)
			for table, count := range counts {
				if count != 0 {
					t.Fatalf("%s=%d, want rollback after %s failure", table, count, failingTable)
				}
			}
		})
	}
}

func TestConcurrentMigrationBootstrapAndRepeat(t *testing.T) {
	base := testStore(t)
	schema := "migration_" + fmt.Sprintf("%d", time.Now().UnixNano())
	if _, err := base.db.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = base.db.Exec(`DROP SCHEMA IF EXISTS ` + schema + ` CASCADE`) })
	u, err := url.Parse(os.Getenv("ICMN_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	db, err := sql.Open("pgx", u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	start := make(chan struct{})
	errs := make(chan error, 2)
	for range 2 {
		go func() { <-start; errs <- Migrate(context.Background(), db) }()
	}
	close(start)
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}
	var versions int
	if err := db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&versions); err != nil || versions != 1 {
		t.Fatalf("versions=%d err=%v", versions, err)
	}
}
