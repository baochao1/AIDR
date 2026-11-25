# ADR 0001 组件控制 IPC

Demo 使用标准 gRPC Health 检查与由 Supervisor 创建的独立进程。Go 和 Python 共用 gRPC Health v1 的 Protobuf 线格式。Supervisor 为每实例生成随机 token 和临时服务器证书，通过 loopback TCP 发起证书绑定的 TLS RPC。管理流量走独立 HTTPS mTLS。组件从带独立 Ed25519 签名的制品仓库下发。

A 使用 go-plugin v1.6.3 的子进程握手、AutoMTLS 和 gRPC Dispense。B 使用标准 gRPC 并显式绑定进程生命周期。原生协议适合直接对接跨语言 gRPC 服务，减少重写握手及控制服务的成本。go-plugin 的进程握手与 AutoMTLS 仍可用作扩展参考。PoC 时长只用于记录一次运行，受进程启动和连接退避影响。

Windows amd64 两轮 PoC 中，A 的握手加 10 次检查约 110–111 ms，B 约 1.02–1.07 s。B 的数字包含初次连接早于监听时触发的 gRPC 重连等待。两轮 B 分别使用 Go 和 Python，并验证错误 token 拒绝。A 的 Go 实现提供 AutoMTLS 和传输握手，Windows 实际使用 TCP，WSL Linux 的 A 实际使用 Unix socket。A 的非 Go 握手、复用多个 broker 流和 Reattach 留作未验证项。

选择 B 是为了复用通用 gRPC 服务实现和独立的运行时策略。Go 和 Python 使用相同的实际运行时 TLS/token 契约。端口仍有竞争窗口，进程身份校验确保错误监听者无法接收 token。后续实现保留 TLS/token，并按签名 manifest 启用 Windows Named Pipe 或 Linux/macOS Unix socket。当前验证见 [原生 IPC 验收](../native-ipc.md)。跨账户 ACL 和同用户隔离仍属后续工作。

