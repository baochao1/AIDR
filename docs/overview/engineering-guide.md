# 研发接手指南

先读[架构设计](../architecture/architecture.md)，其中有模块边界、时序图和开源来源映射。具体接口以代码和协议为准。

| 任务 | 源码入口 | 关注点 |
|---|---|---|
| Agent 管控与升级 | [agent.go](../../internal/demo/agent.go)、[agent_control.go](../../internal/demo/agent_control.go) | 策略 revision、期望与实际状态、重启预算、版本切换 |
| 插件和 IPC | [component.go](../../internal/component/component.go)、[events.go](../../internal/component/events.go)、[events.proto](../../api/proto/component/v1/events.proto) | 监听与拨号、TLS 和 token、Health、事件流与 ACK |
| 制品和完整性 | [artifact.go](../../internal/demo/artifact.go) | 签名、解包限制、缓存复用和启动前验证 |
| 数据入队和平台接收 | [events.go](../../internal/demo/events.go)、[state.go](../../internal/demo/state.go)、[http.go](../../internal/demo/http.go) | SQLite 提交顺序、事件序号、去重和背压 |
| 构建与 UI | [verify.ps1](../../scripts/verify.ps1)、[e2e.py](../../scripts/e2e.py)、[图形控制台](../../scripts/aidr-agent-console.py) | Agent、插件和配套管理端真实运行 |

## 构建与回归

见 [项目构建入口](../../README.md#构建和回归) 和 [命令行手工流程](../../README.md#手动运行)。先运行 Go 单测和 vet，再跑端到端脚本。记录 OS、CPU、实际运行条件。交叉构建、WSL2 和原生运行分别记录。图形控制台的 smoke 参数只验证同一后端流程。

## 排障顺序

1. 安装失败先核对 revision、制品 ID、签名、平台和校验错误，再看旧版恢复情况。
2. 插件运行但无事件，检查 Agent 是否启用 -events，再看采集状态、最近接收时间和插件是否实现 Events。Python Mock 当前只实现 Health。
3. 有终端 ACK 却查不到平台数据，检查终端队列、平台 mTLS 可达性及同一 event_id 的管理端入库记录。
4. 插件反复退出，查看失败预算、组件 PID、运行时收尾。验证同一插件 stream 断开后能否恢复。
5. 变更后更新[验收矩阵](../delivery/acceptance.md)。正式权限隔离、配置回执、完整升级事务和文件通道见[加固边界](../delivery/agent-hardening.md)及[数据契约](../integration/aidr-data-contract.md)。
