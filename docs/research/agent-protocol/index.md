# Agent 协议调研索引

当前唯一设计基线是 [AIDR Agent 与管理端协议](endpoint-platform-v2.md)。其他文档均为友商调研参考，不是设计结论。

## 设计基线

| 文档 | 内容 |
|---|---|
| [endpoint-platform-v2.md](endpoint-platform-v2.md) | 只覆盖 Agent ↔ 管理端及部署侧交互：认证上线、心跳管控、制品下载、遥测、附件上传 |

## 友商调研

| 文档 | 覆盖范围 |
|---|---|
| [候选清单](candidates.md) | AIDR 已有参考、补充候选、首批产品及固定版本 |
| [Elastic Agent / Fleet](elastic/elastic.md) | 注册与访问凭据、Fleet 控制接口、Elasticsearch Bulk 数据面 |
| [FleetDM / osquery](fleetdm/fleetdm.md) | 节点注册和认证、日志及查询结果上报、文件分块上传 |
| [FleetDM / Orbit](fleetdm-orbit/fleetdm-orbit.md) | Orbit 注册、节点密钥、主机身份签名、SCEP 与 WebSocket |
| [Elkeid](elkeid/elkeid.md) | 双端 TLS、服务发现、gRPC 上报及文件流、AgentCenter 到 Kafka |
| [Elkeid 采集字段附录](elkeid/elkeid-fields.md) | 53 种记录、926 个唯一类型与字段配对 |
| [Wazuh 4.14.0](wazuh/wazuh.md) | 正式 Agent 与 Manager 的 TLS 注册、密钥、事件与控制消息 |
| [独立 wazuh-agent 实现](wazuh-agent/wazuh-agent.md) | 独立实验 Agent，包含 HTTP 注册、JWT、事件批次与命令 |

每个产品目录另有一份认证过程文档（`authentication.md`），从接口规格提炼注册、凭据与失败重试路径。

已删除的历史设计文档、旧版端管规范与演示文档不再存在；如遇引用它们的链接，一律以 [endpoint-platform-v2.md](endpoint-platform-v2.md) 为准。
