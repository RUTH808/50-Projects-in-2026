CREATE EXTENSION IF NOT EXISTS btree_gist;
CREATE TABLE app_user (
  id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL, email TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL, role TEXT NOT NULL CHECK (role IN ('STUDENT','STAFF','ADMIN'))
);
CREATE TABLE equipment (
  id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL, category TEXT NOT NULL,
  location TEXT NOT NULL, description TEXT NOT NULL DEFAULT '',
  maintenance BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE TABLE booking (
  id BIGSERIAL PRIMARY KEY, equipment_id BIGINT NOT NULL REFERENCES equipment(id),
  user_id BIGINT NOT NULL REFERENCES app_user(id), starts_at TIMESTAMPTZ NOT NULL,
  ends_at TIMESTAMPTZ NOT NULL, purpose TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('PENDING','CONFIRMED','WAITLISTED','CHECKED_OUT','RETURNED','CANCELLED','REJECTED')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), CHECK (ends_at > starts_at),
  EXCLUDE USING gist (equipment_id WITH =, tstzrange(starts_at,ends_at,'[)') WITH &&)
    WHERE (status IN ('PENDING','CONFIRMED','CHECKED_OUT'))
);
CREATE INDEX booking_waitlist ON booking(equipment_id,created_at,id) WHERE status='WAITLISTED';
CREATE INDEX booking_user ON booking(user_id,created_at DESC);
CREATE TABLE maintenance_ticket (
  id BIGSERIAL PRIMARY KEY, equipment_id BIGINT NOT NULL REFERENCES equipment(id),
  reported_by BIGINT NOT NULL REFERENCES app_user(id), description TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN','RESOLVED')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(), resolved_at TIMESTAMPTZ
);
CREATE TABLE audit_event (
  id BIGSERIAL PRIMARY KEY, actor_id BIGINT NOT NULL REFERENCES app_user(id),
  action TEXT NOT NULL, entity_type TEXT NOT NULL, entity_id BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
