CREATE TABLE IF NOT EXISTS schema_migrations (version bigint PRIMARY KEY, applied_at timestamptz NOT NULL);
CREATE TABLE entities (id text PRIMARY KEY, kind text NOT NULL CHECK (kind <> ''), created_at timestamptz NOT NULL);
CREATE TABLE external_references (
 entity_id text NOT NULL REFERENCES entities(id), source_system text NOT NULL, object_type text NOT NULL,
 source_key text NOT NULL, uri text NOT NULL DEFAULT '', observed_at timestamptz NOT NULL,
 PRIMARY KEY (source_system, object_type, source_key)
);
CREATE INDEX external_references_entity_idx ON external_references(entity_id, source_system, object_type, source_key);
CREATE TABLE assertions (
 id text PRIMARY KEY, entity_id text NOT NULL REFERENCES entities(id), domain text NOT NULL, predicate text NOT NULL,
 value jsonb NOT NULL, valid_from timestamptz NOT NULL, valid_to timestamptz, provenance jsonb NOT NULL, recorded_at timestamptz NOT NULL
);
CREATE INDEX assertions_entity_idx ON assertions(entity_id, recorded_at, id);
CREATE TABLE audit_events (
 id text PRIMARY KEY, entity_id text NOT NULL REFERENCES entities(id), event_type text NOT NULL,
 occurred_at timestamptz NOT NULL, details jsonb NOT NULL
);
CREATE TABLE outbox (
 id text PRIMARY KEY, topic text NOT NULL, payload jsonb NOT NULL, occurred_at timestamptz NOT NULL,
 attempts integer NOT NULL DEFAULT 0, available_at timestamptz NOT NULL, delivered_at timestamptz,
 claimed_at timestamptz
);
CREATE TABLE idempotency (
 key text PRIMARY KEY, command text NOT NULL, request_hash text NOT NULL, status text NOT NULL,
 response jsonb, created_at timestamptz NOT NULL
);
