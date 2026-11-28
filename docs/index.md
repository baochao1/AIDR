# AIDR 文档导航

当前设计基线以 [AIDR Agent 与管理端协议](research/agent-protocol/endpoint-platform-v2.md) 为准，只覆盖 Agent ↔ 管理端交互。友商调研（Elastic、FleetDM、Elkeid、Wazuh）固定到具体版本，仅作参照，不是设计结论。

## 主目录

| 内容 | 入口 |
|---|---|
| 设计基线：协议与插件数据契约 | [endpoint-platform-v2.md](research/agent-protocol/endpoint-platform-v2.md) |
| 调研索引 | [agent-protocol/index.md](research/agent-protocol/index.md) |
| 候选清单与固定版本 | [candidates.md](research/agent-protocol/candidates.md) |
| Elastic Agent / Fleet | [elastic/](research/agent-protocol/elastic/elastic.md) |
| FleetDM osquery | [fleetdm/](research/agent-protocol/fleetdm/fleetdm.md) |
| FleetDM Orbit | [fleetdm-orbit/](research/agent-protocol/fleetdm-orbit/fleetdm-orbit.md) |
| Elkeid | [elkeid/](research/agent-protocol/elkeid/elkeid.md) |
| Wazuh 4.14.0 | [wazuh/](research/agent-protocol/wazuh/wazuh.md) |
| 独立 wazuh-agent 实现 | [wazuh-agent/](research/agent-protocol/wazuh-agent/wazuh-agent.md) |

历史阶段的架构、集成、演示与验收文档已删除，不再保留；遇到指向 `docs/archive/`、`design-comparison.md`、`design-optimization.md` 或旧版端管规范的引用，一律以 endpoint-platform-v2.md 为准。
