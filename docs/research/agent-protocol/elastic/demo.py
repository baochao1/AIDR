"""Elastic Agent / Fleet 认证过程本地 demo。

启动 mock Kibana / Fleet Server / Elasticsearch，走通注册令牌签发、enroll、
check-in、ACK 与 Bulk 写入的凭据流转。仅使用标准库，不连接真实服务。
"""

import base64
import json
import threading
import urllib.error
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

ENROLL_KEY = base64.b64encode(b"enroll-id:enroll-secret").decode()
ACCESS_KEY = base64.b64encode(b"access-id:access-secret").decode()
OUTPUT_RAW = "output-id:output-secret"
OUTPUT_KEY = base64.b64encode(OUTPUT_RAW.encode()).decode()
AGENT_ID = "26b0b025-0f29-430c-a003-29efadb46340"

state = {"checkins": 0, "acks": 0, "bulk": 0}


class Handler(BaseHTTPRequestHandler):
    def log_message(self, *_args):
        pass

    def _json(self, status, payload):
        body = json.dumps(payload).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _apikey(self):
        value = self.headers.get("Authorization", "")
        return value[7:] if value.startswith("ApiKey ") else None

    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0"))
        raw = self.rfile.read(length)
        path = self.path.split("?")[0]

        if path == "/api/fleet/enrollment_api_keys":
            if self.headers.get("kbn-xsrf") != "1":
                return self._json(400, {"error": "BadRequest", "message": "missing kbn-xsrf"})
            return self._json(200, {"action": "created", "item": {"api_key": ENROLL_KEY}})

        key = self._apikey()
        if path == "/api/fleet/agents/enroll":
            if key is None:
                return self._json(401, {"statusCode": 401, "error": "ErrNoAuthHeader"})
            if key != ENROLL_KEY:
                return self._json(401, {"statusCode": 401, "error": "ErrInvalidAPIKey"})
            payload = json.loads(raw or b"{}")
            if payload.get("type") != "PERMANENT":
                return self._json(400, {"statusCode": 400, "error": "BadRequest"})
            return self._json(200, {
                "action": "created",
                "item": {"id": AGENT_ID, "access_api_key": ACCESS_KEY,
                         "access_api_key_id": "access-id", "active": True,
                         "status": "online", "type": "PERMANENT", "actions": None,
                         "user_provided_metadata": None, "local_metadata": {},
                         "tags": ["lab"], "policy_id": "lab-policy",
                         "enrolled_at": "2026-01-01T00:00:00Z"},
            })

        if "/checkin" in path:
            if key != ACCESS_KEY:
                return self._json(401, {"statusCode": 401, "error": "ErrInvalidAPIKey"})
            state["checkins"] += 1
            return self._json(200, {
                "action": "checkin",
                "ack_token": "delivery-token",
                "actions": [{"id": "settings-001", "type": "SETTINGS",
                             "agent_id": AGENT_ID, "data": {"log_level": "info"}}],
            })

        if "/acks" in path:
            if key != ACCESS_KEY:
                return self._json(401, {"statusCode": 401, "error": "ErrInvalidAPIKey"})
            state["acks"] += 1
            return self._json(200, {"action": "acks", "items": [{"status": 200, "message": "OK"}]})

        if path == "/_bulk":
            if key != OUTPUT_KEY:
                return self._json(401, {"error": "security_exception"})
            state["bulk"] += 1
            lines = raw.decode().splitlines()
            assert len(lines) == 2 and json.loads(lines[0])["create"]["_index"] == "logs-example-default"
            return self._json(200, {"took": 3, "errors": False, "items": [
                {"create": {"status": 201, "result": "created"}}]})

        return self._json(404, {"error": "not found"})


def call(base, path, method="POST", body=None, headers=None):
    data = json.dumps(body).encode() if body is not None else b""
    req = urllib.request.Request(base + path, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    for k, v in (headers or {}).items():
        req.add_header(k, v)
    try:
        with urllib.request.urlopen(req, timeout=5) as resp:
            return resp.status, json.loads(resp.read())
    except urllib.error.HTTPError as err:
        return err.code, json.loads(err.read())


def main():
    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    base = f"http://127.0.0.1:{server.server_port}"
    try:
        status, _ = call(base, "/api/fleet/agents/enroll", body={"type": "PERMANENT"})
        assert status == 401, "无认证头应返回 401"
        print("[1/6] 无认证头注册 -> 401 OK")

        status, data = call(base, "/api/fleet/enrollment_api_keys", body={},
                            headers={"kbn-xsrf": "1"})
        assert status == 200 and data["item"]["api_key"] == ENROLL_KEY
        print("[2/6] Kibana 签发注册令牌 OK")

        status, data = call(base, "/api/fleet/agents/enroll", body={"type": "PERMANENT"},
                            headers={"Authorization": f"ApiKey {ENROLL_KEY}",
                                     "User-Agent": "Elastic Agent v8.17.0"})
        assert status == 200 and data["item"]["access_api_key"] == ACCESS_KEY
        print("[3/6] 注册换得 agent id 与 access API key OK")

        status, _ = call(base, f"/api/fleet/agents/{AGENT_ID}/checkin", body={"status": "online"},
                         headers={"Authorization": "ApiKey wrong"})
        assert status == 401, "错误 access key 应返回 401"
        print("[4/6] 错误 access key 签入 -> 401 OK")

        status, data = call(base, f"/api/fleet/agents/{AGENT_ID}/checkin", body={"status": "online"},
                            headers={"Authorization": f"ApiKey {ACCESS_KEY}"})
        assert status == 200 and data["actions"][0]["type"] == "SETTINGS"
        print("[5/6] 正确 access key 签入下发动作 OK")

        status, _ = call(base, f"/api/fleet/agents/{AGENT_ID}/acks",
                         body={"events": []},
                         headers={"Authorization": f"ApiKey {ACCESS_KEY}"})
        assert status == 200
        bulk_body = ('{"create":{"_index":"logs-example-default","_id":"event-001"}}\n'
                     '{"@timestamp":"2026-01-01T00:00:00Z","message":"example"}\n').encode()
        req = urllib.request.Request(base + "/_bulk", data=bulk_body, method="POST")
        req.add_header("Authorization", f"ApiKey {OUTPUT_KEY}")
        req.add_header("Content-Type", "application/x-ndjson")
        with urllib.request.urlopen(req, timeout=5) as resp:
            assert resp.status == 200
        assert state["acks"] == 1 and state["bulk"] == 1
        print("[6/6] ACK 与 Bulk 使用各自凭据写入 OK")

        print("PASS: Elastic 认证过程 6/6 步通过")
    finally:
        server.shutdown()


if __name__ == "__main__":
    main()
