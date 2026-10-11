package relay

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

//go:embed schema.sql
var schema string
var ErrConflict = errors.New("idempotency key already used for a different request")
var ErrNotFound = errors.New("not found")
var ErrNotReplayable = errors.New("only delivered or dead deliveries can be replayed")

type Store struct{ Pool *pgxpool.Pool }
type Endpoint struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Secret    string    `json:"secret,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
type EventRequest struct {
	EndpointID string          `json:"endpoint_id"`
	Type       string          `json:"type"`
	Payload    json.RawMessage `json:"payload"`
}
type Delivery struct {
	ID            string    `json:"id"`
	EventID       string    `json:"event_id"`
	EndpointName  string    `json:"endpoint_name"`
	EventType     string    `json:"event_type"`
	Status        string    `json:"status"`
	AttemptCount  int       `json:"attempt_count"`
	Run           int       `json:"run"`
	NextAttemptAt time.Time `json:"next_attempt_at"`
	LastError     string    `json:"last_error"`
	CreatedAt     time.Time `json:"created_at"`
}
type Attempt struct {
	ID              int64     `json:"id"`
	Run             int       `json:"run"`
	Number          int       `json:"number"`
	StatusCode      int       `json:"status_code"`
	DurationMS      int64     `json:"duration_ms"`
	Error           string    `json:"error"`
	ResponseExcerpt string    `json:"response_excerpt"`
	CreatedAt       time.Time `json:"created_at"`
}
type Job struct {
	ID, EventID, URL, Secret, Type, LeaseToken string
	Payload                                    json.RawMessage
	Attempt, Run                               int
}

func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func (s *Store) Migrate(ctx context.Context) error {
	// Serializes startup of API and worker processes; schema v1 is intentionally additive.
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(742011)"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, schema); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) AddEndpoint(ctx context.Context, name, url, secret string) (Endpoint, error) {
	if secret == "" {
		secret = NewID() + NewID()
	}
	e := Endpoint{ID: NewID(), Name: name, URL: url, Secret: secret}
	err := s.Pool.QueryRow(ctx, "INSERT INTO endpoint(id,name,url,secret) VALUES ($1,$2,$3,$4) RETURNING created_at", e.ID, e.Name, e.URL, e.Secret).Scan(&e.CreatedAt)
	return e, err
}
func (s *Store) Endpoints(ctx context.Context) ([]Endpoint, error) {
	rows, err := s.Pool.Query(ctx, "SELECT id,name,url,created_at FROM endpoint ORDER BY created_at DESC LIMIT 200")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Endpoint{}
	for rows.Next() {
		var e Endpoint
		if err = rows.Scan(&e.ID, &e.Name, &e.URL, &e.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}
func (s *Store) Enqueue(ctx context.Context, r EventRequest, key string) (string, bool, error) {
	// Canonicalizing JSON makes whitespace/key ordering irrelevant for duplicate requests.
	var value any
	decoder := json.NewDecoder(bytes.NewReader(r.Payload))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return "", false, err
	}
	payload, _ := json.Marshal(value)
	raw, _ := json.Marshal(struct {
		Endpoint, Type string
		Payload        json.RawMessage
	}{r.EndpointID, r.Type, payload})
	hash := fmt.Sprintf("%x", sha256.Sum256(raw))
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback(ctx)
	// Lock same-key submissions before checking; persists the event and its job atomically.
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", key); err != nil {
		return "", false, err
	}
	var existingID, existingHash string
	err = tx.QueryRow(ctx, "SELECT d.id,e.request_hash FROM event e JOIN delivery d ON d.event_id=e.id WHERE e.idempotency_key=$1", key).Scan(&existingID, &existingHash)
	if err == nil {
		if existingHash != hash {
			return "", false, ErrConflict
		}
		return existingID, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", false, err
	}
	var exists bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM endpoint WHERE id=$1)", r.EndpointID).Scan(&exists); err != nil {
		return "", false, err
	}
	if !exists {
		return "", false, ErrNotFound
	}
	eventID, id := NewID(), NewID()
	if _, err = tx.Exec(ctx, "INSERT INTO event(id,endpoint_id,event_type,payload,idempotency_key,request_hash) VALUES ($1,$2,$3,$4,$5,$6)", eventID, r.EndpointID, r.Type, string(payload), key, hash); err != nil {
		return "", false, err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO delivery(id,event_id) VALUES ($1,$2)", id, eventID); err != nil {
		return "", false, err
	}
	return id, false, tx.Commit(ctx)
}
func (s *Store) Deliveries(ctx context.Context) ([]Delivery, error) {
	rows, err := s.Pool.Query(ctx, `SELECT d.id,d.event_id,ep.name,e.event_type,d.status,d.attempt_count,d.run,d.next_attempt_at,d.last_error,d.created_at FROM delivery d JOIN event e ON e.id=d.event_id JOIN endpoint ep ON ep.id=e.endpoint_id ORDER BY d.created_at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Delivery{}
	for rows.Next() {
		var d Delivery
		if err = rows.Scan(&d.ID, &d.EventID, &d.EndpointName, &d.EventType, &d.Status, &d.AttemptCount, &d.Run, &d.NextAttemptAt, &d.LastError, &d.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}
func (s *Store) Attempts(ctx context.Context, id string) ([]Attempt, error) {
	rows, err := s.Pool.Query(ctx, "SELECT id,run,number,status_code,duration_ms,error,response_excerpt,created_at FROM attempt WHERE delivery_id=$1 ORDER BY id", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Attempt{}
	for rows.Next() {
		var a Attempt
		if err = rows.Scan(&a.ID, &a.Run, &a.Number, &a.StatusCode, &a.DurationMS, &a.Error, &a.ResponseExcerpt, &a.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, rows.Err()
}
func (s *Store) Replay(ctx context.Context, id string) error {
	tag, err := s.Pool.Exec(ctx, "UPDATE delivery SET status='pending',run=run+1,attempt_count=0,next_attempt_at=now(),leased_until=NULL,lease_token=NULL,last_error='',delivered_at=NULL WHERE id=$1 AND status IN ('dead','delivered')", id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotReplayable
	}
	return nil
}
func (s *Store) Claim(ctx context.Context, lease time.Duration) (*Job, error) {
	token := NewID()
	var job Job
	err := s.Pool.QueryRow(ctx, `WITH candidate AS (
 SELECT id FROM delivery WHERE (status IN ('pending','retrying') AND next_attempt_at<=now()) OR (status='processing' AND leased_until<now())
 ORDER BY next_attempt_at,id FOR UPDATE SKIP LOCKED LIMIT 1
 ), claimed AS (
 UPDATE delivery d SET status='processing',leased_until=now()+($1::double precision * interval '1 second'),lease_token=$2,attempt_count=attempt_count+1
 FROM candidate c WHERE d.id=c.id RETURNING d.*
 ) SELECT d.id,e.id,ep.url,ep.secret,e.event_type,e.payload,d.attempt_count,d.run,d.lease_token
 FROM claimed d JOIN event e ON e.id=d.event_id JOIN endpoint ep ON ep.id=e.endpoint_id`, lease.Seconds(), token).Scan(&job.ID, &job.EventID, &job.URL, &job.Secret, &job.Type, &job.Payload, &job.Attempt, &job.Run, &job.LeaseToken)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &job, nil
}

type Result struct {
	Code           int
	Duration       time.Duration
	Error, Excerpt string
}

func (s *Store) Complete(ctx context.Context, j *Job, r Result, maxAttempts int, delay time.Duration) (bool, error) {
	status := "retrying"
	if r.Code >= 200 && r.Code < 300 {
		status = "delivered"
	} else if j.Attempt >= maxAttempts {
		status = "dead"
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE delivery SET status=$1,last_error=$2,next_attempt_at=now()+($3::double precision*interval '1 second'),leased_until=NULL,lease_token=NULL,
 delivered_at=CASE WHEN $1='delivered' THEN now() ELSE NULL END WHERE id=$4 AND lease_token=$5 AND status='processing'`, status, r.Error, delay.Seconds(), j.ID, j.LeaseToken)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	if _, err = tx.Exec(ctx, "INSERT INTO attempt(delivery_id,run,number,status_code,duration_ms,error,response_excerpt) VALUES ($1,$2,$3,$4,$5,$6,$7)", j.ID, j.Run, j.Attempt, r.Code, r.Duration.Milliseconds(), r.Error, r.Excerpt); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}
