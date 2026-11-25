"""Run the real HTTPS/SQLite lifecycle with a temporary management and agent root."""
from contextlib import closing
import json
import os
from pathlib import Path
import signal
import socket
import sqlite3
import subprocess
import sys
import tempfile
import time

binary, mock_go, mock_python = [Path(arg).resolve() for arg in sys.argv[1:]]
with socket.socket() as listener:
    listener.bind(("127.0.0.1", 0))
    port = listener.getsockname()[1]

with tempfile.TemporaryDirectory(prefix="supervisor-e2e-") as temp:
    root = Path(temp) / "demo"
    base = [str(binary)]
    url = f"https://127.0.0.1:{port}"
    listen = f"127.0.0.1:{port}"

    def arguments(command, *args):
        return base + [command, "-root", str(root), "-url", url, "-listen", listen, *map(str, args)]

    def run(command, *args, ok=True):
        result = subprocess.run(arguments(command, *args), capture_output=True, text=True, timeout=25)
        assert (result.returncode == 0) == ok, (command, result.stdout, result.stderr)
        return result.stdout

    def wait(predicate):
        last = None
        for _ in range(160):
            assert supervisor.poll() is None, "Supervisor exited"
            result = subprocess.run(arguments("status"), capture_output=True, text=True, timeout=5)
            if result.returncode == 0:
                last = json.loads(result.stdout)
                if predicate(last):
                    return last
            time.sleep(0.2)
        raise AssertionError(last)

    def policy(revision, *components, ok=True):
        path = Path(temp) / "policy.json"
        path.write_text(json.dumps({"revision": revision, "components": components}), encoding="utf-8")
        run("policy", "-file", path, ok=ok)

    def desired(identifier, version="", state="running"):
        return {"id": identifier, "version": version, "desired_state": state,
                "artifact_id": f"{identifier}-{version}"}

    def component(report, identifier="mock-a"):
        return (report.get("components") or {}).get(identifier, {})

    def query(role, statement):
        with closing(sqlite3.connect(root / role / "state.db", timeout=2)) as db:
            return db.execute(statement).fetchall()

    def assert_no_tcp(pid):
        if os.name == "nt":
            rows = subprocess.check_output(["netstat", "-ano"], text=True, errors="replace").splitlines()
            assert not any(len(parts) >= 5 and parts[0] == "TCP" and parts[-1] == str(pid)
                           for parts in map(str.split, rows)), (pid, rows)
        elif sys.platform.startswith("linux"):
            proc = Path("/proc") / str(pid)
            sockets = {os.readlink(fd)[8:-1] for fd in (proc / "fd").iterdir()
                       if os.readlink(fd).startswith("socket:[")}
            tcp = {line.split()[9] for name in ("tcp", "tcp6")
                   for line in (proc / "net" / name).read_text().splitlines()[1:]}
            assert sockets.isdisjoint(tcp), (pid, sockets & tcp)
        else:
            result = subprocess.run(["lsof", "-a", "-p", str(pid), "-iTCP", "-Fp"], capture_output=True)
            assert result.returncode == 1 and not result.stdout and not result.stderr, result

    run("bootstrap")
    server = subprocess.Popen(arguments("server"))
    supervisor_log = Path(temp) / "supervisor.log"
    log_output = supervisor_log.open("ab")
    supervisor = subprocess.Popen(arguments("agent", "-events"), stderr=log_output)
    try:
        wait(lambda report: not report["components"])
        print("T01 blank agent registered", flush=True)
        for identifier, version, executable in [
            ("mock-a", "1.0.0", mock_go), ("mock-a", "2.0.0", mock_go),
            ("mock-a", "3.0.0", mock_go), ("mock-c", "1.0.0", mock_go),
            ("mock-b", "1.0.0", mock_python),
        ]:
            artifact = Path(temp) / f"{identifier}-{version}.json"
            run("sign", "-id", identifier, "-version", version, "-artifact",
                f"{identifier}-{version}", "-ipc", "tcp" if identifier == "mock-b" else "native",
                "-file", executable, "-out", artifact)
            run("publish", "-file", artifact)
        tampered = json.loads(artifact.read_text())
        signature = tampered["signature"]
        tampered["signature"] = ("A" if signature[0] != "A" else "B") + signature[1:]
        artifact.write_text(json.dumps(tampered), encoding="utf-8")
        run("publish", "-file", artifact, ok=False)
        # Supply the agent identity under the filenames expected by the CLI client.
        agent_dir = root / "agent"
        denied_dir = Path(temp) / "deny"
        denied_dir.mkdir()
        for source, dest in (("ca.pem", "ca.pem"), ("agent.pem", "admin.pem"), ("agent.key", "admin.key")):
            (denied_dir / dest).write_bytes((agent_dir / source).read_bytes())
        denial = subprocess.run(arguments("status", "-identity", denied_dir), capture_output=True)
        assert denial.returncode != 0 and b"403" in denial.stderr, denial.stderr
        print("T11 invalid signature and unauthorized admin access rejected", flush=True)

        b = desired("mock-b", "1.0.0")
        a1 = desired("mock-a", "1.0.0")
        a2 = desired("mock-a", "2.0.0")
        c = desired("mock-c", "1.0.0")
        policy(1, a1, b, c)
        report = wait(lambda r: r["applied_revision"] == 1)
        assert component(report)["state"] == "running" and component(report, "mock-b")["state"] == "running"
        b_pid = component(report, "mock-b")["pid"]
        a_pid = component(report)["pid"]
        c_pid = component(report, "mock-c")["pid"]
        for identifier in ("mock-a", "mock-c"):
            assert component(report, identifier)["ipc_transport"] == ("npipe" if os.name == "nt" else "unix")
        assert component(report, "mock-b")["ipc_transport"] == "tcp"
        assert a_pid != c_pid
        assert_no_tcp(a_pid)
        assert_no_tcp(c_pid)
        deadline = time.monotonic() + 10
        while not {"mock-a", "mock-c"}.issubset(
            {json.loads(row[0])["component_id"] for row in query("server", "SELECT body FROM event_receipts")}
        ):
            assert time.monotonic() < deadline, "both native streams must deliver events"
            time.sleep(0.2)
        wait(lambda r: component(r).get("data", {}).get("state") == "receiving"
             and component(r, "mock-b").get("data", {}).get("state") == "unsupported")
        print("Native IPC: two concurrent Go components delivered events with zero TCP sockets", flush=True)
        policy(1, a1, b, c)
        policy(1, a2, b, ok=False)
        assert component(wait(lambda r: r["applied_revision"] == 1))["pid"] == a_pid
        print("T02/T03/T07/T12 Go and Python dynamically installed, same revision idempotent", flush=True)

        policy(2, a2, b, desired("mock-c", state="absent"))
        report = wait(lambda r: r["applied_revision"] == 2)
        assert component(report)["version"] == "2.0.0"
        assert component(report, "mock-b")["pid"] == b_pid
        policy(1, a1, b, ok=False)
        print("T08 successful upgrade and stale revision rejected", flush=True)

        policy(3, desired("mock-a", "3.0.0"), b)
        report = wait(lambda r: r["received_revision"] == 3 and component(r).get("error"))
        assert component(report)["version"] == "2.0.0"
        assert component(report)["state"] == "running"
        assert component(report, "mock-b")["pid"] == b_pid
        assert report["applied_revision"] != 3
        print("T09 failed upgrade rolled back, Python PID unchanged", flush=True)

        policy(4, desired("mock-a", "2.0.0", "stopped"), b)
        wait(lambda r: r["applied_revision"] == 4 and component(r)["state"] == "stopped")
        time.sleep(2)
        wait(lambda r: r["applied_revision"] == 4 and component(r)["state"] == "stopped")
        policy(5, a2, b)
        report = wait(lambda r: r["applied_revision"] == 5)
        os.kill(component(report)["pid"], signal.SIGTERM)
        report = wait(lambda r: r["applied_revision"] == 5 and component(r).get("state") == "running"
                      and component(r)["pid"] != component(report)["pid"])
        print("T05 forced component exit recovered", flush=True)
        server.terminate()
        server.wait(timeout=10)
        time.sleep(1.5)
        server = subprocess.Popen(arguments("server"))
        wait(lambda r: r["applied_revision"] == 5
             and r["checkin_at"] > report["checkin_at"]
             and component(r)["pid"] == component(report)["pid"])
        print("T06 management restart and agent reconnection", flush=True)

        policy(6, desired("mock-a", state="absent"), desired("mock-b", state="absent"))
        wait(lambda r: r["applied_revision"] == 6 and not r["components"])
        assert not (root / "agent" / "packages" / "mock-a").exists()
        assert not (root / "agent" / "packages" / "mock-b").exists()
        assert (root / "agent" / "data" / "mock-a").is_dir()
        print(f"T04 stopped, restarted and uninstalled with Supervisor PID {supervisor.pid}", flush=True)

        policy(7, a2, b)
        wait(lambda r: r["applied_revision"] == 7)
        offset = supervisor_log.stat().st_size
        policy(8, desired("mock-a", "3.0.0"), b)
        transition = b"transition 2.0.0/running -> 3.0.0/running"
        deadline = time.monotonic() + 12
        while transition not in supervisor_log.read_bytes()[offset:]:
            assert time.monotonic() < deadline, "upgrade never entered candidate start"
            time.sleep(0.03)
        supervisor.kill()
        supervisor.wait(timeout=10)
        supervisor = subprocess.Popen(arguments("agent", "-events"), stderr=log_output)
        report = wait(lambda r: r["received_revision"] == 8
                      and component(r).get("error") and component(r).get("state") == "running")
        assert component(report)["version"] == "2.0.0"
        print("T10 killed Supervisor after stopping old instance; restart recovered v2", flush=True)
        server.terminate()
        server.wait(timeout=10)
        deadline = time.monotonic() + 8
        pending = []
        while not pending:
            pending = query("agent", "SELECT body FROM queue LIMIT 1")
            assert time.monotonic() < deadline, "no durable offline event"
            time.sleep(0.15)
        event_id = json.loads(pending[0][0])["event_id"]
        supervisor.kill()
        supervisor.wait(timeout=10)
        assert query("agent", "SELECT body FROM queue"), "queue lost after agent crash"
        supervisor = subprocess.Popen(arguments("agent", "-events"), stderr=log_output)
        server = subprocess.Popen(arguments("server"))
        deadline = time.monotonic() + 12
        while event_id not in {row[0] for row in query("server", "SELECT event_id FROM event_receipts")}:
            assert time.monotonic() < deadline, "persisted event never reached management"
            time.sleep(0.2)
        print("T14 durable event collected offline and uploaded after Supervisor crash", flush=True)
        policy(9, desired("mock-a", state="absent"), desired("mock-b", state="absent"))
        wait(lambda r: r["applied_revision"] == 9 and not r["components"])
    finally:
        for process in (supervisor, server):
            if process.poll() is None:
                process.terminate()
                process.wait(timeout=10)
        log_output.close()
