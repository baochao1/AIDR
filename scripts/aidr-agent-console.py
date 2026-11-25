import argparse
from contextlib import closing
import json
from pathlib import Path
import socket
import sqlite3
import subprocess
import tempfile
from threading import Thread
import time
import tkinter as tk
from tkinter import ttk

WORKSPACE = Path(__file__).resolve().parents[1]
BINARY = WORKSPACE / "build" / "supervisor-demo.exe"
MOCK = WORKSPACE / "build" / "mock-go.exe"
COMPONENT = "aidr-mock"


class Backend:
    def __init__(self):
        self.temp = tempfile.TemporaryDirectory(prefix="aidr-console-")
        self.root = Path(self.temp.name) / "demo"
        with socket.socket() as listener:
            listener.bind(("127.0.0.1", 0))
            self.address = f"127.0.0.1:{listener.getsockname()[1]}"
        self.processes = []
        self.revision = 0
        self.log = (Path(self.temp.name) / "runtime.log").open("w", encoding="utf-8")

    def command(self, action, *options):
        return [str(BINARY), action, "-root", str(self.root), "-listen", self.address,
                "-url", f"https://{self.address}", *map(str, options)]

    def call(self, action, *options):
        result = subprocess.run(self.command(action, *options), capture_output=True, text=True,
                                encoding="utf-8", errors="replace", timeout=25,
                                creationflags=subprocess.CREATE_NO_WINDOW)
        if result.returncode:
            raise RuntimeError(result.stderr.strip() or f"{action} failed")
        return result.stdout

    def start(self):
        self.call("bootstrap")
        for command in [self.command("server"), self.command("agent", "-events")]:
            process = subprocess.Popen(command, stdout=self.log, stderr=self.log,
                                       creationflags=subprocess.CREATE_NO_WINDOW)
            self.processes.append(process)
        self.wait(lambda: self.status())

    def status(self):
        try:
            return json.loads(self.call("status"))
        except RuntimeError:
            if any(process.poll() is not None for process in self.processes):
                raise RuntimeError(Path(self.log.name).read_text(encoding="utf-8", errors="replace"))
            return None

    @staticmethod
    def wait(condition):
        deadline = time.monotonic() + 25
        while time.monotonic() < deadline:
            result = condition()
            if result:
                return result
            time.sleep(0.25)
        raise TimeoutError("等待状态确认超时")

    def apply(self, state, publish=False):
        if publish:
            artifact = Path(self.temp.name) / "aidr-mock.json"
            self.call("sign", "-id", COMPONENT, "-version", "1.0.0",
                      "-artifact", "aidr-mock-1", "-ipc", "native", "-file", MOCK, "-out", artifact)
            self.call("publish", "-file", artifact)
        self.revision += 1
        revision = self.revision
        path = Path(self.temp.name) / "policy.json"
        path.write_text(json.dumps({"revision": revision, "components": [{
            "id": COMPONENT, "version": "1.0.0", "artifact_id": "aidr-mock-1",
            "desired_state": state,
        }]}), encoding="utf-8")
        self.call("policy", "-file", path)

        def confirmed():
            report = self.status() or {}
            component = (report.get("components") or {}).get(COMPONENT, {})
            if component.get("error"):
                raise RuntimeError(component["error"])
            return report if report.get("applied_revision") == revision else None

        return self.wait(confirmed)

    def query(self, role, statement):
        database = self.root / role / "state.db"
        if not database.is_file():
            return []
        with closing(sqlite3.connect(database.as_uri() + "?mode=ro", uri=True, timeout=1)) as db:
            return db.execute(statement).fetchall()

    def snapshot(self):
        stats = self.query("server", "SELECT COUNT(*), COALESCE(SUM(length(body)),0) FROM event_receipts")
        recent = self.query("server", "SELECT body FROM event_receipts ORDER BY rowid DESC LIMIT 8")
        queued = self.query("agent", "SELECT COUNT(*) FROM queue")
        return self.status(), stats[0] if stats else (0, 0), queued[0][0] if queued else 0, [
            json.loads(row[0]) for row in recent
        ]

    def close(self):
        for process in reversed(self.processes):
            if process.poll() is None:
                process.terminate()
                process.wait(timeout=8)
        self.log.close()
        self.temp.cleanup()


