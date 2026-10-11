CREATE TABLE IF NOT EXISTS endpoint (
  id TEXT PRIMARY KEY, name TEXT NOT NULL, url TEXT NOT NULL, secret TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS event (
  id TEXT PRIMARY KEY, endpoint_id TEXT NOT NULL REFERENCES endpoint(id),
  event_type TEXT NOT NULL, payload JSONB NOT NULL, idempotency_key TEXT UNIQUE NOT NULL,
  request_hash TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS delivery (
  id TEXT PRIMARY KEY, event_id TEXT UNIQUE NOT NULL REFERENCES event(id),
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','processing','retrying','delivered','dead')),
  attempt_count INTEGER NOT NULL DEFAULT 0, run INTEGER NOT NULL DEFAULT 1,
  next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(), leased_until TIMESTAMPTZ,
  lease_token TEXT, last_error TEXT NOT NULL DEFAULT '', created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  delivered_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS delivery_due ON delivery(next_attempt_at) WHERE status IN ('pending','retrying','processing');
CREATE TABLE IF NOT EXISTS attempt (
  id BIGSERIAL PRIMARY KEY, delivery_id TEXT NOT NULL REFERENCES delivery(id),
  run INTEGER NOT NULL, number INTEGER NOT NULL, status_code INTEGER NOT NULL,
  duration_ms BIGINT NOT NULL, error TEXT NOT NULL, response_excerpt TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS attempt_delivery ON attempt(delivery_id,id);
