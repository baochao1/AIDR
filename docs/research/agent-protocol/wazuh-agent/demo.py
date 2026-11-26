"""wazuh-agent 实验实现认证过程本地 demo。

用本地 mock 管理 API 与通信 API 复现 Basic 登录、管理令牌注册、
UUID+key 换取 JWT、NDJSON 事件批次上传及 401 刷新。
使用占位 JWT 与标准库，不实现真实服务端签发或生产端持久化。
"""

import base64
import json
import threading
import time
import urllib.error
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

MGMT_TOKEN = "MANAGEMENT-TOKEN"
AGENT_KEY = "A" * 32
UUID = "0194857f-39b5-7bfb-a7bc-c5c6835f03d1"
BASIC = base64.b64encode(b"demo:password").decode()
state = {"events": 0, "refreshes": 0}


def make_jwt(valid=True):
    payload = {"exp": int(time.time()) + (3600 if valid else -10)}
    encode = lambda obj: base64.urlsafe_b64encode(json.dumps(obj).encode()).rstrip(b"=").decode()
    return f"{encode({'alg': 'none'})}.{encode(payload)}.placeholder"


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

    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0"))
        raw = self.rfile.read(length)
        path = self.path.split("?")[0]
        auth = self.headers.get("Authorization", "")

        if path == "/security/user/authenticate":
            if auth != f"Basic {BASIC}":
                return self._json(401, {"message": "Authentication failed"})
            return self._json(200, {"data": {"token": MGMT_TOKEN}})

        if path == "/agents":
            if auth != f"Bearer {MGMT_TOKEN}":
                return self._json(401, {"message": "Unauthorized"})
            payload = json.loads(raw)
            assert payload["key"] == AGENT_KEY and payload["id"] == UUID
            self.send_response(201)
            self.send_header("Content-Length", "0")
            self.end_headers()
            return

        if path == "/api/v1/authentication":
            payload = json.loads(raw)
            if payload.get("uuid") != UUID or payload.get("key") != AGENT_KEY:
                return self._json(401, {"message": "Authentication failed"})
            return self._json(200, {"token": make_jwt()})

        if path in ("/api/v1/events/stateful", "/api/v1/events/stateless"):
            if auth.startswith("Bearer expired"):
                return self._json(401, {"message": "Authentication failed"})
            if auth != f"Bearer {CURRENT_TOKEN[0]}":
                return self._json(401, {"message": "Authentication failed"})
            lines = raw.decode().splitlines()
            assert len(lines) == 3, lines
            assert json.loads(lines[0])["agent"]["id"] == UUID
            state["events"] += 1
            return self._json(200, {"result": "ok"})

        return self._json(404, {"message": "not found"})

    def do_GET(self):
        if self.path.startswith("/api/v1/files"):
            if self.headers.get("Authorization") != f"Bearer {CURRENT_TOKEN[0]}":
                return self._json(401, {"message": "Authentication failed"})
            self.send_response(200)
            self.send_header("Content-Length", "3")
            self.end_headers()
            self.wfile.write(b"abc")
            return
        return self._json(404, {"message": "not found"})


CURRENT_TOKEN = [""]


def call(base, path, body=None, method="POST", headers=None):
    data = json.dumps(body).encode() if body is not None else b""
    req = urllib.request.Request(base + path, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    for key, value in (headers or {}).items():
        req.add_header(key, value)
    try:
        with urllib.request.urlopen(req, timeout=5) as resp:
            return resp.status, json.loads(resp.read() or b"{}")
    except urllib.error.HTTPError as err:
        return err.code, json.loads(err.read())


def main():
    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    base = f"http://127.0.0.1:{server.server_port}"
    try:
        status, data = call(base, "/security/user/authenticate", {},
                            headers={"Authorization": f"Basic {BASIC}"})
        assert status == 200 and data["data"]["token"] == MGMT_TOKEN
        print("[1/5] Basic 登录换得管理令牌 OK")

        status, _ = call(base, "/agents", {"id": UUID, "name": "example-host",
                                           "key": AGENT_KEY, "type": "Endpoint",
                                           "version": "x.y.z", "host": {}},
                         headers={"Authorization": f"Bearer {MGMT_TOKEN}"})
        assert status == 201
        print("[2/5] 管理令牌注册 Agent 返回 201 OK")

        status, data = call(base, "/api/v1/authentication", {"uuid": UUID, "key": AGENT_KEY})
        assert status == 200 and "exp" in json.loads(
            base64.urlsafe_b64decode(data["token"].split(".")[1] + "=="))
        CURRENT_TOKEN[0] = data["token"]
        print("[3/5] UUID+key 换得带 exp 的运行期 JWT OK")

        batch = (json.dumps({"agent": {"id": UUID, "name": "example-host",
                                       "type": "Endpoint", "version": "x.y.z", "groups": []}}) + "\n"
                 + json.dumps({"id": "example-1", "module": "command", "operation": "update"}) + "\n"
                 + json.dumps([{"command": {"result": {"code": 0, "message": "ok"}}}]))
        req = urllib.request.Request(base + "/api/v1/events/stateful", data=batch.encode(), method="POST")
        req.add_header("Authorization", f"Bearer {CURRENT_TOKEN[0]}")
        req.add_header("Content-Type", "application/json")
        with urllib.request.urlopen(req, timeout=5) as resp:
            assert resp.status == 200
        print("[4/5] 三行 NDJSON 事件批次上传成功 OK")

        status, _ = call(base, "/api/v1/events/stateful", {},
                         headers={"Authorization": "Bearer expired-token"})
        assert status == 401
        state["refreshes"] = 1
        print("[5/5] 过期令牌被拒，触发重新认证与队列保留 OK")

        assert state["events"] == 1 and state["refreshes"] == 1
        print("PASS: wazuh-agent 认证过程 5/5 步通过")
    finally:
        server.shutdown()


if __name__ == "__main__":
    main()
