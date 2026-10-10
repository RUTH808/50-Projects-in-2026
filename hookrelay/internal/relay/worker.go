package relay

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func Signature(secret, timestamp string, body []byte) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(timestamp + "."))
	m.Write(body)
	return "v1=" + hex.EncodeToString(m.Sum(nil))
}
func Verify(secret, timestamp, signature string, body []byte, now time.Time) bool {
	t, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || t < now.Unix()-300 || t > now.Unix()+300 {
		return false
	}
	return hmac.Equal([]byte(Signature(secret, timestamp, body)), []byte(signature))
}
func publicIP(ip net.IP) bool {
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, cidr := range []string{"100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32"} {
		_, block, _ := net.ParseCIDR(cidr)
		if block.Contains(ip) {
			return false
		}
	}
	return true
}
func ValidateURL(raw string, allowPrivate bool) error {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return errors.New("invalid endpoint URL")
	}
	if u.Scheme != "https" && !(allowPrivate && u.Scheme == "http") {
		return errors.New("HTTPS is required outside local demo mode")
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil && !allowPrivate && !publicIP(ip) {
		return errors.New("private or reserved destinations are blocked")
	}
	return nil
}
func HTTPClient(allowPrivate bool) *http.Client {
	transport := &http.Transport{Proxy: nil, MaxIdleConns: 20, IdleConnTimeout: 30 * time.Second, TLSHandshakeTimeout: 5 * time.Second,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			if len(ips) == 0 {
				return nil, errors.New("no IP addresses resolved")
			}
			for _, ip := range ips {
				if !allowPrivate && !publicIP(ip.IP) {
					return nil, errors.New("private or reserved destinations are blocked")
				}
			}
			dialer := net.Dialer{Timeout: 5 * time.Second}
			// Dial the validated address directly, preventing a second DNS lookup/rebinding.
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
		}}
	return &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
}
func Backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 8 {
		attempt = 8
	}
	base := time.Second * time.Duration(1<<uint(attempt))
	return base + time.Duration(rand.Int64N(int64(base/2)))
}

type Worker struct {
	Store        *Store
	Client       *http.Client
	MaxAttempts  int
	Poll         time.Duration
	Lease        time.Duration
	AllowPrivate bool
}

func (w *Worker) Deliver(ctx context.Context, j *Job) Result {
	started := time.Now()
	result := Result{}
	if err := ValidateURL(j.URL, w.AllowPrivate); err != nil {
		return Result{Error: err.Error()}
	}
	body, err := json.Marshal(struct {
		ID   string          `json:"id"`
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}{j.EventID, j.Type, j.Payload})
	if err != nil {
		return Result{Error: err.Error()}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, j.URL, bytes.NewReader(body))
	if err != nil {
		return Result{Error: err.Error()}
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "HookRelay/1.0")
	req.Header.Set("X-HookRelay-Id", j.ID)
	req.Header.Set("X-HookRelay-Timestamp", timestamp)
	req.Header.Set("X-HookRelay-Signature", Signature(j.Secret, timestamp, body))
	response, err := w.Client.Do(req)
	result.Duration = time.Since(started)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer response.Body.Close()
	excerpt, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
	result.Excerpt = strings.ToValidUTF8(string(excerpt), "�")
	result.Code = response.StatusCode
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		result.Error = fmt.Sprintf("receiver returned HTTP %d", response.StatusCode)
	}
	return result
}
func (w *Worker) Run(ctx context.Context) {
	timer := time.NewTicker(w.Poll)
	defer timer.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		job, err := w.Store.Claim(ctx, w.Lease)
		if err != nil {
			slog.Error("claim failed", "error", err)
		}
		if job != nil {
			// A process dying after receiver acceptance but before completion can cause a duplicate.
			r := w.Deliver(ctx, job)
			if ctx.Err() != nil {
				return
			} // Leave lease for recovery on shutdown.
			done, err := w.Store.Complete(ctx, job, r, w.MaxAttempts, Backoff(job.Attempt))
			if err != nil {
				slog.Error("completion failed", "delivery", job.ID, "error", err)
			} else {
				slog.Info("delivery attempt", "delivery", job.ID, "status_code", r.Code, "attempt", job.Attempt, "lease_owned", done)
			}
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
	}
}
