package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cgars/icmn/internal/identity"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type Store struct {
	db  *sql.DB
	now func() time.Time
}

func Open(ctx context.Context, url string) (*Store, error) {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return nil, err
	}
	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if err = Migrate(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db, now: time.Now}, nil
}
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Create(ctx context.Context, in identity.CreateEntity) (identity.Entity, error) {
	if strings.TrimSpace(in.Kind) == "" {
		return identity.Entity{}, fmt.Errorf("%w: kind is required", identity.ErrInvalid)
	}
	e := identity.Entity{ID: newID("ent"), Kind: in.Kind, CreatedAt: s.now().UTC()}
	return command(s, ctx, "create_entity", in, func(tx *sql.Tx) (identity.Entity, error) {
		if _, err := tx.ExecContext(ctx, `INSERT INTO entities(id,kind,created_at) VALUES($1,$2,$3)`, e.ID, e.Kind, e.CreatedAt); err != nil {
			return e, err
		}
		return e, record(tx, ctx, e, "identity.created", map[string]any{"kind": e.Kind}, s.now().UTC())
	})
}
func (s *Store) Get(ctx context.Context, id string) (identity.Entity, error) {
	return get(ctx, s.db, id)
}
func (s *Store) AddReference(ctx context.Context, id string, ref identity.ExternalReference) (identity.Entity, error) {
	if strings.TrimSpace(ref.SourceSystem) == "" || strings.TrimSpace(ref.ObjectType) == "" || strings.TrimSpace(ref.SourceKey) == "" {
		return identity.Entity{}, fmt.Errorf("%w: source_system, object_type, and source_key are required", identity.ErrInvalid)
	}
	input := ref
	if ref.ObservedAt.IsZero() {
		ref.ObservedAt = s.now().UTC()
	} else {
		ref.ObservedAt = ref.ObservedAt.UTC()
	}
	return command(s, ctx, "add_reference:"+id, input, func(tx *sql.Tx) (identity.Entity, error) {
		if _, err := tx.ExecContext(ctx, `INSERT INTO external_references(entity_id,source_system,object_type,source_key,uri,observed_at) VALUES($1,$2,$3,$4,$5,$6)`, id, ref.SourceSystem, ref.ObjectType, ref.SourceKey, ref.URI, ref.ObservedAt); err != nil {
			return identity.Entity{}, mapError(err)
		}
		e, err := get(ctx, tx, id)
		if err != nil {
			return e, err
		}
		return e, record(tx, ctx, e, "reference.added", map[string]any{"source_system": ref.SourceSystem, "object_type": ref.ObjectType, "source_key": ref.SourceKey}, s.now().UTC())
	})
}
func (s *Store) AddAssertion(ctx context.Context, id string, a identity.Assertion) (identity.Entity, error) {
	if strings.TrimSpace(a.Domain) == "" || strings.TrimSpace(a.Predicate) == "" || len(a.Value) == 0 || strings.TrimSpace(a.Provenance.Source) == "" {
		return identity.Entity{}, fmt.Errorf("%w: domain, predicate, value, and provenance.source are required", identity.ErrInvalid)
	}
	input := a
	now := s.now().UTC()
	a.ID = newID("ast")
	a.RecordedAt = now
	if a.ValidFrom.IsZero() {
		a.ValidFrom = now
	} else {
		a.ValidFrom = a.ValidFrom.UTC()
	}
	if a.ValidTo != nil {
		v := a.ValidTo.UTC()
		a.ValidTo = &v
	}
	p, _ := json.Marshal(a.Provenance)
	return command(s, ctx, "add_assertion:"+id, input, func(tx *sql.Tx) (identity.Entity, error) {
		if _, err := tx.ExecContext(ctx, `INSERT INTO assertions(id,entity_id,domain,predicate,value,valid_from,valid_to,provenance,recorded_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, a.ID, id, a.Domain, a.Predicate, []byte(a.Value), a.ValidFrom, a.ValidTo, p, a.RecordedAt); err != nil {
			return identity.Entity{}, mapError(err)
		}
		e, err := get(ctx, tx, id)
		if err != nil {
			return e, err
		}
		return e, record(tx, ctx, e, "assertion.recorded", map[string]any{"assertion_id": a.ID, "domain": a.Domain, "predicate": a.Predicate}, now)
	})
}
func (s *Store) FindByReference(ctx context.Context, r identity.ExternalReference) (identity.Entity, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT entity_id FROM external_references WHERE source_system=$1 AND object_type=$2 AND source_key=$3`, r.SourceSystem, r.ObjectType, r.SourceKey).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return identity.Entity{}, identity.ErrNotFound
	}
	if err != nil {
		return identity.Entity{}, err
	}
	return s.Get(ctx, id)
}
func (s *Store) List(ctx context.Context, p identity.Page) (identity.EntityPage, error) {
	if p.Limit <= 0 {
		p.Limit = 50
	}
	if p.Limit > 200 {
		return identity.EntityPage{}, fmt.Errorf("%w: limit must not exceed 200", identity.ErrInvalid)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,kind,created_at FROM entities WHERE id>$1 ORDER BY id LIMIT $2`, p.After, p.Limit+1)
	if err != nil {
		return identity.EntityPage{}, err
	}
	defer rows.Close()
	var ids []string
	entities := make(map[string]identity.Entity, p.Limit)
	for rows.Next() {
		var e identity.Entity
		if err := rows.Scan(&e.ID, &e.Kind, &e.CreatedAt); err != nil {
			return identity.EntityPage{}, err
		}
		e.CreatedAt = e.CreatedAt.UTC()
		ids = append(ids, e.ID)
		entities[e.ID] = e
	}
	if err := rows.Err(); err != nil {
		return identity.EntityPage{}, err
	}
	out := identity.EntityPage{}
	if len(ids) > p.Limit {
		ids = ids[:p.Limit]
		out.NextCursor = ids[len(ids)-1]
	}
	if len(ids) == 0 {
		return out, nil
	}
	args := make([]any, len(ids))
	holders := make([]string, len(ids))
	for i, id := range ids {
		args[i] = id
		holders[i] = fmt.Sprintf("$%d", i+1)
	}
	referenceRows, err := s.db.QueryContext(ctx, `SELECT entity_id,source_system,object_type,source_key,uri,observed_at FROM external_references WHERE entity_id IN (`+strings.Join(holders, ",")+`) ORDER BY entity_id,source_system,object_type,source_key`, args...)
	if err != nil {
		return out, err
	}
	for referenceRows.Next() {
		var (
			entityID string
			ref      identity.ExternalReference
		)
		if err := referenceRows.Scan(&entityID, &ref.SourceSystem, &ref.ObjectType, &ref.SourceKey, &ref.URI, &ref.ObservedAt); err != nil {
			referenceRows.Close()
			return out, err
		}
		ref.ObservedAt = ref.ObservedAt.UTC()
		e := entities[entityID]
		e.References = append(e.References, ref)
		entities[entityID] = e
	}
	if err := referenceRows.Err(); err != nil {
		referenceRows.Close()
		return out, err
	}
	referenceRows.Close()
	assertionRows, err := s.db.QueryContext(ctx, `SELECT entity_id,id,domain,predicate,value,valid_from,valid_to,provenance,recorded_at FROM assertions WHERE entity_id IN (`+strings.Join(holders, ",")+`) ORDER BY entity_id,recorded_at,id`, args...)
	if err != nil {
		return out, err
	}
	defer assertionRows.Close()
	for assertionRows.Next() {
		var (
			entityID string
			a        identity.Assertion
			p        []byte
		)
		if err := assertionRows.Scan(&entityID, &a.ID, &a.Domain, &a.Predicate, &a.Value, &a.ValidFrom, &a.ValidTo, &p, &a.RecordedAt); err != nil {
			return out, err
		}
		if err := json.Unmarshal(p, &a.Provenance); err != nil {
			return out, err
		}
		a.ValidFrom = a.ValidFrom.UTC()
		a.RecordedAt = a.RecordedAt.UTC()
		if a.ValidTo != nil {
			v := a.ValidTo.UTC()
			a.ValidTo = &v
		}
		e := entities[entityID]
		e.Assertions = append(e.Assertions, a)
		entities[entityID] = e
	}
	if err := assertionRows.Err(); err != nil {
		return out, err
	}
	for _, id := range ids {
		out.Items = append(out.Items, entities[id])
	}
	return out, nil
}

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func get(ctx context.Context, q queryer, id string) (identity.Entity, error) {
	var e identity.Entity
	if err := q.QueryRowContext(ctx, `SELECT id,kind,created_at FROM entities WHERE id=$1`, id).Scan(&e.ID, &e.Kind, &e.CreatedAt); errors.Is(err, sql.ErrNoRows) {
		return e, identity.ErrNotFound
	} else if err != nil {
		return e, err
	}
	e.CreatedAt = e.CreatedAt.UTC()
	rr, err := q.QueryContext(ctx, `SELECT source_system,object_type,source_key,uri,observed_at FROM external_references WHERE entity_id=$1 ORDER BY source_system,object_type,source_key`, id)
	if err != nil {
		return e, err
	}
	for rr.Next() {
		var r identity.ExternalReference
		if err := rr.Scan(&r.SourceSystem, &r.ObjectType, &r.SourceKey, &r.URI, &r.ObservedAt); err != nil {
			rr.Close()
			return e, err
		}
		r.ObservedAt = r.ObservedAt.UTC()
		e.References = append(e.References, r)
	}
	rr.Close()
	ar, err := q.QueryContext(ctx, `SELECT id,domain,predicate,value,valid_from,valid_to,provenance,recorded_at FROM assertions WHERE entity_id=$1 ORDER BY recorded_at,id`, id)
	if err != nil {
		return e, err
	}
	defer ar.Close()
	for ar.Next() {
		var a identity.Assertion
		var p []byte
		if err := ar.Scan(&a.ID, &a.Domain, &a.Predicate, &a.Value, &a.ValidFrom, &a.ValidTo, &p, &a.RecordedAt); err != nil {
			return e, err
		}
		if err := json.Unmarshal(p, &a.Provenance); err != nil {
			return e, err
		}
		a.ValidFrom = a.ValidFrom.UTC()
		a.RecordedAt = a.RecordedAt.UTC()
		if a.ValidTo != nil {
			v := a.ValidTo.UTC()
			a.ValidTo = &v
		}
		e.Assertions = append(e.Assertions, a)
	}
	return e, ar.Err()
}

func command(s *Store, ctx context.Context, name string, input any, fn func(*sql.Tx) (identity.Entity, error)) (identity.Entity, error) {
	raw, _ := json.Marshal(input)
	sum := sha256.Sum256(append([]byte(name+"\x00"), raw...))
	hash := hex.EncodeToString(sum[:])
	key := identity.IdempotencyKey(ctx)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return identity.Entity{}, err
	}
	defer tx.Rollback()
	if key != "" {
		result, err := tx.ExecContext(ctx, `INSERT INTO idempotency(key,command,request_hash,status,created_at) VALUES($1,$2,$3,'started',$4) ON CONFLICT DO NOTHING`, key, name, hash, s.now().UTC())
		if err != nil {
			return identity.Entity{}, err
		}
		inserted, err := result.RowsAffected()
		if err != nil {
			return identity.Entity{}, err
		}
		if inserted == 0 {
			var oldHash, status string
			var response []byte
			err = tx.QueryRowContext(ctx, `SELECT request_hash,status,response FROM idempotency WHERE key=$1 FOR UPDATE`, key).Scan(&oldHash, &status, &response)
			if err != nil {
				return identity.Entity{}, err
			}
			if oldHash != hash {
				return identity.Entity{}, fmt.Errorf("%w: idempotency key was used for a different command", identity.ErrConflict)
			}
			var e identity.Entity
			if status == "completed" && json.Unmarshal(response, &e) == nil {
				return e, nil
			}
			return identity.Entity{}, fmt.Errorf("%w: command is already in progress", identity.ErrConflict)
		}
	}
	e, err := fn(tx)
	if err != nil {
		return identity.Entity{}, err
	}
	if key != "" {
		response, _ := json.Marshal(e)
		if _, err = tx.ExecContext(ctx, `UPDATE idempotency SET status='completed',response=$2 WHERE key=$1`, key, response); err != nil {
			return identity.Entity{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		return identity.Entity{}, mapError(err)
	}
	return e, nil
}
func record(tx *sql.Tx, ctx context.Context, e identity.Entity, eventType string, details any, at time.Time) error {
	d, _ := json.Marshal(details)
	eventID := newID("evt")
	payload, _ := json.Marshal(identity.AuditEvent{ID: eventID, EntityID: e.ID, Type: eventType, OccurredAt: at, Details: d})
	if _, err := tx.ExecContext(ctx, `INSERT INTO audit_events(id,entity_id,event_type,occurred_at,details) VALUES($1,$2,$3,$4,$5)`, eventID, e.ID, eventType, at, d); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO outbox(id,topic,payload,occurred_at,available_at) VALUES($1,$2,$3,$4,$4)`, newID("msg"), eventType, payload, at)
	return err
}
func mapError(err error) error {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		if pe.Code == "23505" {
			return identity.ErrConflict
		}
		if pe.Code == "23503" {
			return identity.ErrNotFound
		}
	}
	return err
}
func newID(prefix string) string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(b)
}

