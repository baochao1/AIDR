# Supervisor / AIDR 交付文档导航

本次交付围绕 Windows Agent 和第三方插件接入。本地管理端和制品仓库用于验证下发、管控、状态回传和事件送达。正式管理平台的页面、角色、批量运营和报表由后续产品设计确定。

| 读者 | 从这里开始 | 交付目的 |
|---|---|---|
| 领导或评审 | [三分钟汇报与截图顺序](aidr-management-brief.md) | 说明业务价值、现场效果和验证边界 |
| 第三方厂商 | [我方提出的 AIDR 插件接入要求](aidr-vendor-handoff.md) | 明确交付形态、启动协议、数据、签名发布和验收责任 |
| 后续研发 | [研发接手指南](engineering-guide.md) | 找到代码入口、构建和回归命令、故障定位路径 |
| 产品经理 | [产品范围与验收](product-scope.md) | 明确功能边界、用户流程和管理端下一阶段决策 |
| 演示人员 | [Windows Agent Demo 操作手册](windows-agent-demo.md) | 双击启动并完成安装、收数、停止、卸载 |

## 专题资料

| 内容 | 主文档 |
|---|---|
| 模块职责、架构图、开源参考如何落地 | [架构设计](architecture.md)；固定源码和许可见[源码调研](research.md) |
| Health、Events 与多语言实现 | [gRPC 端点与语言示例](aidr-grpc-endpoints.md)；实测传输见[原生 IPC](native-ipc.md) |
| 智能体、日志、Skill、MCP 的数据字段与文件通道 | [AIDR 数据契约](aidr-data-contract.md) |
| 已运行、仅构建、尚未验收 | [验收记录](acceptance.md)；故障测试见[管控加固](agent-hardening.md) |
| 依赖、设计决策及阶段进度 | [依赖](dependencies.md) · [IPC ADR](adr/0001-component-ipc.md) · [实施计划](plan.md) |

图形窗口是主要演示入口，逐步终端脚本作为备用。厂商说明保留我方接入要求，语言代码、数据字段和架构各有主文档。原生 IPC 无插件 TCP 监听端口，只有兼容 TCP 模式使用插件端口。业务事件当前按原文转发，分类、关联和文件上传属于后续方案。
