package relay

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"sync"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; PostgreSQL integration test skipped")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	s := &Store{Pool: pool}
	if err = s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(context.Background(), "TRUNCATE attempt,delivery,event,endpoint CASCADE"); err != nil {
		t.Fatal(err)
	}
	return s
}
func TestPostgresIdempotencyAndAtomicIngestion(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	e, err := s.AddEndpoint(ctx, "Test", "https://example.com/hook", "")
	if err != nil {
		t.Fatal(err)
	}
	r := EventRequest{EndpointID: e.ID, Type: "test", Payload: json.RawMessage(`{"large":9007199254740993,"x":1}`)}
	id, dup, err := s.Enqueue(ctx, r, "same-key")
	if err != nil || dup {
		t.Fatalf("enqueue: %v", err)
	}
	r.Payload = json.RawMessage(`{ "x": 1, "large": 9007199254740993 }`)
	id2, dup, err := s.Enqueue(ctx, r, "same-key")
	if err != nil || !dup || id2 != id {
		t.Fatal("idempotency failed")
	}
	r.Payload = json.RawMessage(`{"large":9007199254740992,"x":1}`)
	_, _, err = s.Enqueue(ctx, r, "same-key")
	if !errors.Is(err, ErrConflict) {
		t.Fatal("different payload accepted")
	}
	var events, jobs int
	s.Pool.QueryRow(ctx, "SELECT count(*) FROM event").Scan(&events)
	s.Pool.QueryRow(ctx, "SELECT count(*) FROM delivery").Scan(&jobs)
	if events != 1 || jobs != 1 {
		t.Fatal(events, jobs)
	}
}
func TestPostgresCompetingClaimsAndExpiredLeaseRecovery(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	e, _ := s.AddEndpoint(ctx, "Test", "https://example.com/hook", "")
	id, _, err := s.Enqueue(ctx, EventRequest{e.ID, "test", json.RawMessage(`{}`)}, "k")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	claims := []*Job{}
	errs := []error{}
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			j, err := s.Claim(ctx, time.Minute)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
			}
			if j != nil {
				claims = append(claims, j)
			}
		}()
	}
	wg.Wait()
	if len(errs) > 0 || len(claims) != 1 {
		t.Fatalf("claims=%d errors=%v", len(claims), errs)
	}
	original := claims[0]
	s.Pool.Exec(ctx, "UPDATE delivery SET leased_until=now()-interval '1 second' WHERE id=$1", id)
	recovered, err := s.Claim(ctx, time.Minute)
	if err != nil || recovered == nil || recovered.LeaseToken == original.LeaseToken {
		t.Fatal("lease not recovered")
	}
	ok, err := s.Complete(ctx, original, Result{Code: 200}, 5, 0)
	if err != nil || ok {
		t.Fatal("stale lease completed recovered job")
	}
	ok, err = s.Complete(ctx, recovered, Result{Code: 200}, 5, 0)
	if err != nil || !ok {
		t.Fatal("new owner could not complete")
	}
}
func TestPostgresDeadLetterAndReplayKeepsEventIdentity(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	e, _ := s.AddEndpoint(ctx, "Test", "https://example.com/hook", "")
	id, _, err := s.Enqueue(ctx, EventRequest{e.ID, "test", json.RawMessage(`{}`)}, "k")
	if err != nil {
		t.Fatal(err)
	}
	var eventID string
	for i := 0; i < 3; i++ {
		j, err := s.Claim(ctx, time.Minute)
		if err != nil || j == nil {
			t.Fatal("claim failed")
		}
		eventID = j.EventID
		ok, err := s.Complete(ctx, j, Result{Code: 503, Error: "unavailable"}, 3, 0)
		if err != nil || !ok {
			t.Fatal("complete failed")
		}
	}
	d, _ := s.Deliveries(ctx)
	if d[0].Status != "dead" {
		t.Fatal(d[0].Status)
	}
	if err = s.Replay(ctx, id); err != nil {
		t.Fatal(err)
	}
	j, err := s.Claim(ctx, time.Minute)
	if err != nil || j == nil || j.EventID != eventID || j.Attempt != 1 || j.Run != 2 {
		t.Fatal("replay identity or counters wrong")
	}
	if err = s.Replay(ctx, id); !errors.Is(err, ErrNotReplayable) {
		t.Fatal("in-flight replay allowed")
	}
}
