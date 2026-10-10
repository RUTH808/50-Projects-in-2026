package relay

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSignatureVerification(t *testing.T) {
	body := []byte(`{"id":"e1"}`)
	timestamp := "1700000000"
	now := time.Unix(1700000000, 0)
	signature := Signature("secret", timestamp, body)
	if !Verify("secret", timestamp, signature, body, now) {
		t.Fatal("valid signature rejected")
	}
	if Verify("secret", timestamp, signature, []byte(`{"id":"e2"}`), now) {
		t.Fatal("tampered payload accepted")
	}
	if Verify("secret", timestamp, signature, body, now.Add(301*time.Second)) {
		t.Fatal("expired signature accepted")
	}
	if Verify("secret", timestamp, signature, body, now.Add(-301*time.Second)) {
		t.Fatal("future signature accepted")
	}
}
func TestURLPolicy(t *testing.T) {
	for _, raw := range []string{"http://example.com", "https://127.0.0.1", "https://10.0.0.1", "https://169.254.169.254/latest", "https://[::1]", "https://user:pass@example.com", "file:///etc/passwd", "https://example.com/#fragment"} {
		if ValidateURL(raw, false) == nil {
			t.Fatalf("accepted unsafe URL: %s", raw)
		}
	}
	if ValidateURL("https://example.com/webhook", false) != nil {
		t.Fatal("public HTTPS rejected")
	}
	if ValidateURL("http://localhost:8091/webhook", true) != nil {
		t.Fatal("local demo rejected")
	}
}
func TestDeliverSignsExactBodyAndDoesNotFollowRedirects(t *testing.T) {
	var received bool
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !Verify("shared-secret", r.Header.Get("X-HookRelay-Timestamp"), r.Header.Get("X-HookRelay-Signature"), body, time.Now()) {
			t.Error("invalid signature")
		}
		var event map[string]any
		json.Unmarshal(body, &event)
		if event["id"] != "event1" || r.Header.Get("X-HookRelay-Id") != "job1" {
			t.Error("missing IDs")
		}
		received = true
		w.WriteHeader(204)
	}))
	defer receiver.Close()
	worker := Worker{Client: HTTPClient(true), AllowPrivate: true}
	job := &Job{ID: "job1", EventID: "event1", URL: receiver.URL, Secret: "shared-secret", Type: "test", Payload: json.RawMessage(`{"value":1}`)}
	result := worker.Deliver(context.Background(), job)
	if !received || result.Code != 204 || result.Error != "" {
		t.Fatalf("delivery failed: %+v", result)
	}
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, receiver.URL, 302) }))
	defer redirect.Close()
	received = false
	job.URL = redirect.URL
	result = worker.Deliver(context.Background(), job)
	if result.Code != 302 || received {
		t.Fatal("redirect was followed")
	}
}
func TestDialBlocksPrivateDNSDestinations(t *testing.T) {
	client := HTTPClient(false)
	_, err := client.Get("https://localhost:8091/webhook")
	if err == nil {
		t.Fatal("private resolved address accepted")
	}
}
func TestRetryBackoffBounds(t *testing.T) {
	for n := 1; n <= 5; n++ {
		d := Backoff(n)
		base := time.Second * time.Duration(1<<uint(n))
		if d < base || d >= base+base/2 {
			t.Fatal("backoff outside jitter bounds")
		}
	}
}
func TestAPIRequiresTokenAndValidJSON(t *testing.T) {
	api := (&API{Token: "a-long-test-token-123456789"}).Handler()
	w := httptest.NewRecorder()
	api.ServeHTTP(w, httptest.NewRequest("GET", "/api/endpoints", nil))
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
	r := httptest.NewRequest("POST", "/api/events", nil)
	r.Header.Set("Authorization", "Bearer a-long-test-token-123456789")
	w = httptest.NewRecorder()
	api.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
}