class Console:
    def __init__(self, window, backend):
        self.window, self.backend = window, backend
        self.busy = False
        self.closed = False
        self.installed = False
        self.state = ""
        self.worker = None
        window.title("Windows Agent Demo | AIDR 能力管理")
        window.geometry("1040x730")
        window.minsize(900, 675)
        window.configure(bg="#eef2f6")
        window.protocol("WM_DELETE_WINDOW", self.close)
        style = ttk.Style(window)
        style.theme_use("clam")
        style.configure("TFrame", background="#eef2f6")
        style.configure("TLabel", background="#eef2f6", foreground="#172b42", font=("Microsoft YaHei UI", 10))
        style.configure("Title.TLabel", font=("Microsoft YaHei UI", 20, "bold"))
        style.configure("Treeview", rowheight=27, font=("Microsoft YaHei UI", 10))
        style.configure("Treeview.Heading", font=("Microsoft YaHei UI", 10, "bold"))
        content = ttk.Frame(window, padding=24)
        content.pack(fill="both", expand=True)
        ttk.Label(content, text="Windows Agent · 能力管理", style="Title.TLabel").pack(anchor="w")
        self.identity = tk.StringVar(value="正在启动本地管理端与 Agent…")
        ttk.Label(content, textvariable=self.identity).pack(anchor="w", pady=(3, 14))
        actions = ttk.Frame(content)
        actions.pack(fill="x")
        self.buttons = {}
        for label, action, state in [("安装 AIDR 样例", "install", "running"),
                                     ("启动", "start", "running"), ("停止", "stop", "stopped"),
                                     ("卸载", "remove", "absent")]:
            button = ttk.Button(actions, text=label, command=lambda a=action, s=state: self.apply(a, s))
            button.pack(side="left", padx=(0, 8))
            self.buttons[action] = button
        ttk.Label(content, text="终端已安装能力").pack(anchor="w", pady=(16, 4))
        self.components = self.table(content, ["能力 ID", "版本", "运行状态", "进程 PID", "本机通信", "采集状态", "最近错误"], [120, 95, 95, 70, 140, 130, 250], 3)
        self.metric = tk.StringVar(value="已送达事件 0 条   │   已落库事件数据 0 B   │   待发送 0 条")
        ttk.Label(content, textvariable=self.metric).pack(anchor="w", pady=(14, 2))
        ttk.Label(content, text="数据量按管理端已落库事件原文计算，单位为字节").pack(anchor="w")
        self.events = self.table(content, ["UTC 时间", "能力 ID", "事件内容", "Event ID"], [205, 110, 165, 405], 5)
        self.feedback = tk.StringVar(value="连接后点击「安装 AIDR 样例」。安装成功会自动启动并发送测试事件。")
        ttk.Label(content, textvariable=self.feedback, wraplength=950).pack(anchor="w", pady=(10, 3))
        ttk.Label(content, text="安装链路  签名制品 → 管理端发布 → Agent 下载验签 → 独立进程 → 事件入库").pack(anchor="w")
        self.update_buttons()
        self.start_work(backend.start, "已连接。点击「安装 AIDR 样例」。")

    @staticmethod
    def table(parent, headers, widths, height):
        tree = ttk.Treeview(parent, columns=headers, show="headings", height=height)
        for header, width in zip(headers, widths):
            tree.heading(header, text=header, anchor="w")
            tree.column(header, width=width, anchor="w")
        tree.pack(fill="x")
        return tree

    def update_buttons(self):
        enabled = {"install": not self.installed, "start": self.state == "stopped",
                   "stop": self.state == "running", "remove": self.installed}
        for name, button in self.buttons.items():
            button.configure(state="normal" if not self.busy and enabled[name] else "disabled")

    def start_work(self, work, message):
        self.busy = True
        self.update_buttons()

        def execute():
            try:
                work()
                failure = None
            except Exception as error:
                failure = str(error)
            self.window.after(0, lambda: self.finished(message, failure))

        self.worker = Thread(target=execute, daemon=True)
        self.worker.start()

    def finished(self, message, error):
        if self.closed:
            return
        self.busy = False
        self.feedback.set(error or message)
        self.refresh()

    def apply(self, action, state):
        self.feedback.set({"install": "正在签名、发布、验签和启动 AIDR 样例…",
                           "start": "正在启动…", "stop": "正在停止…", "remove": "正在卸载…"}[action])
        self.start_work(lambda: self.backend.apply(state, publish=action == "install"),
                        {"install": "安装成功。稍等片刻，事件将出现在下方。",
                         "start": "启动成功。", "stop": "已停止。", "remove": "已卸载。历史接收记录保留。"}[action])

    def refresh(self):
        if self.closed:
            return
        try:
            report, (total, octets), queued, recent = self.backend.snapshot()
            if report:
                components = report.get("components") or {}
                selected = components.get(COMPONENT, {})
                self.installed = bool(selected)
                self.state = selected.get("state", "")
                self.identity.set(f"终端 demo-01    Supervisor PID {self.backend.processes[-1].pid}    "
                                  f"已接收 / 已应用策略 {report['received_revision']} / {report['applied_revision']}")
                self.components.delete(*self.components.get_children())
                for name, item in components.items():
                    label = {"running": "运行中", "stopped": "已停止", "failed": "失败",
                             "installing": "安装中", "restart_limited": "重启限流", "degraded": "异常",
                             "backoff": "等待重启"}
                    data = item.get("data") or {}
                    data_label = {"connecting": "正在建流", "receiving": "已收到事件", "reconnecting": "正在重连",
                                  "backpressure": "队列已满", "unsupported": "不支持事件流"}
                    actual, desired = item.get("version", ""), item.get("desired_version", "")
                    version = f"{actual or '—'} → {desired}" if desired and desired != actual else actual
                    self.components.insert("", "end", values=(name, version,
                        label.get(item["state"], item["state"]), item.get("pid", "—"),
                        {"npipe": "命名管道 · 无端口", "unix": "Unix socket · 无端口", "tcp": "回环 TCP"}.get(item.get("ipc_transport"), "—"),
                        data_label.get(data.get("state"), "—"), item.get("error") or data.get("error", "")))
                self.metric.set(f"已送达事件 {total:,} 条   │   已落库事件数据 {octets:,} B   │   待发送 {queued:,} 条")
                self.events.delete(*self.events.get_children())
                for event in recent:
                    self.events.insert("", "end", values=(event["timestamp"][:19], event["component_id"],
                                                          event["payload"], event["event_id"]))
        except Exception as error:
            self.feedback.set(str(error))
        self.update_buttons()
        if self.poll_job is not None:
            self.window.after_cancel(self.poll_job)
        self.poll_job = self.window.after(1000, self.refresh)

    poll_job = None

    def close(self):
        self.closed = True
        self.window.destroy()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--smoke", action="store_true", help="Verify the same actions used by the UI")
    args = parser.parse_args()
    backend = Backend()
    try:
        if args.smoke:
            backend.start()
            assert not backend.status()["components"]
            installed = backend.apply("running", publish=True)
            assert installed["components"][COMPONENT]["ipc_transport"] == "npipe"
            _, metric, _, events = backend.wait(lambda: value if (value := backend.snapshot())[1][0] else None)
            print(f"installed; received={metric[0]}; bytes={metric[1]}; event_id={events[0]['event_id']}")
            assert backend.apply("stopped")["components"][COMPONENT]["state"] == "stopped"
            assert backend.apply("running")["components"][COMPONENT]["state"] == "running"
            assert not backend.apply("absent")["components"]
            assert backend.processes[-1].poll() is None
            print(f"stop, restart and uninstall OK; Supervisor PID={backend.processes[-1].pid}")
            return
        window = tk.Tk()
        console = Console(window, backend)
        window.mainloop()
        if console.worker:
            console.worker.join(timeout=28)
    finally:
        backend.close()


if __name__ == "__main__":
    main()
