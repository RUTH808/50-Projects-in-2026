"""CampusFlow HTTP workflow demonstration. Requires a disposable local demo database."""
import base64
import datetime as dt
import http.cookiejar
import json
import os
import urllib.request
import uuid

BASE = os.environ.get("CAMPUSFLOW_URL", "http://localhost:8080")
PASSWORD = os.environ.get("DEMO_PASSWORD", "CampusFlow-local-2026!")

class Client:
    def __init__(self, role):
        self.auth = base64.b64encode((role + "@campusflow.local:" + PASSWORD).encode()).decode()
        self.opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
        self.csrf = self.call("/csrf")["token"]

    def call(self, path, body=None):
        headers = {"Authorization": "Basic " + self.auth}
        raw = json.dumps(body).encode() if body is not None else None
        if raw is not None:
            headers.update({"Content-Type": "application/json", "X-XSRF-TOKEN": self.csrf})
        req = urllib.request.Request(BASE + "/api" + path, data=raw, headers=headers)
        with self.opener.open(req, timeout=15) as response:
            return json.load(response)

def main():
    student, staff, admin = Client("student"), Client("staff"), Client("admin")
    item = admin.call("/admin/equipment", {"name": "Demo camera " + uuid.uuid4().hex[:6], "category": "Media", "location": "Demo Lab", "description": "Created by examples/demo.py"})
    start = dt.datetime.now(dt.timezone.utc) + dt.timedelta(hours=1)
    interval = {"equipmentId": item["id"], "startsAt": start.isoformat(), "endsAt": (start + dt.timedelta(hours=1)).isoformat(), "purpose": "Student demo"}
    first = student.call("/bookings", interval)
    interval["purpose"] = "Staff demo"
    second = staff.call("/bookings", interval)
    assert first["status"] == "PENDING" and second["status"] == "WAITLISTED"
    staff.call("/bookings/" + str(first["id"]) + "/approve", {})
    student.call("/bookings/" + str(first["id"]) + "/cancel", {})
    promoted = next(b for b in staff.call("/bookings") if b["id"] == second["id"])
    assert promoted["status"] == "PENDING"
    print("PASS: overlapping requests waitlist; cancellation promotes the next eligible request.")
    ticket = student.call("/maintenance", {"equipmentId": item["id"], "description": "Demo repair"})
    equipment = next(e for e in student.call("/equipment") if e["id"] == item["id"])
    assert equipment["maintenance"] is True
    staff.call("/staff/maintenance/" + str(ticket["id"]) + "/resolve", {})
    equipment = next(e for e in student.call("/equipment") if e["id"] == item["id"])
    assert equipment["maintenance"] is False
    print("PASS: reporting blocks equipment; resolving the last issue restores serviceability.")

if __name__ == "__main__":
    main()
