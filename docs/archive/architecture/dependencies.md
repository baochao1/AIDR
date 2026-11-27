# 依赖清单

Go 依赖的版本和完整性摘要见 `go.mod` 和 `go.sum`。以下许可证依据下载模块的根许可证核对。

| 直接依赖 | 版本 | 许可证 | 用途 |
|---|---|---|---|
| hashicorp/go-plugin | 1.6.3 | MPL-2.0 | IPC A/B PoC |
| grpc-go | 1.70.0 | Apache-2.0 | 组件 RPC |
| protobuf-go | 1.36.1 | BSD-3-Clause | EventStream 消息 |
| golang.org/x/sys | 0.31.0 | BSD-3-Clause | 状态锁、Windows Job Object、Unix 进程控制 |
| modernc.org/sqlite | 1.36.3 | BSD-3-Clause | 无 CGO 的 SQLite 接口 |
| klauspost/compress | 1.18.0 | BSD-3-Clause，模块内第三方文件各自声明 | zstd |
| grpcio | 1.76.0 | Apache-2.0 | Python Mock |
| PyInstaller | 6.16.0 | GPL-2.0-or-later，含分发例外 | Python Mock 构建工具与 bootloader |

`go list -m all` 可复现锁定的 Go 模块图，`go version -m <binary>` 可核对实际参与链接的模块。Python 入口依赖版本由 `plugins/mock-python/requirements.txt` 固定。生产分发前仍需生成精确到制品的平台许可证清单并附带要求的许可证与通知。
Windows 命名管道使用 Microsoft/go-winio v0.6.2，许可证为 MIT。
