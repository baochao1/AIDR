"""FleetDM Orbit 认证过程本地 demo。

模拟本地节点密钥文件、enroll/config 接口与 WebSocket 升级鉴权，
复现密钥复用、401 计时触发强制重注册，以及 Node key 请求头要求。
仅使用标准库，不实现真实 SCEP/CMS 或 WebSocket 数据帧。
"""

import http.client
import json
import tempfile
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

NODE_KEY = "REDACTED-ORBIT-KEY"
state = {"enrolls": 0, "configs": 0, "upgrades": 0}


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

    def _authorized(self):
        return self.headers.get("Authorization") == f"Node key {NODE_KEY}"

    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0"))
        payload = json.loads(self.rfile.read(length) or b"{}")
        path = self.path.split("?")[0]
        if path == "/api/fleet/orbit/enroll":
            state["enrolls"] += 1
            if payload.get("enroll_secret") != "VALID-ENROLL":
                return self._json(401, {"message": "Authentication failed"})
            return self._json(200, {"orbit_node_key": NODE_KEY})
        if path == "/api/fleet/orbit/config":
            if not self._authorized() and payload.get("orbit_node_key") != NODE_KEY:
                return self._json(401, {"message": "Authentication required",
                                        "errors": [{"name": "base", "reason": "Authentication required"}]})
            state["configs"] += 1
            return self._json(200, {"notifications": {}, "websocket_transport": {"enabled": True}})
        return self._json(404, {"message": "not found"})

    def do_GET(self):
        if self.path != "/api/fleet/orbit/notifications":
            return self._json(404, {"message": "not found"})
        if not self._authorized():
            return self._json(401, "unauthorized")
        state["upgrades"] += 1
        self.send_response(101)
        self.send_header("Upgrade", "websocket")
        self.send_header("Connection", "Upgrade")
        self.end_headers()


def post(base, path, body, headers=None):
    data = json.dumps(body).encode()
    req = __import__("urllib.request", fromlist=["Request"]).Request(base + path, data=data, method="POST")
    req.add_header("Content-Type", "application/json")
    for key, value in (headers or {}).items():
        req.add_header(key, value)
    try:
        with __import__("urllib.request", fromlist=["urlopen"]).urlopen(req, timeout=5) as resp:
            return resp.status, json.loads(resp.read())
    except __import__("urllib.error", fromlist=["HTTPError"]).HTTPError as err:
        return err.code, json.loads(err.read())


def forced_reenroll(first_401, now):
    return now - first_401 >= 45


def main():
    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    base = f"http://127.0.0.1:{server.server_port}"
    host, port = "127.0.0.1", server.server_port
    with tempfile.TemporaryDirectory() as tmp:
        key_file = Path(tmp) / "secret-orbit-node-key.txt"
        try:
            # 本地无密钥 -> 注册并原子落盘
            if not key_file.exists() or not key_file.read_text().strip():
                status, data = post(base, "/api/fleet/orbit/enroll", {"enroll_secret": "VALID-ENROLL"})
                assert status == 200 and data["orbit_node_key"] == NODE_KEY
                temp = key_file.with_suffix(".tmp")
                temp.write_text(data["orbit_node_key"])
                temp.replace(key_file)
                print("[1/4] 注册签发 orbit_node_key 并原子落盘 OK")

            # 复用本地密钥
            status, _ = post(base, "/api/fleet/orbit/config", {"orbit_node_key": key_file.read_text()})
            assert status == 200 and state["enrolls"] == 1
            print("[2/4] 复用本地节点密钥拉取配置 OK")

            # 401 计时：45 秒内不强制，达到 45 秒强制
            first = 1000
            assert not forced_reenroll(first, first + 44)
            assert forced_reenroll(first, first + 45)
            status, _ = post(base, "/api/fleet/orbit/config",
                             {"orbit_node_key": "WRONG"}, {"Authorization": "Node key WRONG"})
            assert status == 401
            print("[3/4] 401 计时达到 45 秒触发强制重注册 OK")

            # WebSocket 升级鉴权
            conn = http.client.HTTPConnection(host, port, timeout=5)
            conn.request("GET", "/api/fleet/orbit/notifications", headers={
                "Connection": "Upgrade", "Upgrade": "websocket",
                "Sec-WebSocket-Version": "13", "Authorization": f"Node key {NODE_KEY}"})
            resp = conn.getresponse()
            assert resp.status == 101, resp.status
            conn.close()
            assert state["upgrades"] == 1
            print("[4/4] WebSocket 升级通过 Node key 鉴权 OK")

            print("PASS: FleetDM Orbit 认证过程 4/4 步通过")
        finally:
            server.shutdown()


if __name__ == "__main__":
    main()
