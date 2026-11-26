"""Wazuh 4.14.0 认证过程本地 demo。

用本地 TCP mock authd 复现 1515 注册文本交换、client.keys 落盘格式，
以及 1514 安全消息的帧封装与 MD5/计数器校验。
不建立真实 TLS，不实现 AES/Blowfish 加解密。
"""

import hashlib
import socket
import struct
import tempfile
import threading
from pathlib import Path

AGENT_ID = "001"
AGENT_NAME = "example-host"
SHARED_KEY = "DEMO-SHARED-KEY"


def serve(listener, stop):
    listener.settimeout(0.2)
    while not stop.is_set():
        try:
            conn, _ = listener.accept()
        except socket.timeout:
            continue
        with conn:
            data = conn.recv(4096)
            if data.startswith(b"OSSEC"):
                line = data.split(b"\n", 1)[0]
                if b"OSSEC PASS: wrong" in line:
                    conn.sendall(b"ERROR: Invalid password. Unable to add agent")
                else:
                    conn.sendall(
                        f"OSSEC K:'{AGENT_ID} {AGENT_NAME} any {SHARED_KEY}'".encode())
                continue
            if len(data) < 4:
                continue
            size = struct.unpack("<I", data[:4])[0]
            payload = data[4:4 + size]
            if not payload.startswith(b"#AES:"):
                conn.sendall(b"FAIL")
                continue
            rest = payload[5:]
            checksum = rest[:32].decode(errors="replace")
            counters_and_event = rest[32:]
            if hashlib.md5(counters_and_event).hexdigest() == checksum:
                conn.sendall(b"ACK")
            else:
                conn.sendall(b"FAIL")


def enroll(port, name=AGENT_NAME, password=None):
    with socket.create_connection(("127.0.0.1", port), timeout=5) as sock:
        text = (f"OSSEC PASS: {password}\n" if password else "") + f"OSSEC A:'{name}'\n"
        sock.sendall(text.encode())
        return sock.recv(4096).decode()


def security_frame(event):
    counters = "12345" + "0000000001" + ":" + "0001" + ":"
    body = counters + event
    checksum = hashlib.md5(body.encode()).hexdigest()
    payload = b"#AES:" + checksum.encode() + body.encode()
    return struct.pack("<I", len(payload)) + payload


def send_frame(port, frame):
    with socket.create_connection(("127.0.0.1", port), timeout=5) as sock:
        sock.sendall(frame)
        return sock.recv(4096).decode()


def main():
    listener = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    listener.bind(("127.0.0.1", 0))
    listener.listen(8)
    stop = threading.Event()
    threading.Thread(target=serve, args=(listener, stop), daemon=True).start()
    port = listener.getsockname()[1]
    try:
        response = enroll(port)
        assert response.startswith("OSSEC K:'") and response.endswith("'"), response
        inner = response[len("OSSEC K:'"):-1]
        fields = inner.split(" ")
        assert len(fields) == 4 and fields[0] == AGENT_ID and fields[3] == SHARED_KEY
        print("[1/4] 注册换得 OSSEC K: 四项响应 OK")

        error = enroll(port, password="wrong")
        assert error.startswith("ERROR: ") and not error.endswith("\n")
        print("[2/4] 错误注册密码返回 ERROR 文本且无 LF OK")

        with tempfile.TemporaryDirectory() as tmp:
            keys = Path(tmp) / "client.keys"
            keys.write_text(" ".join(fields) + "\n")
            assert keys.read_text().strip() == inner
            print("[3/4] 四项写入 client.keys 格式 OK")

        ok = send_frame(port, security_frame("1:/var/log/example.log:application status=healthy"))
        assert ok == "ACK", ok
        bad = send_frame(port, struct.pack("<I", 5) + b"#AES:" + b"x" * 40)
        assert bad == "FAIL", bad
        print("[4/4] 安全消息帧通过 MD5 校验，错误校验被拒 OK")

        print("PASS: Wazuh 认证过程 4/4 步通过")
    finally:
        stop.set()
        listener.close()


if __name__ == "__main__":
    main()
