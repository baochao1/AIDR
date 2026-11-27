# Supervisor / AIDR 交付文档导航

本次交付围绕 Windows Agent 和第三方插件接入。本地管理端和制品仓库用于验证下发、管控、状态回传和事件送达。正式管理平台的页面、角色、批量运营和报表由后续产品设计确定。

| 读者 | 从这里开始 | 交付目的 |
|---|---|---|
| 领导或评审 | [三分钟汇报与截图顺序](demo/aidr-management-brief.md) | 说明业务价值、现场效果和验证边界 |
| 第三方厂商 | [我方提出的 AIDR 插件接入要求](integration/aidr-vendor-handoff.md) | 明确交付形态、启动协议、数据、签名发布和验收责任 |
| 后续研发 | [研发接手指南](overview/engineering-guide.md) | 找到代码入口、构建和回归命令、故障定位路径 |
| 产品经理 | [产品范围与验收](overview/product-scope.md) | 明确功能边界、用户流程和管理端下一阶段决策 |
| 演示人员 | [Windows Agent Demo 操作手册](demo/windows-agent-demo.md) | 双击启动并完成安装、收数、停止、卸载 |

## 专题资料

| 内容 | 主文档 |
|---|---|
| 模块职责、架构图、开源参考如何落地 | [架构设计](architecture/architecture.md)；固定源码和许可见[源码调研](research/research.md) |
| Health、Events 与多语言实现 | [gRPC 端点与语言示例](integration/aidr-grpc-endpoints.md)；实测传输见[原生 IPC](architecture/native-ipc.md) |
| 智能体、日志、Skill、MCP 的数据字段与文件通道 | [AIDR 数据契约](integration/aidr-data-contract.md) |
| 已运行、仅构建、尚未验收 | [验收记录](delivery/acceptance.md)；故障测试见[管控加固](delivery/agent-hardening.md) |
| 依赖、设计决策及阶段进度 | [依赖](architecture/dependencies.md) · [IPC ADR](adr/0001-component-ipc.md) · [实施计划](delivery/plan.md) |
| Agent 协议对照（Elastic、FleetDM、Elkeid、Wazuh） | [协议调研索引](../research/agent-protocol/index.md) |

## 目录结构

| 目录 | 内容 |
|---|---|
| [overview/](overview/) | 产品范围、研发接手指南 |
| [architecture/](architecture/) | 架构设计、原生 IPC、依赖清单 |
| [integration/](integration/) | 端管规范、gRPC 端点、数据契约、厂商接入 |
| [demo/](demo/) | 汇报讲稿、图形/终端/认证注册演示 |
| [delivery/](delivery/) | 实施计划、验收记录、管控加固 |
| [research/](research/) | 源码调研与 [Agent 协议调研（任务书、产品明细、认证过程与本地 Demo）](../research/agent-protocol/) |
| [adr/](adr/) | 架构决策记录 |

图形窗口是主要演示入口，逐步终端脚本作为备用。厂商说明保留我方接入要求，语言代码、数据字段和架构各有主文档。原生 IPC 无插件 TCP 监听端口，只有兼容 TCP 模式使用插件端口。业务事件当前按原文转发，分类、关联和文件上传属于后续方案。

研发接口基线见 [AIDR 端管对接规范 v1](integration/aidr-endpoint-platform-v1.md)。

认证注册演示见 [AIDR 管理端认证注册演示](demo/aidr-enrollment-demo.md)。