// ClaimOutbox atomically leases messages. A crashed worker can recover them
// after leaseUntil; delivery is therefore at-least-once and consumers must deduplicate IDs.
func (s *Store) ClaimOutbox(ctx context.Context, limit int, leaseUntil time.Time) ([]identity.OutboxMessage, error) {
	if limit <= 0 || limit > 200 {
		return nil, fmt.Errorf("%w: invalid outbox limit", identity.ErrInvalid)
	}
	rows, err := s.db.QueryContext(ctx, `UPDATE outbox SET claimed_at=now(),available_at=$2,attempts=attempts+1 WHERE id IN (SELECT id FROM outbox WHERE delivered_at IS NULL AND available_at<=now() ORDER BY occurred_at,id FOR UPDATE SKIP LOCKED LIMIT $1) RETURNING id,topic,payload,occurred_at,attempts,available_at`, limit, leaseUntil.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []identity.OutboxMessage
	for rows.Next() {
		var m identity.OutboxMessage
		if err := rows.Scan(&m.ID, &m.Topic, &m.Payload, &m.OccurredAt, &m.Attempts, &m.AvailableAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
func (s *Store) AcknowledgeOutbox(ctx context.Context, id string) error {
	r, err := s.db.ExecContext(ctx, `UPDATE outbox SET delivered_at=now() WHERE id=$1 AND delivered_at IS NULL`, id)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return identity.ErrNotFound
	}
	return nil
}
