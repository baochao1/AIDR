"""Exercise and display the real management server and Agent enrollment flow."""
import argparse
import http.client
import json
from pathlib import Path
import socket
import ssl
import subprocess
import tempfile
import time

parser = argparse.ArgumentParser()
parser.add_argument("binary", type=Path)
parser.add_argument("--present", action="store_true", help="pause after each demonstrated stage")
args = parser.parse_args()
binary = args.binary.resolve()


def stage(message):
    print(message, flush=True)
    if args.present:
        input("Press Enter to continue...")


def wait_for(probe):
    deadline = time.monotonic() + 20
    while time.monotonic() < deadline:
        result = probe()
        if result:
            return result
        time.sleep(0.2)
    raise RuntimeError("Timed out waiting for the expected demo state")


with tempfile.TemporaryDirectory(prefix="aidr-enrollment-") as temp:
    root = Path(temp) / "demo"
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        port = listener.getsockname()[1]

    def command(role):
        return [str(binary), role, "-root", str(root),
                "-listen", f"127.0.0.1:{port}", "-url", f"https://127.0.0.1:{port}"]

    subprocess.run(command("bootstrap"), check=True, timeout=10)

    def request(role, method, path, body=None):
        context = ssl.create_default_context(cafile=str(root / "server" / "ca.pem"))
        if role:
            identity = root / role
            context.load_cert_chain(str(identity / f"{role}.pem"), str(identity / f"{role}.key"))
        connection = http.client.HTTPSConnection("127.0.0.1", port, context=context, timeout=3)
        try:
            connection.request(method, path, json.dumps(body) if body is not None else None,
                               {"Content-Type": "application/json"})
            response = connection.getresponse()
            return response.status, response.read()
        finally:
            connection.close()

    def ready(server):
        if server.poll() is not None:
            raise RuntimeError("Management server exited")
        try:
            with socket.create_connection(("127.0.0.1", port), timeout=1):
                return True
        except ConnectionRefusedError:
            return False

    processes = []
    try:
        def start_server():
            server = subprocess.Popen(command("server"))
            processes.append(server)
            wait_for(lambda: ready(server))
            return server

        server = start_server()
        status_path = "/v1/admin/agents/demo-01"
        try:
            request(None, "GET", status_path)
        except (ssl.SSLError, ConnectionResetError):
            stage("PASS 1/5  Management rejects TLS clients without a certificate")
        else:
            raise AssertionError("Server accepted a connection without an mTLS client certificate")

        assert request("agent", "POST", "/v1/agents/checkin", {})[0] == 403
        status, body = request("agent", "POST", "/v1/agents/enroll", "invalid-demo-token")
        assert status == 400 and b"invalid enrollment token" in body, (status, body)
        stage("PASS 2/5  Unregistered check-in and invalid enrollment token both rejected")

        log_path = Path(temp) / "agent.log"
        with log_path.open("wb") as output:
            agent = subprocess.Popen(command("agent"), stderr=output)
        processes.append(agent)

        def report():
            if agent.poll() is not None:
                raise RuntimeError(log_path.read_text(encoding="utf-8", errors="replace"))
            status, body = request("admin", "GET", status_path)
            return json.loads(body) if status == 200 else None

        initial = wait_for(report)
        log = wait_for(lambda: (content if "management check-in accepted" in
                                (content := log_path.read_text(encoding="utf-8", errors="replace")) else None))
        assert "management enrollment accepted" in log, log
        stage(f"PASS 3/5  Agent demo-01 registered; first check-in stored at {initial['checkin_at']}")
        print(log.rstrip(), flush=True)

        assert request("agent", "GET", status_path)[0] == 403
        stage("PASS 4/5  Agent certificate cannot read the administrator endpoint")

        server.terminate()
        server.wait(timeout=5)
        wait_for(lambda: "/v1/agents/checkin" in log_path.read_text(encoding="utf-8", errors="replace"))
        server = start_server()

        def reconnected():
            current = report()
            return current if current and current["checkin_at"] != initial["checkin_at"] else None

        current = wait_for(reconnected)
        wait_for(lambda: log_path.read_text(encoding="utf-8", errors="replace")
                 .count("management check-in accepted") >= 2)
        stage(f"PASS 5/5  Management restarted; Agent resumed check-ins at {current['checkin_at']}")
        print(json.dumps(current, indent=2), flush=True)
    finally:
        for process in reversed(processes):
            if process.poll() is None:
                process.terminate()
                process.wait(timeout=5)
