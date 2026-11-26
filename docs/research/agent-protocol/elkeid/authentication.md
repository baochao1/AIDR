# Elkeid 认证过程

本文从 [接口规格](elkeid.md) 提炼认证过程，配套可运行示例 [demo.py](demo.py)。固定提交 `c906c512`，双向 TLS 与连接生命周期结论以接口规格为准。

## 角色与凭据

| 角色 | 凭据 | 来源 | 验证方式 |
|---|---|---|---|
| Agent | CA + 客户端证书 + 客户端私钥 | 构建时嵌入 PEM | 连接前解析，自定义校验服务端证书链到指定 CA 并核对 DNS 名 `elkeid.com` |
| AgentCenter | 服务端证书 + 私钥 + 客户端信任 CA | 磁盘加载（`server.ssl.certfile` 等） | 要求并验证客户端证书，失败则退出进程 |
| 业务身份 | `agent_id` | 配置、主机标识或本地 `machine-id` | 首包上报，AgentCenter 以该 ID 登记在线连接 |

该路径没有注册 RPC，也没有签发、续期或吊销 RPC。证书生命周期完全由部署环境提供；Agent 证书轮换需要重新构建并部署，服务端材料在启动时重新加载。

## 认证时序

```mermaid
sequenceDiagram
    participant A as Agent
    participant C as AgentCenter
    A->>A: 加载嵌入 CA / 客户端证书 / 私钥
    C->>C: 加载服务端证书 / 私钥 / 信任 CA
    A->>C: TCP + TLS ClientHello
    C-->>A: 服务端证书链
    A->>A: 校验证书链到 CA，核对 DNS 名 elkeid.com
    A->>C: 客户端证书
    C->>C: 按信任 CA 验证客户端证书
    A->>C: gRPC Transfer 首包（含 agent_id）
    C->>C: 注册在线连接，下发当前配置
    C-->>A: Command
```

## 逐步过程

1. **准备材料**：Agent 在构建时嵌入 CA、客户端证书与私钥；AgentCenter 在启动时从磁盘读取服务端证书、私钥及客户端信任 CA。任一加载失败即退出。
2. **服务端校验**：Agent 的 `setDialOptions` 解析 PEM，自定义校验对端证书链到指定 CA，并验证预期 DNS 名 `elkeid.com`。TLS 失败发生在任何 RPC 消息交换之前。
3. **客户端校验**：AgentCenter 要求并验证客户端证书。未通过握手时连接直接失败，不会进入 `Transfer` 流。
4. **业务身份登记**：TLS 建立后 Agent 打开 `/grpc.Transfer/Transfer` 双向流，首个 `PackagedData` 携带 `agent_id`。AgentCenter 读取首包 ID 登记连接池；冲突时返回 `agentID conflict`，已有连接继续存在。首条心跳初始化在线详情并触发配置下发。
5. **应用层**：所有键位于 `Payload.fields`，值均为字符串。握手认证没有独立应用字段，应用代码不附加 bearer token 或 API key。

## 失败与重试

| 场景 | 结果 |
|---|---|
| Agent 证书不被信任 | TLS 握手失败，无 RPC 交换 |
| 服务端 DNS 名不匹配 | Agent 自定义校验拒绝连接 |
| AgentCenter 凭据加载失败 | 进程退出 |
| 首包 agent_id 冲突 | `agentID conflict`，已有连接保留 |
| 连接配额超限 | gRPC `UNKNOWN=2`，消息 `out of max connection limit` |
| 流结束或拨号失败 | 约等 5 秒重开；连续获取连接失败超阈值后传输协程退出 |

## Demo 说明

`demo.py` 用 `cryptography` 在内存中生成临时 CA、服务端证书和客户端证书，启动本地双向 TLS 服务，按上述步骤完成握手并发送首包登记。运行：

```powershell
python demo.py
```

脚本验证：受信任客户端完成 mTLS 并登记 `agent_id`、未受信任的客户端证书被握手拒绝、非预期 DNS 名被 Agent 侧拒绝。全部通过输出 `PASS`。

Demo 复现认证边界，不使用真实 gRPC、Kafka 或 AgentCenter 实现，也不验证配置下发与任务分支。
