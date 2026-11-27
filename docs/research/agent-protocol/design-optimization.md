# 协议与组件复用选型

| 能力 | 复用路径 | 正式接入前核验 |
|---|---|---|
| 认证 | AIDR 独立实现，参考 Elkeid：部署证书、双向 TLS、Transfer 首包携带 agent_id、服务端自动登记在线连接。 [Elkeid 认证](elkeid/authentication.md) | 验证可信与不可信证书、首包失败、重复 ID 和重连 |
| 插件建连 | 复用仓库 gRPC Events、标准 Health、平台原生 IPC。厂商仅实现插件监听和业务事件 | 用签名制品声明协议版本，检查 Demo 纯 ID 回执与正式 JSON ACK 的互操作 |
| 制品下载 | 优先验证复用 [Orbit 的 TUF 路线](https://github.com/fleetdm/fleet/blob/3807bd79fedbd25424ae55738b03cc8a4ae28a80/orbit/README.md) | 固定 TUF 客户端和仓库，验证离线、过期、目标摘要与旧版本回滚保护。试点前使用归档 v1 的签名制品交互作为接口基线 |
| 字段与枚举 | 标准 gRPC 枚举直接沿用，AIDR 业务枚举沿用归档定义并补齐语义 | 按主稿样例验证字段、非法枚举、重放和错误恢复 |

对接入口为 [AIDR 协议与第三方插件数据契约](endpoint-platform-v2.md)。逐产品证据见[参考对应关系](design-comparison.md)。
