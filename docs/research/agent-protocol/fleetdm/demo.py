"""FleetDM osquery 认证过程本地 demo。

用本地 mock Fleet Server 复现 nodeKey 复用、注册换取 node_key、
节点凭据请求，以及 node_invalid 触发的重新注册。仅使用标准库。
"""

import json
import threading
import urllib.error
import urllib.parse
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

NODE_KEY = "REDACTED-NODE-KEY"
state = {"enrolls": 0, "configs": 0}


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
        payload = json.loads(raw or b"{}")
        path = self.path.split("?")[0]

        if path == "/api/osquery/enroll":
            state["enrolls"] += 1
            if payload.get("enroll_secret") != "VALID-ENROLL":
                return self._json(401, {"error": "enroll failed", "uuid": "demo",
                                        "node_invalid": True})
            return self._json(200, {"node_key": NODE_KEY})

        key = payload.get("node_key")
        strict = self.headers.get("Authorization")
        if strict:
            if strict != f"NodeKey {NODE_KEY}":
                return self._json(401, {"error": "authentication error: invalid authorization header",
                                        "uuid": "demo", "node_invalid": True})
            key = strict.removeprefix("NodeKey ")
        if key != NODE_KEY:
            return self._json(401, {"error": "authentication error: invalid node key",
                                    "uuid": "demo", "node_invalid": True})
        if path == "/api/osquery/config":
            state["configs"] += 1
            return self._json(200, {"options": {}, "schedule": {}, "etag": "abc"})
        if path == "/api/osquery/distributed/read":
            return self._json(200, {"queries": {"42": "SELECT 1;"}, "discovery": {}})
        return self._json(404, {"error": "not found"})


def call(base, path, body, headers=None):
    data = json.dumps(body).encode()
    req = urllib.request.Request(base + path, data=data, method="POST")
    req.add_header("Content-Type", "application/json")
    for key, value in (headers or {}).items():
        req.add_header(key, value)
    try:
        with urllib.request.urlopen(req, timeout=5) as resp:
            return resp.status, json.loads(resp.read())
    except urllib.error.HTTPError as err:
        return err.code, json.loads(err.read())


def main():
    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    base = f"http://127.0.0.1:{server.server_port}"
    local_node_key = None
    try:
        # osquery: 本地无 nodeKey 时注册
        if not local_node_key:
            status, data = call(base, "/api/osquery/enroll", {"enroll_secret": "VALID-ENROLL",
                                                              "host_identifier": "host-example"})
            assert status == 200 and data["node_key"] == NODE_KEY
            local_node_key = data["node_key"]
            print("[1/4] 首次注册换得 node_key 并写入本地存储 OK")

        # 再次启动：复用本地 nodeKey
        status, data = call(base, "/api/osquery/config", {"node_key": local_node_key})
        assert status == 200 and state["enrolls"] == 1
        print("[2/4] 复用本地 nodeKey 拉取配置，未重复注册 OK")

        # 严格请求头认证
        status, data = call(base, "/api/osquery/distributed/read", {},
                            {"Authorization": f"NodeKey {local_node_key}"})
        assert status == 200 and "42" in data["queries"]
        print("[3/4] NodeKey 请求头认证下发查询 OK")

        # 错误密钥 -> node_invalid -> 重新注册
        status, data = call(base, "/api/osquery/config", {"node_key": "WRONG"})
        assert status == 401 and data.get("node_invalid") is True
        local_node_key = None
        if not local_node_key:
            status, data = call(base, "/api/osquery/enroll", {"enroll_secret": "VALID-ENROLL"})
            assert status == 200
            local_node_key = data["node_key"]
        assert state["enrolls"] == 2
        print("[4/4] 无效节点凭据触发本地清理与重新注册 OK")

        print("PASS: FleetDM osquery 认证过程 4/4 步通过")
    finally:
        server.shutdown()


if __name__ == "__main__":
    main()
