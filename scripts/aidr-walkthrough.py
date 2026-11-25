"""A screenshot-ready walkthrough. Runs isolated management and agent processes."""
from contextlib import closing
import argparse
import json
from pathlib import Path
import socket
import sqlite3
import subprocess
import tempfile
import time

parser = argparse.ArgumentParser()
parser.add_argument("--pause", action="store_true", help="Wait after each screenshot step")
args = parser.parse_args()
workspace = Path(__file__).resolve().parents[1]
binary = workspace / "build" / "supervisor-demo.exe"
adapter = workspace / "build" / "mock-go.exe"

def show(title, value):
    print(f"\n=== {title} ===", flush=True)
    print(json.dumps(value, ensure_ascii=False, indent=2), flush=True)
    if args.pause:
        input("截图后按 Enter 继续… ")

with socket.socket() as listener:
    listener.bind(("127.0.0.1", 0))
    address = f"127.0.0.1:{listener.getsockname()[1]}"

with tempfile.TemporaryDirectory(prefix="aidr-walkthrough-") as temporary:
    root = Path(temporary) / "demo"

    def command(action, *options):
        return [str(binary), action, "-root", str(root), "-url", f"https://{address}",
                "-listen", address, *map(str, options)]

    def run(action, *options):
        return subprocess.check_output(command(action, *options), text=True, timeout=25)

    def wait(check):
        deadline = time.monotonic() + 25
        while time.monotonic() < deadline:
            try:
                value = check()
                if value is not None:
                    return value
            except (subprocess.CalledProcessError, sqlite3.OperationalError):
                pass
            time.sleep(0.25)
        raise TimeoutError("未在 25 秒内取得期望状态，请检查进程日志")

    def status():
        result = subprocess.run(command("status"), capture_output=True, text=True, timeout=5)
        return json.loads(result.stdout) if result.returncode == 0 else {}

    def policy(revision, state):
        request = {"revision": revision, "components": [{
            "id": "aidr-mock", "version": "1.0.0",
            "artifact_id": "aidr-mock-1", "desired_state": state,
        }]}
        path = Path(temporary) / "policy.json"
        path.write_text(json.dumps(request), encoding="utf-8")
        run("policy", "-file", path)
        return request

    def applied(revision):
        report = status()
        return report if report.get("applied_revision") == revision else None

    def event():
        db = root / "server" / "state.db"
        with closing(sqlite3.connect(db, timeout=2)) as connection:
            row = connection.execute(
                "SELECT agent, body FROM event_receipts WHERE agent = ? ORDER BY rowid LIMIT 1",
                ("demo-01",),
            ).fetchone()
        if row:
            value = json.loads(row[1])
            if value["component_id"] == "aidr-mock":
                return {"终端": row[0], "证据来源": "管理端 SQLite event_receipts", "事件": value}

    def listening():
        try:
            host, port = address.rsplit(":", 1)
            with socket.create_connection((host, int(port)), timeout=0.3):
                return True
        except OSError:
            return None

    run("bootstrap")
    processes = []
    try:
        processes.append(subprocess.Popen(command("server")))
        wait(listening)
        processes.append(subprocess.Popen(command("agent", "-events")))
        pid = processes[-1].pid
        first = wait(lambda: status() or None)
        assert not first["components"]
        show("01 空白终端已注册", {"Supervisor PID": pid, "实际状态": first})
        package = Path(temporary) / "aidr-mock.json"
        run("sign", "-id", "aidr-mock", "-version", "1.0.0", "-artifact",
            "aidr-mock-1", "-ipc", "native", "-file", adapter, "-out", package)
        run("publish", "-file", package)
        requested = policy(1, "running")
        report = wait(lambda: applied(1))
        assert report["components"]["aidr-mock"]["state"] == "running"
        show("02 签名制品发布、远程安装并启动", {
            "Supervisor PID": pid, "策略": requested, "实际状态": report,
        })
        show("03 AIDR Mock 实时事件已到达管理端", wait(event))
        policy(2, "stopped")
        stopped = wait(lambda: applied(2))
        assert stopped["components"]["aidr-mock"]["state"] == "stopped"
        show("04 远程停止", {"Supervisor PID": pid, "实际状态": stopped})
        policy(3, "absent")
        final = wait(lambda: applied(3))
        removed = not (root / "agent" / "packages" / "aidr-mock").exists()
        assert not final["components"] and removed and processes[-1].poll() is None
        show("05 远程卸载完成", {"Supervisor PID": pid, "制品已移除": removed, "实际状态": final})
    finally:
        for process in reversed(processes):
            if process.poll() is None:
                process.terminate()
                process.wait(timeout=10)

