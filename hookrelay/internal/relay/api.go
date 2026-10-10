package relay

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

type API struct {
	Store        *Store
	Token        string
	AllowPrivate bool
}

func write(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
func failure(w http.ResponseWriter, status int, message string) {
	write(w, status, map[string]string{"error": message})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 256*1024)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		failure(w, 400, "Invalid JSON, unknown field, or body exceeds 256 KiB")
		return false
	}
	if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
		failure(w, 400, "Only one JSON value is allowed")
		return false
	}
	return true
}
func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := a.Store.Pool.Ping(r.Context()); err != nil {
			failure(w, 503, "Database unavailable")
			return
		}
		write(w, 200, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/endpoints", func(w http.ResponseWriter, r *http.Request) {
		items, err := a.Store.Endpoints(r.Context())
		if err != nil {
			a.internal(w, err)
			return
		}
		write(w, 200, items)
	})
	mux.HandleFunc("POST /api/endpoints", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name   string `json:"name"`
			URL    string `json:"url"`
			Secret string `json:"secret"`
		}
		if !decode(w, r, &body) {
			return
		}
		if strings.TrimSpace(body.Name) == "" || len(body.Name) > 120 || len(body.URL) > 2048 {
			failure(w, 400, "Name (1–120 characters) and URL are required")
			return
		}
		if err := ValidateURL(body.URL, a.AllowPrivate); err != nil {
			failure(w, 400, err.Error())
			return
		}
		if body.Secret != "" && (len(body.Secret) < 32 || len(body.Secret) > 256) {
			failure(w, 400, "Signing secret must contain 32–256 characters")
			return
		}
		endpoint, err := a.Store.AddEndpoint(r.Context(), body.Name, body.URL, body.Secret)
		if err != nil {
			a.internal(w, err)
			return
		}
		write(w, 201, endpoint)
	})
	mux.HandleFunc("POST /api/events", func(w http.ResponseWriter, r *http.Request) {
		var body EventRequest
		if !decode(w, r, &body) {
			return
		}
		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if key == "" || len(key) > 200 || body.EndpointID == "" || strings.TrimSpace(body.Type) == "" || len(body.Type) > 120 || len(body.Payload) == 0 || !json.Valid(body.Payload) {
			failure(w, 400, "Provide Idempotency-Key (1–200 characters), endpoint_id, type (1–120 characters), and valid payload")
			return
		}
		id, duplicate, err := a.Store.Enqueue(r.Context(), body, key)
		if errors.Is(err, ErrConflict) {
			failure(w, 409, err.Error())
			return
		}
		if errors.Is(err, ErrNotFound) {
			failure(w, 404, "Endpoint not found")
			return
		}
		if err != nil {
			a.internal(w, err)
			return
		}
		write(w, 202, map[string]any{"delivery_id": id, "duplicate": duplicate})
	})
	mux.HandleFunc("GET /api/deliveries", func(w http.ResponseWriter, r *http.Request) {
		items, err := a.Store.Deliveries(r.Context())
		if err != nil {
			a.internal(w, err)
			return
		}
		write(w, 200, items)
	})
	mux.HandleFunc("GET /api/deliveries/{id}/attempts", func(w http.ResponseWriter, r *http.Request) {
		items, err := a.Store.Attempts(r.Context(), r.PathValue("id"))
		if err != nil {
			a.internal(w, err)
			return
		}
		write(w, 200, items)
	})
	mux.HandleFunc("POST /api/deliveries/{id}/replay", func(w http.ResponseWriter, r *http.Request) {
		err := a.Store.Replay(r.Context(), r.PathValue("id"))
		if errors.Is(err, ErrNotReplayable) {
			failure(w, 409, err.Error())
			return
		}
		if err != nil {
			a.internal(w, err)
			return
		}
		write(w, 202, map[string]string{"status": "pending"})
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Path != "/healthz" {
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(a.Token)) != 1 {
				failure(w, 401, "A valid bearer token is required")
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
}
func (a *API) internal(w http.ResponseWriter, err error) {
	slog.Error("API operation failed", "error", err)
	failure(w, 500, "Internal error; inspect server logs")
}
