# Demo 验收记录

## 环境和命令

- Windows 11 amd64，Go 1.24.13，Python 3.13.7。便携 Go 用于构建。执行 `./scripts/verify.ps1`，涵盖 `go test ./...`、`go vet ./...`、三条 Go 构建命令、Go/Python A/B PoC、组件打包和端到端脚本。
- Ubuntu 24.04 amd64 运行于 WSL2，Python 3.12.3。Windows Go 交叉生成的 ELF 在 WSL2 中执行单测二进制、A/B PoC 及 Python 打包制品的端到端脚本。实际执行的 Linux 二进制包含 Supervisor、管理端、Go Mock 和 Python Mock。
- 六目标 Go 构建均使用 `CGO_ENABLED=0 go build ./...`，退出码 0。Python 制品在 Windows amd64 和 WSL2 Linux amd64 构建。
- 本机日志见 `build/verify-windows.log` 和 `build/matrix.log`。包含测试输入和临时凭据的根目录由测试脚本分配和清理。

## 验收场景

| ID | Windows amd64 和 Ubuntu amd64 WSL2 | 证据 |
|---|---|---|
| T01 | 通过 | 空状态目录启动，注册并出现端点状态 |
| T02/T03 | 通过 | 动态发布并安装 Go、Python，Supervisor PID 保持 |
| T04 | 通过 | 停止后保持停止，再启动，卸载删除版本目录，数据目录保留 |
| T05 | 部分通过 | 强杀一个组件，随后启动新 PID。卡死与磁盘耗尽专项未执行 |
| T06 | 通过 | 管理服务离线后重启，收到更新的 check-in，组件 PID 保持 |
| T07 | 通过 | 重复策略保持同一 PID，同 revision 不同内容与旧 revision 拒绝 |
| T08 | 通过 | v1 至 v2 生效，同机 Python PID 保持 |
| T09 | 通过 | v3 连续探活失败，恢复 v2，状态包含失败原因 |
| T10 | 部分通过 | 等待旧版停止后进入 v3 启动，强杀 Supervisor，重启后从持久化状态恢复 v2。其他崩溃时点未覆盖 |
| T11 | 部分通过 | 实跑错误签名、ZIP 篡改、穿越、平台误匹配、非授权管理员请求拒绝。恶意重解析点、操作系统权限边界未实测 |
| T12 | 通过 | Go、Python Mock 通过同一 gRPC Health 接入，Python 随制品打包 |
| T13 | 部分通过 | Windows 本机和 Linux WSL2 运行。macOS、ARM64 和独立 Linux 机器待验证 |
| T14 | 通过基本场景 | 控制场景全程启用合成事件，离线入队，强杀重启后原 event ID 出现在管理库。重复批次去重另由 Go 单测验证 |

## 平台矩阵

| 平台 | amd64 Go 构建 | amd64 运行 | arm64 Go 构建 | arm64 运行 |
|---|---|---|---|---|
| Windows | 通过 | 本机通过上述场景 | 通过 | 未执行 |
| Linux | 通过 | WSL2 通过上述场景 | 通过 | 未执行 |
| macOS | 通过 | 未执行 | 通过 | 未执行 |

三端原生 CI 定义位于 `.github/workflows/demo.yml`。此工作区未初始化 Git 仓库，远程 CI runner 无实际运行记录。

## 实现边界和后续核验

- Demo 身份引导面向一个测试 CA、一个端点及一个管理角色。服务器证书默认绑定 localhost/127.0.0.1。证书轮换、多租户和完整动作审计待实施。
- 组件控制已提供 gRPC Health、启动、停止及合成事件流。GetInfo、Configure、Shutdown、Execute 的通用 SDK 契约和临时任务 ACK API 待实施。
- Windows 使用挂起启动并绑定 Job Object。Linux/macOS 使用进程组，合作型 Mock 监视父进程管道。进程组逃逸、异常重启后的平台级清理验证，以及 SCM/systemd/launchd 安装验收待完成。
- 组件支持 Named Pipe、Unix socket 和 TLS 回环 TCP，均使用每实例证书与 token。Windows 本机和 Linux WSL2 的双组件零 TCP socket 场景见 [原生 IPC 验收](../architecture/native-ipc.md)。跨账户 ACL 验收待完成。同一权限账户下可运行的 Mock 未作沙箱化。
- SQLite 持久化期望状态、已提交版本、队列和接收记录。崩溃恢复基于旧版本提交点和重放当前策略。完整升级事务历史、阶段清理和涵盖所有故障窗口的验证待实施。
- 周期上传目前使用固定间隔。队列满时对组件背压。服务器保留接收记录，长期容量、批量吞吐、磁盘满恢复、大文件通道和长时间 soak 未验证。未运行 race 检测，亦未采集 CPU、RSS 或吞吐基准。

