# AIDR 文档导航

当前设计基线以 [Agent 协议调研](research/agent-protocol/index.md) 为准。调研固定到各厂商的具体版本，覆盖认证注册、数据面协议、字段字典和可运行的本地 Demo，作为后续设计的直接依据。

## 主目录

| 内容 | 入口 |
|---|---|
| 调研索引、任务书与复核结论 | [agent-protocol/index.md](research/agent-protocol/index.md) |
| 六个参考实现 | [design-comparison.md](research/agent-protocol/design-comparison.md) |
| 协议与组件复用选型 | [design-optimization.md](research/agent-protocol/design-optimization.md) |
| AIDR 协议和第三方插件数据契约 | [endpoint-platform-v2.md](research/agent-protocol/endpoint-platform-v2.md) |
| 候选清单与固定版本 | [candidates.md](research/agent-protocol/candidates.md) |
| Elastic Agent / Fleet | [elastic/](research/agent-protocol/elastic/elastic.md) |
| FleetDM osquery | [fleetdm/](research/agent-protocol/fleetdm/fleetdm.md) |
| FleetDM Orbit | [fleetdm-orbit/](research/agent-protocol/fleetdm-orbit/fleetdm-orbit.md) |
| Elkeid | [elkeid/](research/agent-protocol/elkeid/elkeid.md) |
| Wazuh 4.14.0 | [wazuh/](research/agent-protocol/wazuh/wazuh.md) |
| 独立 wazuh-agent 实现 | [wazuh-agent/](research/agent-protocol/wazuh-agent/wazuh-agent.md) |

各产品目录包含主分析、认证过程和一个可运行的 `demo.py`；Elkeid 另附采集字段字典。运行方式与覆盖范围见调研索引。

## 归档

上一阶段的架构、集成、演示、实施与验收文档已移至 [archive/](archive/legacy-index.md)，仅保留历史追溯，不再作为当前设计基线。其中 [固定源码核对](archive/research/research.md) 仍可作为项目来源与许可的参考。
