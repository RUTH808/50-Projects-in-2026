import importlib.util
import hashlib
import hmac
import json
import os
import sqlite3
import tempfile
import threading
import time
import unittest
import urllib.request
import urllib.error

class ReceiverTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temp = tempfile.TemporaryDirectory()
        os.environ["RECEIVER_DB"] = cls.temp.name + "/receiver.db"
        spec = importlib.util.spec_from_file_location("receiver", os.path.join(os.path.dirname(__file__), "receiver.py"))
        cls.module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(cls.module)
        cls.server = cls.module.ThreadingHTTPServer(("127.0.0.1", 0), cls.module.Handler)
        cls.thread = threading.Thread(target=cls.server.serve_forever, daemon=True)
        cls.thread.start()
        cls.url = "http://127.0.0.1:" + str(cls.server.server_port)

    @classmethod
    def tearDownClass(cls):
        cls.server.shutdown()
        cls.server.server_close()
        cls.temp.cleanup()

    def send(self, payload, timestamp=None, secret=None):
        raw = json.dumps(payload).encode()
        timestamp = str(timestamp if timestamp is not None else int(time.time()))
        secret = secret or self.module.SECRET
        signature = "v1=" + hmac.new(secret.encode(), timestamp.encode() + b"." + raw, hashlib.sha256).hexdigest()
        req = urllib.request.Request(self.url + "/webhook", data=raw, headers={"X-HookRelay-Timestamp": timestamp, "X-HookRelay-Signature": signature})
        with urllib.request.urlopen(req) as response:
            return json.load(response)

    def test_deduplicates_signed_event_id(self):
        self.assertFalse(self.send({"id": "same", "data": {"booking": 42}})["duplicate"])
        self.assertTrue(self.send({"id": "same", "data": {"booking": 42}})["duplicate"])
        with sqlite3.connect(self.module.DB) as db:
            self.assertEqual(db.execute("SELECT count(*) FROM received WHERE id='same'").fetchone()[0], 1)

    def test_rejects_invalid_signature_and_expired_timestamp(self):
        with self.assertRaises(urllib.error.HTTPError) as error:
            self.send({"id": "invalid"}, secret="wrong")
        self.assertEqual(error.exception.code, 401)
        with self.assertRaises(urllib.error.HTTPError) as error:
            self.send({"id": "expired"}, timestamp=int(time.time()) - 1000)
        self.assertEqual(error.exception.code, 401)

    def test_control_requires_authentication(self):
        req = urllib.request.Request(self.url + "/control", data=b'{"fail_next":2}')
        with self.assertRaises(urllib.error.HTTPError) as error:
            urllib.request.urlopen(req)
        self.assertEqual(error.exception.code, 401)

if __name__ == "__main__":
    unittest.main()
