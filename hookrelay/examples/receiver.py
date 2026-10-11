"""Local-only receiver: HMAC verification, SQLite deduplication and injected failures.
No third-party Python dependencies. /control and /events require CONTROL_TOKEN.
"""
import hashlib
import hmac
import json
import os
import sqlite3
import time
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

SECRET = os.environ.get("WEBHOOK_SECRET", "hookrelay-demo-signing-secret-change-me")
CONTROL_TOKEN = os.environ.get("CONTROL_TOKEN", "hookrelay-local-development-token-2026")
DB = os.environ.get("RECEIVER_DB", "receiver.db")
lock = threading.Lock()
failures = 0
with sqlite3.connect(DB) as db:
    db.execute("CREATE TABLE IF NOT EXISTS received(id TEXT PRIMARY KEY, payload TEXT NOT NULL, received_at REAL NOT NULL)")

class Handler(BaseHTTPRequestHandler):
    def reply(self, code, value):
        raw = json.dumps(value).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def authorized(self):
        return hmac.compare_digest(self.headers.get("Authorization", ""), "Bearer " + CONTROL_TOKEN)

    def do_GET(self):
        if self.path != "/events":
            return self.reply(404, {"error": "not found"})
        if not self.authorized():
            return self.reply(401, {"error": "control token required"})
        with sqlite3.connect(DB) as db:
            events = [json.loads(row[0]) for row in db.execute("SELECT payload FROM received ORDER BY received_at DESC LIMIT 200")]
        self.reply(200, {"processed_count": len(events), "events": events})

    def do_POST(self):
        global failures
        try:
            length = int(self.headers.get("Content-Length", "0"))
            if length <= 0 or length > 300_000:
                return self.reply(400, {"error": "invalid body size"})
            raw = self.rfile.read(length)
            body = json.loads(raw)
        except (ValueError, json.JSONDecodeError):
            return self.reply(400, {"error": "invalid JSON"})
        if self.path == "/control":
            if not self.authorized():
                return self.reply(401, {"error": "control token required"})
            if not isinstance(body, dict):
                return self.reply(400, {"error": "JSON object required"})
            count = body.get("fail_next", 0)
            if not isinstance(count, int) or not 0 <= count <= 100:
                return self.reply(400, {"error": "fail_next must be an integer from 0 to 100"})
            with lock:
                failures = count
            return self.reply(200, {"fail_next": count})
        if self.path != "/webhook":
            return self.reply(404, {"error": "not found"})
        timestamp = self.headers.get("X-HookRelay-Timestamp", "")
        signature = self.headers.get("X-HookRelay-Signature", "")
        expected = "v1=" + hmac.new(SECRET.encode(), timestamp.encode() + b"." + raw, hashlib.sha256).hexdigest()
        try:
            valid_time = abs(time.time() - int(timestamp)) <= 300
        except ValueError:
            valid_time = False
        if not valid_time or not hmac.compare_digest(signature, expected):
            return self.reply(401, {"error": "invalid or expired signature"})
        if not isinstance(body, dict) or not isinstance(body.get("id"), str):
            return self.reply(400, {"error": "signed event ID required"})
        with lock:
            if failures:
                failures -= 1
                return self.reply(503, {"error": "injected temporary failure"})
        # The event ID comes from the signed body. Dedup and the example effect are one transaction.
        with sqlite3.connect(DB) as db:
            changed = db.execute("INSERT OR IGNORE INTO received VALUES (?,?,?)", (body["id"], raw.decode(), time.time())).rowcount
        self.reply(200, {"accepted": True, "duplicate": not bool(changed)})

    def log_message(self, fmt, *args):
        print(json.dumps({"component": "demo-receiver", "message": fmt % args}), flush=True)

if __name__ == "__main__":
    ThreadingHTTPServer(("0.0.0.0", 8091), Handler).serve_forever()
