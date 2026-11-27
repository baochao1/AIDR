# 六个参考实现与 AIDR 的对应关系

字段和报文以 [AIDR 协议与第三方插件数据契约](endpoint-platform-v2.md) 为准。

| 边界 | 参考实现与可追溯项 | AIDR 用法 |
|---|---|---|
| Agent 认证 | [Elkeid](elkeid/authentication.md) 部署证书、双向 TLS、Transfer 首包 | AIDR 独立实现，参考该流程。服务端从首包读取 agent_id 并加入在线连接池 |
| 证书签发与制品更新参考 | [FleetDM Orbit](fleetdm-orbit/fleetdm-orbit.md) 记录 SCEP；[Orbit TUF](https://github.com/fleetdm/fleet/blob/3807bd79fedbd25424ae55738b03cc8a4ae28a80/orbit/README.md) 用于制品更新 | SCEP 留作参考，端管认证按 Elkeid 的部署证书流程。更新部署条件见[选型](design-optimization.md) |
| 管控、遥测 | [Elastic 8.17.0](elastic/elastic.md) 的 Check-in、ACK、Bulk 逐条响应 | 采用接收与应用结果分开、批量逐条确认的行为，字段用 AIDR 既有定义 |
| 文件上传 | [FleetDM/osquery](fleetdm/fleetdm.md#文件采集上传) 的分块与分块确认 | 分块模式参考。续传使用 AIDR 的 offset 与完整性校验 |
| 在线状态 | [Wazuh 4.14.0](wazuh/wazuh.md) 的启动和 keepalive | 对比上线判定；帧名、seq 和离线阈值沿用 AIDR |
| 命令结果 | [独立 wazuh-agent 实验实现](wazuh-agent/wazuh-agent.md) 的 HTTP/JWT、事件及命令 | 作为交互对照，证据范围限该实验实现 |

插件本机 IPC 直接复用 AIDR 已有 gRPC 和标准 Health。业务类型由 [AIDR 数据契约](endpoint-platform-v2.md#业务类型和-data-字段) 定义。
