# M0 固定源码核对

| 项目 | 固定引用 | 源码入口与结论 | 许可证及复用 |
|---|---|---|---|
| Elastic Agent | [v8.17.0 · 96f2b9f](https://github.com/elastic/elastic-agent/tree/96f2b9fc0dbddfcaac1c55e0e0bc9aed3bf6badb) | `pkg/component/runtime/manager.go` 的 `Manager.Update(component.Model)` 接收期望模型。该文件直接导入 `elastic-agent-client/v7/pkg/{client,proto}`，组件协议有独立来源。 | 根许可证需按 ELv2 约束理解，参考协调模型 |
| Fleet Server | [v8.17.0 · d05ee7b](https://github.com/elastic/fleet-server/tree/d05ee7b30ed62dba0cad7b31ff2ea003897cde82) | `internal/pkg/api/api.go` 的 `AgentEnroll`、`AgentCheckin` 和 `AgentAcks` 分派请求。`handleAck.go` 中 `handleAcks → processRequest → handleAckEvents` 处理确认。 | Elastic License，参考协议分工 |
| go-plugin | [v1.6.3 · 5b05b3e](https://github.com/hashicorp/go-plugin/tree/5b05b3e78c6c3e4e0b8f21a9921cbd739ba72f2a) | `Client.Start` 启动并读取握手。`server.go` 读取协议版本环境变量，Windows 使用 TCP。`plugin.go` 提供 `GRPCPlugin` 和 `NetRPCUnsupportedPlugin`。`ReattachConfig` 是候选能力，重连未实测。 | MPL-2.0，仅 PoC 依赖 |
| Nomad | [v1.9.5 · 0b7bb8b](https://github.com/hashicorp/nomad/tree/0b7bb8b60758981dae2a78a0946742e09f8316f5) | `plugins/drivers/driver.go` 的 `DriverPlugin` 分离任务恢复、等待、检查与生命周期，定义 `RecoverTask(*TaskHandle)`、`WaitTask` 和 `InspectTask`。 | 根 LICENSE 为 BUSL，借鉴接口边界 |
| Go supervisord | [ea8e753](https://github.com/ochinchina/supervisord/tree/ea8e753278fc6df10b766026258483837aeae508) | `process/process.go` 的 `Process.Start` 与 `LivenessChecker.DoLivenessCheck` 分别处理进程启动、成功及失败阈值。 | MIT，参考状态和阈值 |
| Mender | [4.0.0 · 146951a](https://github.com/mendersoftware/mender/tree/146951aa650064af0c28b1e42eb729bcd7aa9155) | `src/mender-update/update_module/v3/update_module.cpp` 的 `UpdateModule::ArtifactCommit` 分派提交状态，`daemon/states.cpp` 调用异步提交。 | 根 LICENSE 默认 Apache-2.0，文件例外以其声明为准，参考事务 |

本表基于实际下载的固定源码树。上游二进制运行验证集中在 go-plugin PoC。

| 独立来源 | 固定引用 | 实际入口与许可 |
|---|---|---|
| Elastic Agent Client | [v7.16.0 · 86f524d](https://github.com/elastic/elastic-agent-client/tree/86f524ddc40d8aae5a01244ae223fb87f8b33265) | 与 Elastic Agent 固定版本的 Go 依赖一致。`elastic-agent-client.proto` 定义 `CheckinV2` 双向期望/实际状态流和 `Actions`。Elastic License，架构参考。 |
| Mender Artifact | [e56270c](https://github.com/mendersoftware/mender-artifact/tree/e56270c4a7cc4a028c11252b78cd1f3fbb0359df) | `awriter/writer.go` 的 `NewWriterSigned`、`WriteSignature` 和 `WriteArtifact` 显式处理签名和版本化打包。Apache-2.0 默认，参考封装。 |
| Elkeid | [c906c51](https://github.com/bytedance/Elkeid/tree/c906c512d20c0d56f2b779b8558ed69abae718c8) | `agent/plugin/plugin.go` 的 `Plugin.ReceiveData`、`SendTask` 与 `Pid`。Agent 模块 Apache-2.0，参考进程及消息边界。 |
| Wazuh Agent | [ca21132](https://github.com/wazuh/wazuh-agent/tree/ca211324af5c88caf77fcd57cab0e98475f8e4ce) | `src/agent/multitype_queue/src/multitype_queue.cpp` 的 `MultiTypeQueue` 构造函数接受持久化后端并读取队列容量和批次配置。AGPL-3.0，参考配置。 |
| MiNiFi C++ | [1c87f39](https://github.com/apache/nifi-minifi-cpp/tree/1c87f39c13e45c226bf11442af5f454f77fa6db9) | `libminifi/src/c2/C2Agent.cpp` 的 `start` 初始化最近成功心跳，`configure` 读取心跳周期。Apache-2.0，参考控制通道。 |
| FleetDM | [3807bd7](https://github.com/fleetdm/fleet/tree/3807bd79fedbd25424ae55738b03cc8a4ae28a80) | `orbit/pkg/installer/installer.go` 的 `Runner.Run` 及下载接口。根许可证区分 MIT、文档和企业目录，参考软件分发流程。 |

## 两份 HTML 与实施指南的取舍

《统一客户端第三方插件 SDK 集成设计方案》适合定义集成边界。独立适配进程可以承接厂商 SDK 的不同 ABI、配置与工作目录，声明式组件身份和能力方便协调。实际 SDK 和高权限服务需另行验证。

《开源 EDR 框架选型评估报告》适合定义运行约束。可复用的是进程状态协调、版本隔离、回滚、队列容量与可观测性思路。其 Windows 检测选型以及 WS/WAL 数据组合属于后续工作。

建议以实施指南为准，结合上述两者的进程隔离和版本控制，先验证签名组件的动态生命周期。当时建议将终端数据出口放到 M5。当前已实现独立出站数据上传，部署与验收范围见架构文档。
各项目调研结果落到哪些模块，见 [架构设计与开源模块映射](architecture.md#4-初期方案和开源参考如何落到模块)。

