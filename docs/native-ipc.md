# 原生本机 gRPC 传输

Go 组件的签名 manifest 可以声明 ipc_transport 为 native。Windows 将它映射为 Named Pipe，Linux 和 macOS 将它映射为 Unix Domain Socket。此模式中，Agent 与组件仍使用实例 TLS 证书和 token，健康检查与事件流方法保持兼容。Agent 的管理端 HTTPS 出站连接独立保留。

| 签名参数 | Windows | Linux/macOS | 适用对象 |
|---|---|---|---|
| -ipc native | Named Pipe，无 TCP 端口 | Unix socket，无 TCP 端口 | 实现原生传输的适配器，已内置 Go Mock |
| -ipc tcp 或省略 | TLS 回环 TCP | TLS 回环 TCP | 现有 Python 样例和仍在验证原生传输的厂商组件 |

每实例由 Agent 通过 DEMO_IPC_TRANSPORT 传入 npipe、unix 或 tcp，并在 DEMO_ENDPOINT 传入对应地址。Windows 管道 ACL 授权运行该组件的账户，库拒绝远程管道客户端。Linux/macOS 的 socket 放在权限为 0700 的随机目录，正常停止后由 Agent 清理。原生模式连接失败会按组件失败上报。Agent 被强杀时可能留下 Unix 临时 socket 目录。制品声明 native 后，连接失败会报告错误，运行时不会自动改用 TCP。IPC 权限与 TLS 认证不等同于组件沙箱。

Windows Agent 图形演示的「安装 AIDR 样例」默认签发 native 制品。重开 Start-AIDR-Demo.cmd 后，能力列表的「本机通信」列显示「命名管道 · 无端口」。命令行可用 sign -ipc native，随后按 README 发布和应用策略。

## 验证记录

- Windows amd64 本机：两个独立 Go 组件同时通过 Named Pipe 完成探活和双向流，管理端查询到两者的记录。测试通过 netstat -ano 按两个组件 PID 核对均无 TCP socket。Python TCP 兼容组件同时运行。完整生命周期和恢复验收通过。
- Linux amd64 WSL2：两个 Go 组件通过 Unix socket 完成相同测试。按 /proc 的组件 socket inode 核对与本机 TCP 表无交集。
- Windows、Linux、macOS 的 amd64 和 arm64 Go 构建通过。macOS 原生运行、ARM64 原生运行、跨账户 ACL 与厂商 Python/Java 原生连接尚未验收。

