# Windows Agent Demo 操作手册

## 1. 打开 Agent

1. 在 Windows 文件资源管理器打开 L:\workspace\AIDR。
2. 双击 Start-AIDR-Demo.cmd。也可以在该目录的 PowerShell 输入 python scripts/aidr-agent-console.py。
3. 等窗口显示「已连接」。此时已启动本地管理端和一个空白 Agent，已安装能力表为空。界面顶部可看到终端 ID、Supervisor PID 和当前策略版本。无需管理员权限。
4. 如果提示找不到 python，先用 python --version 确认 Windows 找得到 Python 3。窗口同时使用 build/supervisor-demo.exe 和 build/mock-go.exe，这两份二进制已经构建。

## 2. 安装并看运行状态

1. 点击「安装 AIDR 样例」。这个按钮使用随项目提供的 Go Mock，界面会自动完成签名和管理端发包。
2. 几秒后，能力列表应出现 aidr-mock，版本 1.0.0，状态「运行中」，以及组件自己的进程 PID。此时可以截图。
3. 若安装出错，界面底部和能力表会显示错误。关闭窗口再打开，可从空白 Agent 重新开始。

按钮对应真实安装流程。演示控制台把 mock-go.exe 和 manifest.json 封装成签名制品，发布到管理端，再下发 running 策略。Agent 主动领取策略，下载制品，验证签名、摘要、平台和入口，最后将插件放入版本目录，并启动独立进程。能力表来自实际 check-in 状态。

## 3. 查看数据量和事件

1. 安装后等约两秒。窗口下半部会出现事件，且「已送达事件」和「已落库事件数据」持续增长。
2. 「已送达事件」是管理端持久化的事件条数。「已落库事件数据」是已提交事件 JSON 的累计字节数。「待发送」来自终端 SQLite 队列。字节数口径为应用层事件大小，无 TLS、zstd 和 HTTP 头开销。
3. 截图时保留整个窗口，同时包含能力 ID、运行状态、数据量及事件 ID。

## 4. 停止、重启和卸载

1. 点「停止」，等待能力状态变成「已停止」，组件 PID 消失。已经进入队列的事件仍可完成发送，随后计数稳定。
2. 点「启动」，能力恢复「运行中」，组件重新分配 PID，事件计数继续增长。
3. 点「卸载」，等待能力表清空。管理端已有事件保留，可同时展示历史送达记录和当前空白的能力表。Supervisor PID 在这几个操作期间保持一致。
4. 关闭窗口会结束本次演示的 Agent 和管理端。下次打开会重新创建隔离的临时环境。

## 5. AIDR 样例的代码和替换边界

本次样例为 build/mock-go.exe，源码入口是 cmd/mock-go/main.go。它作为独立进程运行，按 internal/component/component.go 提供 TLS gRPC Health，并按 internal/component/events.go 每约 250 毫秒产生一条合成心跳。冷启动后每条事件都经历「适配器发出 → Agent 落入 SQLite → 返回事件 ID → 压缩上传 → 管理端持久化」。最新事件原文中的 mock heartbeat 就是这个样例发送的内容。

实际对接时，厂商提供独立适配器二进制，或提供 SDK 由适配器加载。适配器实现当前 Health 与 EventStream 契约，再用相同流程签名发布。本界面的安装按钮用于内置样例。第三方制品目前走 README 中的 sign、publish、policy 命令和 manifest 校验流程。

## 技术入口

- 协议 api/proto/component/v1/events.proto
- 对接规范 docs/demo/aidr-integration-demo.md
- 界面 scripts/aidr-agent-console.py
- 不打开窗口的流程验证 python scripts/aidr-agent-console.py --smoke
「本机通信」列显示「命名管道 · 无端口」时，表示本次 Go AIDR 样例没有使用 TCP 监听端口。管理端 HTTPS 仍独立工作。

## 备用方式

需要终端日志或暂停截图时，可运行 [逐步终端演示](aidr-integration-demo.md)。现场演示讲法见 [三分钟汇报](aidr-management-brief.md)。
