"""End-to-end retry, ingestion-idempotency and replay demonstration. Python 3.10+."""
import json
import os
import time
import urllib.request
import uuid

API = os.environ.get("RELAY_URL", "http://localhost:8090")
RECEIVER = os.environ.get("RECEIVER_URL", "http://localhost:8091")
ENDPOINT_URL = os.environ.get("ENDPOINT_URL", "http://receiver:8091/webhook")
TOKEN = os.environ.get("API_TOKEN", "hookrelay-local-development-token-2026")
SECRET = os.environ.get("DEMO_WEBHOOK_SECRET", "hookrelay-demo-signing-secret-change-me")

def request(base, path, body=None, key=None):
    headers = {"Authorization": "Bearer " + TOKEN}
    if key:
        headers["Idempotency-Key"] = key
    raw = json.dumps(body).encode() if body is not None else None
    if raw is not None:
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(base + path, data=raw, headers=headers)
    with urllib.request.urlopen(req, timeout=15) as response:
        return json.load(response)

def wait_for_delivery(delivery_id, predicate, timeout=100):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        items = request(API, "/api/deliveries")
        delivery = next((d for d in items if d["id"] == delivery_id), None)
        if delivery and predicate(delivery):
            return delivery
        time.sleep(0.5)
    raise AssertionError("Delivery did not reach the expected state")

def main():
    for _ in range(60):
        try:
            request(API, "/healthz")
            request(RECEIVER, "/events")
            break
        except Exception:
            time.sleep(1)
    else:
        raise RuntimeError("Start docker compose first, or check RELAY_URL and RECEIVER_URL")
    endpoint = request(API, "/api/endpoints", {"name": "Retry demo " + uuid.uuid4().hex[:6], "url": ENDPOINT_URL, "secret": SECRET})
    request(RECEIVER, "/control", {"fail_next": 2})
    key = "demo-" + uuid.uuid4().hex
    event = {"endpoint_id": endpoint["id"], "type": "booking.confirmed", "payload": {"booking_id": key, "resource": "Camera kit"}}
    accepted = request(API, "/api/events", event, key)
    duplicate = request(API, "/api/events", event, key)
    assert duplicate["duplicate"] and duplicate["delivery_id"] == accepted["delivery_id"]
    delivered = wait_for_delivery(accepted["delivery_id"], lambda d: d["status"] == "delivered")
    attempts = request(API, "/api/deliveries/" + delivered["id"] + "/attempts")
    assert [a["status_code"] for a in attempts] == [503, 503, 200], attempts
    print("PASS: duplicate ingestion returns one job; two failures recover on attempt three.")
    request(API, "/api/deliveries/" + delivered["id"] + "/replay", {})
    replay = wait_for_delivery(delivered["id"], lambda d: d["status"] == "delivered" and d["run"] == 2)
    received = request(RECEIVER, "/events")
    assert sum(e["id"] == replay["event_id"] for e in received["events"]) == 1
    attempts = request(API, "/api/deliveries/" + delivered["id"] + "/attempts")
    assert json.loads(attempts[-1]["response_excerpt"])["duplicate"] is True
    print("PASS: replay preserves event identity; the receiver applies the effect once.")
    print("Open http://localhost:3001 to inspect the recorded attempts.")

if __name__ == "__main__":
    main()
