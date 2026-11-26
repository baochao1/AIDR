# AIDR 端管对接规范 v1

本规范供终端 Agent、管理平台和检测组件研发联调使用。正式端管接口使用 gRPC，服务包名 aidr.endpoint.v1。仓库 Demo 的 /v1/agents/checkin、/v1/agents/events、/v1/artifacts/{id} 继续承担现有回归。正式接入需要新增 gRPC 服务和 Agent 客户端。存储基线为终端 SQLite、平台 PostgreSQL、附件和制品使用私有对象存储。以下默认值作为首轮联调基线。

## 1. 协议、身份与公共类型

| 面 | 链路 | 协议 | 认证 |
|---|---|---|---|
| 本机控制与事件 | Agent 主动连接插件 | gRPC，Windows Named Pipe，Unix Domain Socket，或 TLS 回环 TCP | 原生 IPC ACL、实例 token，TCP 另校验实例证书 |
| 管控面 | Agent 主动连接平台 | gRPC 双向流 Control/Connect，认证接入、心跳和策略推送 | mTLS |
| 数据面 | Agent 主动连接平台 | gRPC 双向流 Telemetry/Report，批量事件及逐条回执 | mTLS |
| 文件面 | Agent 主动上传证据、下载制品 | gRPC，制品服务端流和附件分块 RPC | mTLS |
| 管理操作 | 管理后台调用平台 | HTTPS + JSON | 管理员身份及租户 RBAC |

终端证书 URI SAN 使用 urn:aidr:tenant:{tenant_id}:agent:{agent_id}。平台从已验证证书提取身份并查终端注册状态。注册和签发由部署侧提供，联调用预置证书和终端记录。租户、终端 ID 由平台派生。服务端校验客户端证书，Agent 校验服务端证书和主机名。普通代理可使用 CONNECT 保持端到端 TLS。接口响应须禁用缓存。

字段均按下文名称传输，未标 ? 的字段必填。ID 默认 UUID 字符串，component_id、asset_id、source_event_id、artifact_id 为 1–128 字符的稳定标识。时间为 UTC RFC3339，大小单位为字节，持续时间字段以 _ms 或 _s 标注。int 为非负整数，最大 9007199254740991，具体范围以本规范定义为准。sha256 为 64 位小写十六进制。版本号、错误码和枚举用字符串。请求 JSON 拒绝未知字段和 null，响应新增字段客户端可忽略。新增不兼容语义使用新版本。

JSON 错误响应示例。
**HTTP 400 响应**

~~~json
{
  "code": "INVALID_ARGUMENT",
  "message": "invalid size_bytes",
  "request_id": "req-001"
}
~~~

400 参数或格式错误，401 身份认证失败，403 身份无权限或终端停用，404 资源不存在或不属于当前身份，409 版本冲突、幂等键冲突或文件偏移冲突，410 上传会话过期，413 请求太大，422 内容校验失败，429 限流，503 暂时不可用。网络失败、429 和 5xx 使用 1–60 秒指数退避加随机抖动，遵守 Retry-After。其余错误按接口语义处理，避免无限重试。
·
### Agent 首次关联管理端、认证和重连

v1 使用预置证书接入。部署人员先在平台登记终端，再通过部署侧 PKI 为该终端签发客户端证书，最后将连接配置与证书写入终端。Agent 首次启动无需调用注册接口。

| 部署项 | 谁设置 | 业务说明 |
|---|---|---|
| management_target | 部署侧 | 管理端 gRPC 目标地址，格式为 DNS 主机名:端口，Agent 使用 TLS 发起连接。 |
| server_ca_file | 部署侧 | 供 Agent 校验管理端证书链的 CA 文件。证书还须匹配 management_target 的主机名。 |
| client_cert_file | 部署侧 | 每台终端的客户端证书，其 URI SAN 使用本节定义的租户和终端标识。 |
| client_key_file | 部署侧 | 与客户端证书配对的私钥，仅 Agent 服务身份及部署管理员可读，禁止写入日志。 |
| tenant_id、agent_id、cert_status | 平台 | 预先登记的归属、主键和证书状态。平台在 mTLS 认证后查询登记记录，确认终端已启用。 |

#### 部署配置与启动检查

Windows 将连接配置写入 C:\ProgramData\AIDR\connection.json。下列 JSON 为安装模板。部署时将 MANAGEMENT_HOST 替换为管理端实际 DNS 名称。证书与私钥按配置中的绝对路径落盘。

~~~json
{
  "management_target": "MANAGEMENT_HOST:8443",
  "server_ca_file": "C:\\ProgramData\\AIDR\\certs\\server-ca.pem",
  "client_cert_file": "C:\\ProgramData\\AIDR\\certs\\agent-cert.pem",
  "client_key_file": "C:\\ProgramData\\AIDR\\certs\\agent-key.pem"
}
~~~

1. 平台登记 (tenant_id,agent_id)，将 cert_status 设为启用。部署侧 PKI 签发带客户端认证用途的证书，URI SAN 写入 urn:aidr:tenant:{tenant_id}:agent:{agent_id}。平台的客户端信任库配置该签发 CA。
2. 管理端配置服务端证书，其 DNS SAN 与 management_target 的主机名一致。平台在 TLS 握手阶段要求客户端证书并校验信任链。
3. Agent 启动先检查四个配置字段、证书和私钥是否可读、客户端证书与私钥是否匹配。配置校验失败时记录失败字段与原因，停止平台通信启动流程，等待修正配置并重启服务。
4. 配置检查通过后，按下表执行双向认证和首次心跳。证书和私钥采用 PEM 编码。日志记录认证阶段和失败原因，禁止记录私钥和完整请求正文。

| 顺序 | 交互 | 完成条件 |
|---|---|---|
| 1 | Agent 启动并读取预置配置与证书，主动连接 management_target。 | 地址可达，开始 TLS 握手。 |
| 2 | 双方进行 mTLS 认证。Agent 校验服务端证书、有效期和主机名。平台验证客户端证书链、有效期和客户端用途，读取唯一符合约定的 URI SAN。 | 握手通过后允许建立 gRPC 流。握手失败发生在 HTTP 请求之前，Agent 记录证书或连接错误。 |
| 3 | Agent 调用 Control/Connect，首帧发送 Hello。平台以证书身份查询租户和终端。 | 身份无法提取时关闭流并返回 UNAUTHENTICATED，终端未登记或停用时返回 PERMISSION_DENIED。验证通过后返回 Welcome。 |
| 4 | Agent 收到 Welcome 后发送首条 Heartbeat。 | Agent 收到 Welcome 后确认接入成功。平台接受首条新心跳后标记在线。 |

管控流和数据流都由 Agent 主动发起，可复用同一 gRPC Channel。同一启动周期断网或连接关闭后，Agent 重新建立 mTLS 连接。若上一条心跳响应丢失，按心跳重试规则沿用原 boot_id、seq 和正文。Agent 进程重启时才生成新 boot_id。网络错误或 UNAVAILABLE、RESOURCE_EXHAUSTED 按公共退避规则处理。证书错误或 UNAUTHENTICATED、PERMISSION_DENIED 记录明确原因，等待证书或终端授权修复。断网期间事件继续进入本地队列，重连后的补传走独立事件和文件接口。

#### gRPC 认证材料、请求和响应字段

Agent 主动打开双向流 `/aidr.endpoint.v1.Control/Connect`。认证分为 TLS 握手与端管授权。TLS 握手中，双方出示证书并证明持有配对私钥。Agent 使用 `server_ca_file` 校验管理端证书链、有效期和 `management_target` 中的主机名。管理端验证客户端证书链、有效期和客户端认证用途，从唯一匹配 `urn:aidr:tenant:{tenant_id}:agent:{agent_id}` 的 URI SAN 取出身份，再核对平台记录中的 `cert_status` 和 `active_cert_fingerprint`（客户端叶证书 DER 的 SHA-256）。证书身份来自 TLS，`Hello.body` 无需携带证书、私钥、租户 ID、终端 ID 或注册 Token。

| 阶段 | Agent 请求字段 | 管理端响应字段 |
|---|---|---|
| 首次接入 | `Frame.kind=Hello`。`body` 包含 `boot_id`（UUID）、`agent_version`（字符串）、`capabilities`（字符串数组）、`received_revision` 和 `applied_revision`（整数，初始均为 0）。开流后 10 秒内发送。 | `Frame.kind=Welcome`。`body` 包含 `connection_id`（UUID）、`server_time`（UTC RFC3339）、`heartbeat_interval_s`、`offline_after_s` 和 `desired_revision`（整数）。 |
| 首次报到 | 收到 `Welcome` 后立即发送 `Heartbeat`，沿用 `Hello` 的 `boot_id`，新启动周期从 `seq=1` 开始。完整字段见 [管控面和心跳](#2-管控面和心跳)。 | `HeartbeatReply.body` 返回 `ack_seq`、`server_time`、`heartbeat_interval_s`、`offline_after_s`、`desired_revision`。平台目标版本高于 Agent 已接收版本时，附带 `policy`。 |
| 同一进程断线重连 | 重新进行 mTLS 并打开 `Control/Connect`，发送同一结构的 `Hello`，沿用原 `boot_id`。上一条心跳响应丢失时，重发原 `boot_id`、`seq` 和正文。 | 返回同一结构的 `Welcome`，分配新的 `connection_id`。心跳回执用 `ack_seq` 确认对应序号。 |

TLS 握手失败时无法建立 gRPC 流。身份无法提取时返回 `UNAUTHENTICATED`，终端未登记或已停用时返回 `PERMISSION_DENIED`，已被淘汰的 `boot_id` 返回 `FAILED_PRECONDITION`。Agent 进程重启时生成新的 `boot_id`。

#### `Control/Connect` 端点与线上帧

| 项目 | 对接值 |
|---|---|
| 连接目标 | `management_target`，格式为 `DNS主机名:端口`，例如 `management.example.com:8443`。主机名须匹配管理端证书的 DNS SAN。 |
| 服务和方法 | `aidr.endpoint.v1.Control` 服务的 `Connect` 方法，完整路径 `/aidr.endpoint.v1.Control/Connect`。 |
| RPC 签名 | `rpc Connect(stream Frame) returns (stream Frame)`，Agent 主动开启双向流。 |
| 传输和身份 | HTTP/2 上的 gRPC，使用 mTLS。平台从已验证的客户端证书派生身份。 |
| 编码 | Protobuf。`Frame.kind` 是区分大小写的 `string`，字段号 `1`。`Frame.body` 是 `google.protobuf.Struct`，字段号 `2`。`Frame.data` 是 `bytes`，字段号 `3`，本节各帧均留空。 |
| 帧顺序 | Agent 在开流后 10 秒内发送 `Hello`，平台授权通过后返回 `Welcome`，Agent 随即发送 `Heartbeat`，平台以 `HeartbeatReply` 确认。后续帧见[管控面和心跳](#2-管控面和心跳)。 |

#### `Hello` 请求和 `Welcome` 响应字段

| 帧 | `body` 字段 | 类型 | 对接规则 |
|---|---|---|---|
| `Hello` | `boot_id` | UUID 字符串 | 必填。标识本次 Agent 进程启动周期。首次心跳及同一进程重连使用相同值。 |
| `Hello` | `agent_version` | 字符串 | 必填。Agent 当前运行版本，例如 `1.2.0`。 |
| `Hello` | `capabilities` | 字符串数组 | 必填。上报支持的能力，可用值为 `ips`、`av`、`evidence_upload`、`aidr_asset_events`。无能力时传 `[]`。 |
| `Hello` | `received_revision` | 整数 | 必填。Agent 已接收并保存的策略版本，初始为 `0`。 |
| `Hello` | `applied_revision` | 整数 | 必填。Agent 已成功应用的策略版本，初始为 `0`。 |
| `Welcome` | `connection_id` | UUID 字符串 | 必填。平台为本次活动流分配，每次成功重连更新。 |
| `Welcome` | `server_time` | UTC RFC3339 字符串 | 必填。平台生成响应时的时间。 |
| `Welcome` | `heartbeat_interval_s` | 整数，秒 | 必填。常规心跳间隔，默认 `30`。收到 `Welcome` 后立即发送首条心跳。 |
| `Welcome` | `offline_after_s` | 整数，秒 | 必填。离线阈值，默认 `90`，从平台最后一次接受新心跳开始计时。 |
| `Welcome` | `desired_revision` | 整数 | 必填。平台期望终端应用的策略版本，`0` 表示尚未发布。 |

以下 JSON 展示完整 `Frame` 的字段结构。实际 RPC 使用 Protobuf 编码。

~~~json
{
  "kind": "Hello",
  "body": {
    "boot_id": "00000000-0000-4000-8000-000000000001",
    "agent_version": "1.2.0",
    "capabilities": ["ips", "av"],
    "received_revision": 0,
    "applied_revision": 0
  }
}
~~~

~~~json
{
  "kind": "Welcome",
  "body": {
    "connection_id": "00000000-0000-4000-8000-000000000003",
    "server_time": "2026-11-26T02:00:00Z",
    "heartbeat_interval_s": 30,
    "offline_after_s": 90,
    "desired_revision": 0
  }
}
~~~

#### 重连时的字段取值

| 场景 | `Hello.boot_id` | 下一条 `Heartbeat.seq` | 平台处理 |
|---|---|---|---|
| 首次启动或 Agent 进程重启 | 新 UUID | `1` | 接受新启动周期的首条心跳，更新平台在线记录。 |
| 同一进程重连，上一条心跳已收到回执 | 沿用原值 | 最后一条已确认序号加 `1` | 分配新的 `connection_id`，接受新序号后刷新在线时间。 |
| 同一进程重连，上一条心跳的回执丢失 | 沿用原值 | 沿用未确认序号，原样重发心跳正文 | 若该序号已接受，重回 `ack_seq` 和当前策略信息，保留原在线时间和终端快照。若此前尚未接受，按新心跳处理。 |

证书指纹与登记值不匹配属于授权失败，使用 `PERMISSION_DENIED`。`Hello` 字段或消息格式错误使用 `INVALID_ARGUMENT`。对于网络中断和可重试的 `UNAVAILABLE` 或 `RESOURCE_EXHAUSTED`，按 1–60 秒指数退避加随机抖动重连，平台提供等待时间时优先遵守。证书错误和授权失败等待部署材料或平台登记修正。

#### 正式接口、认证产物与状态维护

| 阶段 | 接口与入参 | 成功产物及存储位置 |
|---|---|---|
| 部署登记 | 部署侧登记 tenant_id、agent_id、cert_status，并通过 PKI 签发客户端证书 | 平台 agents 表保存终端登记和启停状态。Windows Agent 从 C:\ProgramData\AIDR\connection.json 读取地址和证书路径。 |
| 传输认证 | TLS 握手。Agent 提交客户端证书并证明持有配对私钥。平台返回服务端证书并证明持有其私钥。 | 双方建立已认证的 TLS 连接。证书及私钥沿用部署文件，无新增凭据文件。 |
| 确认接入 | Control/Connect 的 Hello 和 Welcome 帧。首次建立启动周期时，Heartbeat 的 seq 从 1 开始。 | 平台返回 Welcome 和后续 HeartbeatReply。平台在同一事务更新 agents 的 boot_id、last_seq、last_seen_at 及终端状态快照。 |
| 持续访问 | 后续管控、事件、文件与制品 RPC 均使用同一终端证书。 | 每次新 TLS 连接重新握手。平台对每个业务请求读取已验证的证书身份，复核终端登记和启用状态。 |

终端身份持续凭据为客户端证书和私钥。协议没有登录 Token、Cookie 或登录会话表。Agent 在内存维护当前启动周期的 boot_id、seq 和最近一次心跳结果。平台在线状态由最后一次接受新心跳的 last_seen_at 推导。网络恢复后 Agent 重建 TLS，继续原启动周期；进程重启后生成新 boot_id 并从 seq=1 开始。平台停用终端后，已建立连接上的后续业务调用返回 PERMISSION_DENIED。

#### gRPC 认证接口与可查产物

| RPC 或帧 | 业务作用 | 认证产物与查询位置 |
|---|---|---|
| Control/Connect，Hello → Welcome | 确认 Agent 已通过 mTLS 且证书对应终端已登记并启用。 | 预置证书和私钥位于 connection.json 指定路径。管理端生成 connection_id，在内存将其绑定到证书派生的 tenant_id、agent_id 和活动流。Agent 收到 Welcome 后记录 connection_id。 |
| Control/Connect，Heartbeat → HeartbeatReply | 持续报到并同步策略状态。 | 管理端在 agents 保存 boot_id、last_seq、last_seen_at，在 agent_status 和 component_status 保存策略及组件快照。通过终端详情查询这些记录。 |
| Telemetry/Report，EventBatch → EventBatchReply | 将插件发现与检测结果归档到对应终端。 | 管理端将证书派生的 tenant_id、agent_id 写入 events 表。通过 event_id 查询原始事件和来源，回执用于确认 Agent 可以出队。 |

断线时释放活动流，持久化终端登记和心跳记录。重连重新执行 mTLS 和 Hello，平台分配新的 connection_id。现有可运行演示仍使用 HTTPS/mTLS 历史接口，见 [认证注册演示指南](aidr-enrollment-demo.md)。以上为正式 gRPC 设计，待实现及验收。

#### 终端唯一性和网络地址

| 字段或约束 | 来源 | 业务用途 |
|---|---|---|
| tenant_id + agent_id | 平台登记后写入客户端证书 URI SAN | 两者组成终端主键。同一个 agent_id 可出现在不同租户。同一租户新增另一台终端时必须分配不同的 agent_id，重复登记返回 ALREADY_EXISTS。 |
| active_cert_fingerprint | 平台登记时绑定客户端叶证书 DER 的 SHA-256 | 平台同时核对证书中的终端标识与登记指纹。另一张证书即使写了相同 URI SAN，也不能接管该终端。证书更换须更新登记绑定。 |
| peer_ip | 平台从当前 gRPC 连接的对端地址取得 | 记录平台实际看到的连接来源，用于定位接入和网络故障。经过 NAT 或代理时，该值可能是出口或代理地址。平台不采信 Agent 自填的 peer_ip。 |
| local_ips | Agent 从本机网卡获取，通过 Heartbeat 上报 | IPv4/IPv6 地址字符串数组，用于定位终端网段和排障。无可用地址传空数组；地址变化时随下一条新心跳更新。 |

平台在 agents 表保存最新 peer_ip、local_ips 和 active_cert_fingerprint。IP 变化不改变终端身份，同一出口 IP 可对应多个终端。复制同一客户端证书和私钥会复用同一个认证身份，平台无法仅凭地址证明是两台不同设备。部署时须为每台终端单独登记并发放独立私钥。

#### 平台如何追溯智能体来源

| 要回答的问题 | 字段与来源 | 校验及入库 |
|---|---|---|
| 哪个租户、哪台终端上报？ | tenant_id、agent_id 来自已验证客户端证书的 URI SAN。 | 平台从当前 RPC 的认证上下文提取身份并检查终端登记。事件正文不得覆盖这两个字段。 |
| 哪个插件实例观测到？ | component_id、instance_id 来自 Agent 的插件启动和认证记录。 | Agent 在本机 gRPC 接收后补齐，插件不得自行指定。平台把它们与终端身份一起存入 events。 |
| 发现了哪个智能体？ | aidr.asset.observed 的 asset_id 为插件维护的稳定标识，data.kind 为 agent。display_name 为显示名称。 | 平台用 (tenant_id,agent_id,component_id,asset_id) 定位资产。同名资产可以分别存在，跨终端归并需要权威部署标识。 |
| 智能体从哪里发现、属于谁？ | data.discovery_source 表示观测方式，data.deployment_ref 可指向部署记录，data.owner_ref 和 data.owner_source 记录负责人及其来源。 | 这些是插件观测或权威目录同步的信息。缺失时省略对应可选字段，平台显示未知。网络源 IP 只用于连接排障。 |

查询某个智能体时，管理端从资产主键定位发现事件，展示所属租户、终端、观测插件、发现方式与部署引用。mTLS 验证的是上报 Agent 的来源。智能体自身身份和部署归属依赖运行时观测或权威部署目录。

#### 与 Elkeid 社区版接入方式对比

对照仓库调研锁定的 Elkeid 提交 c906c512d20c0d56f2b779b8558ed69abae718c8。

| 业务环节 | AIDR 历史 HTTPS Demo | Elkeid 社区版 |
|---|---|---|
| 终端首次纳入管理 | Agent 调用 POST /v1/agents/enroll，平台校验一次性凭据并持久绑定证书指纹。 | Agent 建立带 TLS 客户端凭据的 gRPC 连接。Agent Center 的 Transfer 处理器从首包读取 AgentID，并按该值登记活动连接。这里的登记是流连接登记。 [连接代码](https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/agent/transport/connection/connection.go) · [服务端处理器](https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/server/agent_center/grpctrans/grpc_handler/transfer_handler.go) |
| 心跳、上报与下发 | Agent 周期调用 POST /v1/agents/checkin 并在响应中收取策略。事件走独立的 HTTPS 批量接口。 | Transfer 双向流的上行是 PackagedData，下行是 Command。平台可以通过已登记的活动流发送命令。 [协议定义](https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/agent/proto/grpc.proto) |
| 管理查询 | 管理员调用 GET /v1/admin/agents/{agent_id}，查询已持久化的终端状态。 | Elkeid Manager 提供后台查询及管理 API，Agent Center 处理 Agent 通信。两者按服务划分职责。 [社区版架构说明](https://github.com/bytedance/Elkeid/tree/c906c512d20c0d56f2b779b8558ed69abae718c8#elkeid-backend-ability) |
| 断线恢复 | Agent 重建 mTLS 连接并继续 check-in，服务端从 SQLite 读取注册绑定。 | Transfer 结束时清理活动连接。新流根据首包 AgentID 重新登记。 [服务端处理器](https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/server/agent_center/grpctrans/grpc_handler/transfer_handler.go) |

AIDR 正式端管采用 Agent 主动建立的 mTLS gRPC 流。平台使用证书身份作为活动流索引，支持策略主动推送。上方 HTTPS 接口仅用于历史 Demo。

#### 接入验收

| 操作 | 通过条件 |
|---|---|
| 用已登记终端的有效证书启动 Agent | 证书握手成功，首帧收到 Welcome，首条心跳收到 HeartbeatReply，平台记录的 tenant_id、agent_id 与证书 SAN 一致且终端在线。 |
| 使用不受信任的服务端证书或主机名不匹配的证书 | Agent 终止握手或证书验证流程，平台不接收业务心跳。 |
| 使用受信任 CA 签发但未登记的终端证书 | TLS 握手通过，Connect 返回 PERMISSION_DENIED，平台不创建在线记录。 |
| 禁用已登记的终端 | 新Connect 返回 PERMISSION_DENIED，已停用终端不再刷新最后在线时间。 |
| 断开网络后恢复，同一 Agent 进程持续运行 | Agent 重连并重新完成 mTLS，沿用 boot_id，平台接受下一条新心跳并恢复在线状态。 |

### 正式 gRPC 服务与编码

~~~proto
syntax = "proto3";
package aidr.endpoint.v1;

import "google/protobuf/struct.proto";

message Frame {
  string kind = 1;
  google.protobuf.Struct body = 2;
  bytes data = 3;
}

service Control {
  rpc Connect(stream Frame) returns (stream Frame);
}
service Telemetry {
  rpc Report(stream Frame) returns (stream Frame);
}
service Artifact {
  rpc Download(Frame) returns (stream Frame);
}
service Evidence {
  rpc CreateUpload(Frame) returns (Frame);
  rpc GetUpload(Frame) returns (Frame);
  rpc PutChunk(Frame) returns (Frame);
  rpc Complete(Frame) returns (Frame);
}
~~~

本规范的 JSON 样例展示 Frame.body 字段。原始文件字节走 Frame.data，JSON 展示中的 Base64 仅用于阅读。单个文件数据帧最多 1 MiB，单批事件正文解码后最多 1 MiB。Agent 使用 mTLS 建立 Channel。所有 RPC 都从已验证的客户端证书获取终端身份，流式 RPC 在接受每条业务帧及推送前复核终端启用状态。首次认证成功只产生平台内存态 connection_id，无 Token 或 Cookie。

| RPC | 请求 kind | 响应 kind | body 和交互 |
|---|---|---|---|
| Control/Connect | Hello、Heartbeat | Welcome、HeartbeatReply、PolicyUpdate、ActionDispatch | 首帧及顺序见第 2 节。 |
| Telemetry/Report | EventBatch | EventBatchReply | 第 4 节批次和逐条回执。每条流一次只允许一个未确认批次，断线重发同一批。 |
| Artifact/Download | ArtifactRequest | ArtifactHeader、ArtifactChunk | 请求 body 含 artifact_id。首帧 body 含 descriptor、signature。后续 body 含 offset，data 为 ZIP 数据。正常结束后校验累计长度、签名和哈希。 |
| Evidence/CreateUpload | CreateUploadRequest | UploadSession | 第 5 节创建会话。 |
| Evidence/GetUpload | GetUploadRequest | UploadSession | 请求 body 含 upload_id。 |
| Evidence/PutChunk | UploadChunk | UploadOffset | 请求 body 含 upload_id、offset、chunk_sha256，data 为原始块。 |
| Evidence/Complete | CompleteUploadRequest | UploadComplete | 请求 body 含 upload_id。 |

| gRPC 状态 | 处理含义 |
|---|---|
| UNAUTHENTICATED | 客户端身份不可用 |
| PERMISSION_DENIED | 未登记、停用或权限不足 |
| INVALID_ARGUMENT | 字段或消息格式错误 |
| NOT_FOUND | 资源不存在或当前身份不可见 |
| FAILED_PRECONDITION | 版本冲突、流已被替换、会话过期或分块偏移不一致，具体原因见错误码 |
| DATA_LOSS | 内容完整性校验失败 |
| RESOURCE_EXHAUSTED | 限流或请求超限，按错误码区分 |
| UNAVAILABLE、DEADLINE_EXCEEDED | 暂时不可用或超时，可按原幂等键重试 |

正式 RPC 错误使用 gRPC status，业务错误码和字段通过状态详情传递。临时失败按 1–60 秒指数退避加抖动，平台提供重试等待时间时优先遵守。HTTP 状态码仅适用于管理后台 HTTP 接口和历史 Demo。

## 2. 管控面和心跳

| RPC 或后台 HTTP 接口 | 请求 | 成功响应 |
|---|---|---|
| Control/Connect | Hello、Heartbeat | Welcome、HeartbeatReply、PolicyUpdate、ActionDispatch |
| Artifact/Download | 无 | 服务端流，字段及样例见 Agent 下载、安装和升级插件 |
| GET /admin/agents/{agent_id}/policy | 无 | 200 Policy，可返回 404 |
| PUT /admin/agents/{agent_id}/policy | 见第 3 节发布策略请求 | 200 Policy，当前版本不匹配返回 409 |

#### Connect 首帧与重连

| 帧 | Body 字段 | 处理规则 |
|---|---|---|
| Hello | boot_id:UUID、agent_version:string、capabilities:string[]、received_revision:int、applied_revision:int | Agent 开流后 10 秒内发送。后续 Heartbeat 的 boot_id 必须一致。 |
| Welcome | connection_id:UUID、server_time:UTC RFC3339、heartbeat_interval_s:int、offline_after_s:int、desired_revision:int | 平台认证及授权通过后返回。Agent 收到后立即发送 Heartbeat。 |
| PolicyUpdate | policy:Policy | 平台可在 Welcome 后推送首次缺失策略，也可在新版本发布后主动推送。Agent 按 revision 去重。 |

平台按证书中的 (tenant_id,agent_id) 维护一个有效管控流。替换活动流时分配新 connection_id，使旧流失效。旧流的清理只移除自己的 connection_id。重新连接允许沿用当前 boot_id，已被淘汰的 boot_id 返回 FAILED_PRECONDITION。平台持久保存登记、最后有效心跳和序号，活动流只存在于内存。每个心跳回执的 Frame.body 携带 ack_seq，Agent 用它对齐已发送心跳。

Heartbeat

| 字段 | 类型及定义 |
|---|---|
| boot_id | UUID，每次 Agent 进程启动生成 |
| seq | int，从 1 递增，同一 boot_id 重试复用原序号和正文 |
| sent_at | 时间 |
| agent_version | string |
| os, arch | string，如 windows、amd64 |
| local_ips | string[]，Agent 本机网卡地址，用于网络定位和排障 |
| received_revision, applied_revision | int，初始均为 0 |
| apply_status | none（暂无应用结果）、applying（正在应用）、applied（已生效）、failed（应用失败） |
| apply_error_code?, apply_error_message? | string，failed 时必填，消息最多 256 字符 |
| capabilities | string[]，值为 ips（入侵防护）、av（恶意文件检测）、evidence_upload（证据上传）、aidr_asset_events（AI 资产事件） |
| queue | 见下方请求中的 queue 对象。oldest_event_at 在有积压时填写，丢弃计数自本次启动累计 |
| components | ComponentStatus[]，component_id 唯一 |

### ComponentStatus

Heartbeat 的 components 数组中，每个元素表示一个插件组件的实际状态。标为可选的字段在无对应信息时省略。

| 字段 | 类型 | 必填 | 业务说明 |
|---|---|---|---|
| component_id | string | 是 | 对应策略中组件的 id，平台据此定位插件 |
| instance_id | UUID | 否 | 标识本次插件进程实例，便于关联重启和事件 |
| actual_version | string | 否 | 端上实际组件版本，用于核对升级是否完成 |
| state | string | 是 | 进程生命周期状态，取 absent（未安装）、starting（启动中）、running（运行中）、stopping（停止中）、stopped（已停止）、failed（运行失败） |
| health | string | 是 | 组件健康探测结果，取 unknown（未获探测结论）、serving（可提供服务）、not_serving（组件明确报告不可服务）、unreachable（连续探测失败） |
| data_state | string | 是 | 数据通道状态，取 idle（无待处理事件）、streaming（流已建立并在处理）、backpressured（接收端背压）、disconnected（流已断开）。idle 表示没有待处理事件 |
| applied_revision | int | 是 | 此组件已生效的策略版本，初始为 0 |
| last_probe_at | UTC RFC3339 | 否 | 最近一次本机健康探测时间 |
| last_event_at | UTC RFC3339 | 否 | 最近一次收到该组件事件的时间 |
| error_code | string | 否 | 当前组件异常的机器可读错误码 |
| error_message | string | 否 | 当前组件异常摘要，用于排障 |

~~~json
{
  "component_id": "aidr",
  "instance_id": "00000000-0000-4000-8000-000000000002",
  "actual_version": "1.2.0",
  "state": "running",
  "health": "serving",
  "data_state": "streaming",
  "applied_revision": 6,
  "last_probe_at": "2026-11-26T02:00:00Z",
  "last_event_at": "2026-11-26T02:00:00Z"
}
~~~

Heartbeat 帧 body 的完整内容如下。
**Heartbeat 请求**

~~~json
{
  "boot_id": "00000000-0000-4000-8000-000000000001",
  "seq": 1,
  "sent_at": "2026-11-26T02:00:00Z",
  "agent_version": "1.2.0",
  "os": "windows",
  "arch": "amd64",
  "local_ips": ["192.0.2.20"],
  "received_revision": 6,
  "applied_revision": 6,
  "apply_status": "applied",
  "capabilities": [
    "ips",
    "av"
  ],
  "queue": {
    "pending_events": 1,
    "pending_bytes": 782,
    "pending_files": 0,
    "dropped_events_total": 0,
    "oldest_event_at": "2026-11-26T02:00:00Z"
  },
  "components": [
    {
      "component_id": "aidr",
      "instance_id": "00000000-0000-4000-8000-000000000002",
      "actual_version": "1.2.0",
      "state": "running",
      "health": "serving",
      "data_state": "streaming",
      "applied_revision": 6,
      "last_probe_at": "2026-11-26T02:00:00Z",
      "last_event_at": "2026-11-26T02:00:00Z"
    }
  ]
}
~~~

### HeartbeatReply

管理端处理心跳后沿 Control/Connect 返回 HeartbeatReply 帧。Agent 用它安排下一次心跳，并判断是否收到新的目标策略。

| 字段 | 类型 | 必填 | 业务说明 |
|---|---|---|---|
| ack_seq | int | 是 | 被确认的心跳序号，用于关联请求。 |
| server_time | UTC RFC3339 | 是 | 平台生成响应时的当前时间，供 Agent 估算与平台的时钟偏差。例如判断平台下发的处置任务是否已经过期。平台另用自身记录的心跳接收时间判定终端离线。 |
| heartbeat_interval_s | int，秒 | 是 | 平台要求的常规心跳间隔。Agent 依据该值安排下一条定时心跳，并加入 ±10% 随机抖动。默认 30 秒，实际定时间隔为 27–33 秒。 |
| offline_after_s | int，秒 | 是 | 平台判定终端离线的阈值。从平台最后一次接受新心跳开始计时，到达该阈值即标记离线。默认 90 秒，固定为心跳间隔的 3 倍。 |
| desired_revision | int | 是 | 平台希望该终端应用的目标策略版本，0 表示尚未发布。Agent 用它与自己的 received_revision 比较，判断是否有尚未收到的版本。 |
| policy | Policy 对象 | 条件必填 | 完整目标策略，包含组件启停、制品版本、采集与防护配置等。desired_revision 大于请求中的 received_revision 时携带，且 policy.revision 等于 desired_revision。Agent 校验并保存后更新 received_revision，实际应用成功后再更新 applied_revision。 |

例如终端上报已收到版本 6，平台目标版本为 7，响应带上版本 7 的完整策略。终端保存后，即使应用还在进行，下一条心跳也会报告已收到版本 7，平台据此省略策略正文。
**HeartbeatReply 响应**

~~~json
{
  "ack_seq": 1,
  "server_time": "2026-11-26T02:00:00Z",
  "heartbeat_interval_s": 30,
  "offline_after_s": 90,
  "desired_revision": 7,
  "policy": {
    "revision": 7,
    "issued_at": "2026-11-26T02:00:00Z",
    "heartbeat_interval_s": 30,
    "reporting": {
      "flush_interval_s": 10,
      "max_batch_events": 200,
      "queue_max_bytes": 268435456
    },
    "evidence": {
      "enabled": false,
      "allowed_kinds": [],
      "max_file_bytes": 52428800
    },
    "components": [
      {
        "id": "aidr",
        "desired_state": "running",
        "version": "1.2.0",
        "artifact_id": "aidr-win-120",
        "config": {
          "asset_collection": {
            "enabled": true,
            "log_categories": [
              "security"
            ],
            "redaction_profile": "strict"
          },
          "ips": {
            "enabled": true,
            "mode": "detect",
            "rule_artifact_id": "ips-rules-12"
          },
          "av": {
            "enabled": true,
            "realtime": true,
            "signature_artifact_id": "av-signatures-30",
            "schedule": {
              "enabled": true,
              "hour_utc": 2,
              "minute_utc": 30
            },
            "on_detection": "report"
          }
        }
      }
    ]
  }
}
~~~

默认 30 秒心跳、90 秒离线，心跳带 ±10% 抖动。启动和策略进入 applied/failed 后立即心跳。平台使用首次接受的新心跳的服务端接收时间计算在线状态，已接受序号重试只重新返回当前策略，不刷新在线时间或覆盖快照。Agent 同时只有一个心跳在途，每次启动以 seq=1 登记 boot_id。平台记录已见 boot_id，切换后对旧 boot_id 返回 FAILED_PRECONDITION（STALE_BOOT），旧序号不得覆盖新快照。平台在 desired_revision > received_revision 时返回完整策略。

本机健康每 10 秒调用 /grpc.health.v1.Health/Check，空 service，超时 2 秒。插件完成初始化且可提供能力时回 SERVING，明确不可服务为 not_serving，连续三次调用失败为 unreachable，下一次成功立即恢复。组件处于 stopped/absent 时记录 unknown。平台分别展示终端在线、进程状态、探针状态和数据通道状态。网关提供 GET /livez（进程可响应即 200）与 GET /readyz（数据库可用且已加载认证配置即 200，否则 503）。文件入口独立检查对象存储可用性，失败返回 503。

## 3. 策略及应用回执

### PolicyDraft（管理员提交的策略内容）

PolicyDraft 是发布策略请求中 policy 字段的类型，包含 heartbeat_interval_s、reporting、evidence、components 四个顶层字段。管理员提交这些配置，平台负责生成版本号和发布时间。

| 字段 | 类型 | 业务含义 |
|---|---|---|
| heartbeat_interval_s | int，秒 | 终端常规心跳间隔 |
| reporting | object | 事件上报频率、批次大小和离线队列容量 |
| evidence | object | 证据上传开关、允许的文件类别和大小 |
| components | array | 每个受管组件的目标状态、安装版本、制品和业务配置 |

以下 JSON 就是一份完整的 PolicyDraft，可以作为发布策略请求中 policy 字段的值。

~~~json
{
  "heartbeat_interval_s": 30,
  "reporting": {"flush_interval_s": 10, "max_batch_events": 200, "queue_max_bytes": 268435456},
  "evidence": {"enabled": false, "allowed_kinds": [], "max_file_bytes": 52428800},
  "components": [{
    "id": "aidr", "desired_state": "running", "version": "1.2.0", "artifact_id": "aidr-win-120",
    "config": {
      "asset_collection": {"enabled": true, "log_categories": ["security"], "redaction_profile": "strict"},
      "ips": {"enabled": true, "mode": "detect", "rule_artifact_id": "ips-rules-12"},
      "av": {"enabled": true, "realtime": true, "signature_artifact_id": "av-signatures-30",
             "schedule": {"enabled": true, "hour_utc": 2, "minute_utc": 30},
             "on_detection": "report"}
    }
  }]
}
~~~


**PUT /admin/agents/{agent_id}/policy 请求**

~~~json
{
  "expected_revision": 6,
  "policy": {
    "heartbeat_interval_s": 30,
    "reporting": {
      "flush_interval_s": 10,
      "max_batch_events": 200,
      "queue_max_bytes": 268435456
    },
    "evidence": {
      "enabled": false,
      "allowed_kinds": [],
      "max_file_bytes": 52428800
    },
    "components": [
      {
        "id": "aidr",
        "desired_state": "running",
        "version": "1.2.0",
        "artifact_id": "aidr-win-120",
        "config": {
          "asset_collection": {
            "enabled": true,
            "log_categories": [
              "security"
            ],
            "redaction_profile": "strict"
          },
          "ips": {
            "enabled": true,
            "mode": "detect",
            "rule_artifact_id": "ips-rules-12"
          },
          "av": {
            "enabled": true,
            "realtime": true,
            "signature_artifact_id": "av-signatures-30",
            "schedule": {
              "enabled": true,
              "hour_utc": 2,
              "minute_utc": 30
            },
            "on_detection": "report"
          }
        }
      }
    ]
  }
}
~~~

### Policy（平台已发布的策略）

Policy 包含上面 PolicyDraft 的四个字段，并由平台补齐以下两个字段。PUT 成功响应、GET 策略查询响应和 HeartbeatReply 的 policy 字段都使用这个结构。完整示例见第 2 节 HeartbeatReply。

| 字段 | 类型 | 业务含义 |
|---|---|---|
| revision | int | 平台为本次发布分配的版本号，按租户和终端递增。已发布版本从 1 开始，0 表示当前尚无已发布策略。 |
| issued_at | UTC RFC3339 | 平台发布该版本的时间，用于查询和审计。 |

下表的字段路径相对于 Policy 对象。components[] 表示数组中任意一个组件条目。第 B.3 节的 ai_runtime 为建议扩展，其字段列在同一处供对接。

| 配置字段 | 类型 | 必填条件 | 约束与业务含义 |
|---|---|---|---|
| heartbeat_interval_s | int | 必填 | 常规心跳间隔，单位秒，范围 10–300。离线阈值为其 3 倍。 |
| reporting | object | 必填 | Agent 向平台批量上报和保存离线事件的配置。 |
| reporting.flush_interval_s | int | 必填 | 上报刷新间隔，单位秒，范围 1–60。 |
| reporting.max_batch_events | int | 必填 | 每批最大事件数，范围 1–200。 |
| reporting.queue_max_bytes | int | 必填 | 本地事件队列容量，范围 16 MiB–1 GiB。缩容保留已入队数据，降至限额后恢复接收。 |
| evidence | object | 必填 | 控制 Agent 可以上传哪些证据、文件可以有多大。 |
| evidence.enabled | boolean | 必填 | true（开启证据上传）、false（关闭证据上传，默认）。 |
| evidence.allowed_kinds | string[] | 必填 | diagnostic_log（诊断日志）、scan_report（扫描报告）、quarantined_sample（隔离样本）。空数组表示没有允许的类别。隔离样本须经租户显式授权。 |
| evidence.max_file_bytes | int | 必填 | 单份证据大小上限，单位字节，范围 1 字节–50 MiB。 |
| components | object[] | 必填 | 终端受管组件的目标状态列表。完整快照应显式声明已纳管组件的目标状态。 |
| components[].id | string | 必填 | 组件稳定标识，例如 aidr，同一快照中唯一。 |
| components[].desired_state | enum | 必填 | running（安装并运行）、stopped（保留安装并停止）、absent（卸载）。absent 条目只传 id 和 desired_state。 |
| components[].version | string | running 或 stopped 时必填 | 目标组件程序版本，例如 1.2.0。 |
| components[].artifact_id | string | running 或 stopped 时必填 | 目标组件安装包的制品 ID，用于下载和验签。 |
| components[].config | object | running 或 stopped 时必填 | 交给插件应用的业务配置。以下列出 AIDR 样例中的配置结构。 |
| components[].config.asset_collection | object | AIDR 配置必填 | 控制 AI 资产发现、日志类别及脱敏。 |
| components[].config.asset_collection.enabled | boolean | 必填 | true（开启 AI 工具、智能体和 MCP 服务等资产发现）、false（关闭资产发现）。 |
| components[].config.asset_collection.log_categories | string[] | 必填 | security（安全日志，如风险告警和策略拒绝）、operation（运行操作日志，如组件启停和配置应用结果）。可同时选择，空数组关闭日志采集。 |
| components[].config.asset_collection.redaction_profile | enum | 必填 | v1 仅支持 strict（严格脱敏）。过滤凭据、Token 和用户原始内容，仅上报允许的元数据和脱敏摘要。 |
| components[].config.ips | object | AIDR 的 IPS 配置项 | 控制 IPS 开关、运行模式和规则来源。平台按组件能力校验。 |
| components[].config.ips.enabled | boolean | 提供 ips 时必填 | true（启用 IPS）、false（停用 IPS）。 |
| components[].config.ips.mode | enum | 提供 ips 时必填 | detect（检测并记录）、block（检测并尝试阻断）。 |
| components[].config.ips.rule_artifact_id | string | ips.enabled 为 true 时必填 | IPS 规则的签名制品 ID，Agent 验签后预置。 |
| components[].config.av | object | AIDR 的 AV 配置项 | 控制 AV 开关、扫描触发、签名库和命中处置。平台按组件能力校验。 |
| components[].config.av.enabled | boolean | 提供 av 时必填 | true（启用 AV）、false（停用 AV）。 |
| components[].config.av.realtime | boolean | 提供 av 时必填 | true（开启实时扫描）、false（关闭实时扫描）。 |
| components[].config.av.signature_artifact_id | string | av.enabled 为 true 时必填 | 签名库的签名制品 ID，Agent 验签后预置。 |
| components[].config.av.schedule | object | 提供 av 时必填 | 每日定时扫描配置，与实时扫描分别控制。 |
| components[].config.av.schedule.enabled | boolean | 提供 schedule 时必填 | true（开启每日扫描）、false（关闭每日扫描）。 |
| components[].config.av.schedule.hour_utc | int | 提供 schedule 时必填 | 每日扫描的 UTC 小时，范围 0–23。关闭定时扫描时该值不触发任务。 |
| components[].config.av.schedule.minute_utc | int | 提供 schedule 时必填 | 每日扫描的 UTC 分钟，范围 0–59。关闭定时扫描时该值不触发任务。 |
| components[].config.av.on_detection | enum | 提供 av 时必填 | report（上报检测结果）、quarantine（尝试隔离命中文件）。 |
| components[].config.ai_runtime | object | 建议扩展 | 控制 AI 运行时的检查点、执行模式和处置能力。 |
| components[].config.ai_runtime.mode | enum | 提供 ai_runtime 时必填 | audit（记录判定）、enforce（在受支持的拦截点执行策略）。 |
| components[].config.ai_runtime.enabled_checks | string[] | 提供 ai_runtime 时必填 | prompt（提示词检查）、intent（意图偏离检查）、data（数据保护检查）、access（访问权限检查）。 |
| components[].config.ai_runtime.rules_artifact_id | string | 提供 ai_runtime 时必填 | AI 运行时规则的制品 ID。 |
| components[].config.ai_runtime.enforcement_hooks | string[] | 提供 ai_runtime 时必填 | model_request（模型请求）、tool_call（工具调用）、data_egress（数据出口）。 |
| components[].config.ai_runtime.on_local_error | enum | 提供 ai_runtime 时必填 | allow（本机检查故障时放行）、block（本机检查故障时阻断）。 |
| components[].config.ai_runtime.response_actions | string[] | 提供 ai_runtime 时必填 | pause_agent（暂停智能体）、revoke_token（撤销 Token）、revoke_tool_permission（撤销工具权限）。 |

平台按租户与终端能力校验配置，制品下载限于终端已授权版本。允许列表中的配置项及制品引用构成全部下发面。

Agent 校验版本和配置后持久化 received_revision 及完整策略，再下载和验签、检查 OS/架构/协议版本、预置组件及规则，最后执行。沿用现有制品验签和包检查。重复 revision 幂等，旧 revision 忽略，重启从持久状态恢复。瞬态应用错误按退避重试，永久错误回传 UNSUPPORTED_CONFIG、ARTIFACT_INVALID 或 CONFIG_REJECTED，等待更高版本。平台回滚通过新 revision 发布旧配置。Agent 保留上一有效快照和制品供恢复，全部组件及 Agent 配置完成后推进 applied_revision。部分失败尝试回退已变更组件，回退失败报告 ROLLBACK_FAILED 和组件实际 revision、状态。卸载仅在成功切换后回收旧制品。

### Agent 下载、安装和升级插件

平台通过 Policy 的 components[] 指定目标组件。Agent 以 id 找到本机安装记录，以 version 和 artifact_id 判断是否换包。仅 config 变化且当前安装包校验通过时，沿用安装目录并调用 Configuration/Apply。相同 version 对应的新 artifact_id 也按换包处理。

#### 1. 下载：Agent 主动向制品仓库取包

Agent 收到并保存策略后，通过 mTLS 请求策略中的 artifact_id。请求提供 artifact_id。

~~~json
{
  "artifact_id": "aidr-win-120"
}
~~~

| 响应字段 | 类型 | 业务说明 |
|---|---|---|
| descriptor.artifact_id | string | 制品编号，必须与请求路径和策略目标一致 |
| descriptor.manifest.id | string | 安装后归属的组件 ID，须匹配 components[].id |
| descriptor.manifest.version | string | 包内程序版本，须匹配 components[].version |
| descriptor.manifest.os | string | 支持的操作系统，例如 windows |
| descriptor.manifest.arch | string | 支持的 CPU 架构，例如 amd64 |
| descriptor.manifest.entrypoint | string | ZIP 根目录下的可执行文件相对路径，例如 aidr.exe |
| descriptor.manifest.runtime | string | 运行方式，当前为 process，由 Agent 启动并监管 |
| descriptor.manifest.protocol_version | int | 本机协议版本。1 对应 Demo，2 对应本文的正式事件回执和 Apply |
| descriptor.manifest.ipc_transport | string，可选 | native（原生 IPC）、tcp（TLS 回环 TCP）。省略时沿用 Agent 默认值 |
| descriptor.sha256 | string | ZIP 原始字节的 SHA-256 |
| descriptor.size | int | ZIP 解码后的字节数 |
| descriptor.key_id | string | Agent 本地信任库中用于验签的发布公钥编号 |
| signature | Base64 字符串 | 制品发布者对 descriptor 的 Ed25519 签名 |
| zip | Base64 字符串 | 插件 ZIP 原始字节的编码 |

~~~json
{
  "descriptor": {
    "artifact_id": "aidr-win-120",
    "manifest": {
      "id": "aidr",
      "version": "1.2.0",
      "os": "windows",
      "arch": "amd64",
      "entrypoint": "aidr.exe",
      "runtime": "process",
      "protocol_version": 2,
      "ipc_transport": "native"
    },
    "sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    "size": 3,
    "key_id": "release-2026"
  },
  "signature": "AA==",
  "zip": "YWJj"
}
~~~

上例展示制品描述和 ZIP 内容的可读结构。正式 Download 首帧只发 descriptor 和 signature，后续按序发送 ArtifactChunk。签名、哈希和 ZIP 是占位值，验签联调使用发布工具生成的真实制品。签名输入沿用仓库 Descriptor 结构的 JSON 序列化字节，使用紧凑 UTF-8、按上表 descriptor 字段及 manifest 字段的顺序编码，省略空 ipc_transport，字符串转义与 Go encoding/json.Marshal 一致。签名公钥由部署侧预置。平台校验终端是否有权下载该不可变制品。NOT_FOUND 表示不存在或当前终端无权获取，Agent 回报 ARTIFACT_MISSING。网络失败、RESOURCE_EXHAUSTED 或 UNAVAILABLE 按公共退避规则重新发起完整 Download。Agent 对描述帧、分块及累计 ZIP 字节设置大小上限，先验签，再核对目标组件、版本、系统、架构、协议、ZIP 长度与哈希。任一校验失败回报 ARTIFACT_INVALID。

#### 2. 落盘：新版本独立放置

Windows 默认根目录为 C:\ProgramData\AIDR，可由部署配置覆盖。下表路径相对于该根目录。

| 本地路径 | 用途 |
|---|---|
| downloads\<artifact_id>.zip.part | 接收下载内容时的临时文件，下载失败后清理，重试重新获取 |
| plugins\<component_id>\.stage-<uuid>\ | 同卷的安装暂存目录，解压并验证完整后才发布 |
| plugins\<component_id>\<artifact_id>\ | 以制品 ID 区分的不可变安装目录，例如 plugins\aidr\aidr-win-120\aidr.exe |
| plugin-data\<component_id>\ | 插件运行数据目录，独立于安装包，升级保留。涉及数据迁移时组件必须保证上一版本可读或提供可恢复备份 |
| config-artifacts\<component_id>\ | 规则包和签名库交接目录，按下节的 Apply 约定使用 |

Agent 校验解压路径留在暂存目录内，拒绝绝对路径、目录穿越、链接、重解析点、重复路径和超出展开配额的包。确认 entrypoint 是安装目录内的普通文件后，在同一卷原子重命名暂存目录。当前运行的版本继续使用原目录，升级期间保留上一个已验证的安装目录。Agent 持久保存组件的 active_artifact_id、previous_artifact_id、target_artifact_id、revision 和 cutover_phase，用于重启恢复与回退。插件执行身份对安装目录只读，对自己的运行数据目录可写。

#### 3. 切换：Agent 下载完成后才停旧进程

| 步骤 | Agent 行为与确认条件 |
|---|---|
| 准备 | 校验并发布新安装目录，同时准备目标配置所需的规则制品。准备失败时保持当前进程运行。 |
| 停旧 | 对 running 目标，将切换阶段写入本地记录后停止旧实例。停止前允许插件完成已在途的本地事件交接。已进入 Agent 持久队列的数据继续上报。停止失败时终止切换并回报错误。 |
| 启新 | 从新安装目录启动 manifest.entrypoint，分配新 instance_id 和本机连接凭据。进程能够接收配置请求后调用 Configuration/Apply。启动探针需要区分 RPC 可达和业务已就绪。 |
| 验证并提交 | Apply 成功并且 Health 返回 SERVING 后建立 Events 流，确认新实例可接收事件连接，再提交 active_artifact_id 和实际运行版本。所有策略变更完成后，心跳推进 applied_revision。 |
| 回退 | 新进程启动、加载配置、健康或建流失败时，Agent 停止新实例，从上一有效安装目录和配置启动旧实例并验证。目标 revision 标记 failed，心跳报告实际版本及失败原因。首次安装失败时没有旧实例可恢复，保留诊断并报告 failed。旧版本也启动失败时报告 ROLLBACK_FAILED。 |
| 恢复与清理 | Agent 重启时根据持久化切换阶段和实际实例状态继续验证目标，或恢复上一次有效版本，避免同时运行两个实例。清理仅针对未被当前、待应用或回退记录引用的安装包。 |

目标为 stopped 时可以准备并登记安装版本，保持进程停止。下次启动时应用已保存配置，再开放事件采集。目标为 absent 时停止实例并卸载，安装目录在提交成功且无回退引用后清理。

仓库 Demo 的制品下载入口为 /v1/artifacts/{id}。本节规定正式端管路径和升级事务边界。

### Agent 向本机插件应用配置

平台通过心跳把 Policy 下发给 Agent。Agent 负责安装和启停组件，并通过本机 gRPC /aidr.component.v1.Configuration/Apply 把该组件的业务配置交给插件。每次调用有一个请求和一个响应，两者都是 google.protobuf.StringValue，value 是 UTF-8 JSON 文本。该方法使用与本机 Health 和 Events 相同的实例认证。

| 步骤 | Agent 和插件各自的职责 |
|---|---|
| 1. 解析引用 | Agent 从目标组件的 config 中读取 IPS 规则、AV 签名库、AI 运行时规则的 artifact_id，按实际启用项准备制品。 |
| 2. 准备文件 | Agent 从平台下载制品、验签并校验包哈希。通过后，将完整 ZIP 存入该组件专用的只读交接目录。先写临时文件再原子发布，使用制品 SHA-256 命名。不在此步骤解压。 |
| 3. 交付路径 | Agent 把 config 和 resolved_artifacts 一起发给插件。config 保留平台下发的制品 ID，resolved_artifacts 提供该 ID 对应的本机绝对路径、包大小和 SHA-256。插件使用 Agent 提供的路径打开 ZIP。 |
| 4. 加载配置 | 插件检查启用项引用都有唯一对应的制品，校验文件长度、SHA-256 和规则兼容性。在插件私有工作区加载并验证新配置，全部成功后原子切换。 |
| 5. 报告结果 | 插件返回 applied 或 rejected。Agent 收到 applied 后再检查组件健康，并在该策略所有变更完成后通过心跳推进 applied_revision。发生拒绝或加载失败时保留上一有效配置，并回报原因。 |

Agent 为交接目录设置权限。Agent 可写，目标插件运行身份只读，其他组件和普通用户无权限。Agent 启动插件时通过 AIDR_CONFIG_ARTIFACT_ROOT 提供该目录绝对路径，插件须确认 local_path 解析后仍在该目录下，并拒绝符号链接或重解析点。制品在该版本仍生效、正在应用或需要回退时保留。插件需要解压时使用自己的工作目录，并拒绝越过目标目录的 ZIP 条目。

| 请求字段 | 类型 | 必填 | 业务说明 |
|---|---|---|---|
| component_id | string | 是 | 被配置的本机插件组件 ID，必须与当前实例一致。 |
| revision | int | 是 | 平台下发的策略版本，用于应用确认和幂等重试。 |
| config | object | 是 | 该组件的业务配置，对应 Policy 中 components[].config。 |
| resolved_artifacts | object[] | 是 | 本配置所需的本机制品映射；没有制品依赖时传空数组。同一 artifact_id 只允许一条。 |
| resolved_artifacts[].artifact_id | string | 是 | 与 config 中的制品引用匹配。 |
| resolved_artifacts[].local_path | string | 是 | Agent 交付的只读 ZIP 绝对路径。 |
| resolved_artifacts[].size_bytes | int | 是 | 完整 ZIP 的字节数。 |
| resolved_artifacts[].sha256 | string | 是 | 完整 ZIP 的 SHA-256，与平台已验签描述中的哈希一致。 |

| 响应字段 | 类型 | 必填 | 业务说明 |
|---|---|---|---|
| revision | int | 是 | 原样回显请求版本。 |
| status | enum | 是 | applied（配置及制品已切换生效）、rejected（本次配置拒绝，保留旧配置）。 |
| error_code | enum | rejected 时必填 | ARTIFACT_MISSING（引用未解析或文件无法读取）、ARTIFACT_INVALID（文件校验失败）、UNSUPPORTED_CONFIG（规则或配置与组件能力不兼容）、CONFIG_REJECTED（配置校验或加载失败）。 |
| error_message | string | rejected 时必填 | 最多 256 字符的脱敏失败摘要。 |
**Configuration/Apply 请求的 StringValue.value**

~~~json
{
  "component_id": "aidr",
  "revision": 7,
  "config": {
    "asset_collection": {
      "enabled": true,
      "log_categories": [
        "security"
      ],
      "redaction_profile": "strict"
    },
    "ips": {
      "enabled": true,
      "mode": "detect",
      "rule_artifact_id": "ips-rules-12"
    },
    "av": {
      "enabled": true,
      "realtime": true,
      "signature_artifact_id": "av-signatures-30",
      "schedule": {
        "enabled": true,
        "hour_utc": 2,
        "minute_utc": 30
      },
      "on_detection": "report"
    }
  },
  "resolved_artifacts": [
    {
      "local_path": "C:\\ProgramData\\AIDR\\config-artifacts\\aidr\\aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.zip",
      "artifact_id": "ips-rules-12",
      "size_bytes": 4096,
      "sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    },
    {
      "local_path": "C:\\ProgramData\\AIDR\\config-artifacts\\aidr\\bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.zip",
      "artifact_id": "av-signatures-30",
      "size_bytes": 8192,
      "sha256": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    }
  ]
}
~~~

**Configuration/Apply 成功响应的 StringValue.value**

~~~json
{
  "revision": 7,
  "status": "applied"
}
~~~

**Configuration/Apply 拒绝响应的 StringValue.value**

~~~json
{
  "revision": 7,
  "status": "rejected",
  "error_code": "CONFIG_REJECTED",
  "error_message": "规则包与引擎版本不兼容"
}
~~~

单次超时 10 秒。组件需持久化并原子切换有效配置，同 revision 和相同 config 已成功时直接返回 applied；上次未成功时允许对相同配置重试。同 revision 但 config 或制品哈希不同则拒绝。若 RPC 超时，Agent 用同一 revision 和配置重试。运行态组件切换后需通过 Health，stopped 组件配置由 Agent 持久化，启动时先下发再采集。

## 4. 数据面、IPS 和 AV

沿用本机 /demo.component.v1.Events/Stream 的 StringValue 双向流和首帧 ready。组件发送下文 Event 的业务字段，各条等待 ACK。正式 Agent 加入每实例认证及 Schema 校验，回执如下。
**已落盘**

~~~json
{
  "status": "accepted",
  "event_id": "6d959129-32bb-4fac-a774-f173254d1fbd"
}
~~~

**永久拒绝**

~~~json
{
  "status": "rejected",
  "error_code": "INVALID_EVENT"
}
~~~

**暂时无法入队**

~~~json
{
  "status": "retry",
  "error_code": "QUEUE_FULL"
}
~~~

该 ACK 格式通过制品 manifest protocol_version=2 区分 Demo 的纯 event_id 回执。插件对未确认事件持久重放，同源事件保持相同 source_event_id 和内容。Agent 可在配置允许范围内校验和脱敏后落盘，事务提交后才 ACK。

| Event 字段 | 定义 |
|---|---|
| type, schema_version | 类型见下表，schema_version=1 |
| source_event_id, observed_at | 插件生成，源事件 ID 在终端和组件范围内跨重启唯一 |
| asset_id? | 资产类事件必填，与同一资产历史一致 |
| data | 对应类型的结构化对象 |
| event_id, component_id, instance_id, sequence, timestamp | Agent 补齐。event_id 为 UUID，sequence 为终端持久化递增序号，从 1 开始，timestamp 为 Agent 接收时间 |

| type | data 字段 |
|---|---|
| ips.result | signature_id:string, signature_name:string, ruleset_version:string, engine_version:string, severity:info/low/medium/high/critical, direction:inbound/outbound, transport:tcp/udp/icmp/other, src_ip:string, dst_ip:string, src_port?:int, dst_port?:int, verdict：detected（已检测到）、blocked（已阻断）、error（检测或处置出错）, error_code?:string |
| av.result | scan_id:UUID, trigger:realtime/scheduled, engine_version:string, signature_version:string, file_sha256?:sha256, file_name?:string, size_bytes?:int, verdict：clean（未发现风险）、malicious（发现恶意内容）、suspicious（可疑）、error（扫描出错）, threat_name?:string, action:none/reported/quarantined, action_status：not_requested（未请求处置）、succeeded（处置成功）、failed（处置失败）, error_code?:string |
| aidr.agent.discovered | name:string, kind:string, version?:string, discovery_source:string |
| aidr.agent.log_observed | agent_asset_id:string, level:info/warn/error, category:security（安全日志）、operation（运行操作日志）, message_summary:string, redacted:true |
| aidr.skill.discovered | agent_asset_id:string, skill_name:string, version?:string, sha256?:sha256, source_kind:string |
| aidr.skill.scan_result | skill_asset_id:string, rule_set_version:string, verdict:clean/suspicious/risk/error, severity:info/low/medium/high, findings:{rule_id:string,category:string,summary:string}[] |
| aidr.mcp.discovered | agent_asset_id:string, server_name:string, transport:stdio/sse/streamable_http/other, source_kind:string |
| aidr.mcp.risk_detected | mcp_asset_id:string, rule_id:string, risk_category:string, severity:info/low/medium/high, summary:string |

IP 字段使用 IPv4/IPv6 字符串，端口限 0–65535，仅 TCP/UDP 输出并必填。IPS blocked 表示执行阻断成功，error 要有 error_code。AV 每个文件产生一条结果，同一次扫描共享 scan_id。malicious 必须给 threat_name，verdict=error 或 action_status=failed 必须给 error_code。action=none 时 action_status=not_requested。file_name 只含脱敏文件名。普通事件只包含结构化元数据及脱敏摘要。单字符串默认最多 256 字符，summary 和 message_summary 最多 1024 字符，findings 最多 50 条。

Telemetry/Report，请求正文见附录 A，至少 1 条，单事件最多 16 KiB，单批不超过 200 条且解压后不超过 1 MiB。服务端逐条校验，一次数据库事务将可接受记录持久化并生成回执。平台沿 Report 流返回回执。
**附录 A 批次的逐条回执**

~~~json
{
  "batch_id": "9cc9e5de-ecc1-4474-97ea-8fc2fb55f01c",
  "results": [
    {
      "event_id": "6d959129-32bb-4fac-a774-f173254d1fbd",
      "status": "accepted"
    },
    {
      "event_id": "4de0a8c4-254d-4137-90e9-21c9709808e3",
      "status": "duplicate"
    }
  ]
}
~~~

| 字段 | 枚举值及含义 |
|---|---|
| results[].status | accepted（本次已持久化）、duplicate（同一事件已持久化）、rejected（该条永久拒收，需查看 code） |

每条输入恰好一个回执，拒收回执包含 code，例如 INVALID_EVENT、ID_CONFLICT。无法解析批信封或识别条目 ID 时整批返回 INVALID_ARGUMENT。事务失败整批返回 UNAVAILABLE。平台按终端和 event_id 去重，同 ID 内容不同返回 ID_CONFLICT，历史事件允许乱序到达。端上只删除 accepted/duplicate 对应待发记录，rejected 转本地死信并计数告警。响应缺失或不完整时保留整批重试。队列离线可持续写入，满时向插件背压。

## 5. 文件通道

组件通过本机事件 aidr.evidence.declared 提交文件上传声明，报文如下。
**本机附件声明**

~~~json
{
  "type": "aidr.evidence.declared",
  "schema_version": 1,
  "source_event_id": "evidence-42",
  "observed_at": "2026-11-26T02:00:00Z",
  "data": {
    "file_id": "00000000-0000-4000-8000-000000000005",
    "related_source_event_id": "ips-42",
    "kind": "scan_report",
    "size_bytes": 3,
    "sha256": "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
  }
}
~~~

该类型由 Agent 本地消费并持久化上传任务，ACK 返回声明 event_id，插件据此释放对声明的重试。插件只可把文件写到实例授权交接目录，以 file_id 为文件名，写临时文件后原子改名。Agent 拒绝目录、链接和重解析点，通过安全打开的句柄复制至私有暂存区，校验大小与 SHA-256。先等待关联事件在平台获得 accepted/duplicate，再创建上传。Agent 从本地源 ID 映射取关联 event_id。

| 方法与路径 | 请求 | 响应 |
|---|---|---|
| Evidence/CreateUpload | 见下方创建请求 | 成功时返回新建或复用的会话，见下方创建响应 |
| Evidence/GetUpload | body 包含 upload_id | 成功，见下方查询响应 |
| Evidence/PutChunk | UploadChunk，字段为 upload_id、offset、chunk_sha256、data | 成功，见下方分块响应，offset 表示下一字节 |
| Evidence/Complete | 请求 body 包含 upload_id，见下方完成请求 | 成功，见下方完成响应 |


**Evidence/CreateUpload 请求**

~~~json
{
  "file_id": "00000000-0000-4000-8000-000000000005",
  "event_id": "6d959129-32bb-4fac-a774-f173254d1fbd",
  "kind": "scan_report",
  "size_bytes": 3,
  "sha256": "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
  "content_type": "text/plain"
}
~~~

**Evidence/CreateUpload 新建响应**

~~~json
{
  "upload_id": "00000000-0000-4000-8000-000000000006",
  "offset": 0,
  "chunk_size": 1048576,
  "expires_at": "2026-11-27T02:00:00Z",
  "status": "uploading"
}
~~~

**Evidence/GetUpload 响应**

~~~json
{
  "offset": 3,
  "size_bytes": 3,
  "status": "uploading",
  "expires_at": "2026-11-27T02:00:00Z",
  "file_id": "00000000-0000-4000-8000-000000000005"
}
~~~

**Evidence/PutChunk 请求**

~~~json
{
  "upload_id": "00000000-0000-4000-8000-000000000006",
  "offset": 0,
  "chunk_sha256": "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
  "data": "YWJj"
}
~~~

**上传分块成功响应**

~~~json
{
  "offset": 3
}
~~~

**Evidence/Complete 请求**

~~~json
{"upload_id":"00000000-0000-4000-8000-000000000006"}
~~~

**文件完成响应**

~~~json
{
  "file_id": "00000000-0000-4000-8000-000000000005",
  "status": "available",
  "sha256": "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
  "size_bytes": 3
}
~~~

**分块偏移冲突响应**

~~~json
{
  "code": "OFFSET_CONFLICT",
  "expected_offset": 1048576,
  "message": "offset mismatch",
  "request_id": "req-002"
}
~~~

| 字段 | 枚举值及含义 |
|---|---|
| status | uploading（会话接收分块中）、available（完整文件通过校验且可读）、expired（会话超时）、failed（完整性校验等原因导致会话失败） |
| content_type | application/octet-stream（二进制）、application/json（JSON 文档）、text/plain（纯文本） |

上传初始 offset=0，chunk_size 固定 1 MiB，末块可短于 1 MiB。按序分块，每个会话串行更新偏移且块持久化后才推进。相同偏移重复且哈希、长度一致返回当前偏移，其他冲突返回 FAILED_PRECONDITION，响应见上面的分块偏移冲突示例。会话 24 小时过期，平台释放暂存空间，Agent 可用原 file_id 重建会话。初始化幂等键为租户、终端、file_id，声明字段冲突返回 409。complete 校验总长度和哈希，失败返回 DATA_LOSS（FILE_INTEGRITY_FAILED） 并终止当前会话。对象写入成功后提交文件元数据并标记 available，重复完成返回同一结果。平台按归属、关联事件、文件类别、租户策略和大小校验上传权限，所有读取由有权限的后台代理并记录审计。Agent 仅在完成确认后清理暂存内容，策略撤销后停止上传。

## 6. 存储和索引

平台所有业务检索按 tenant_id 限定。下表 id 类型沿用协议，时间用 timestamptz，计数和 revision 用 bigint，布尔用 boolean，其余结构化对象用 jsonb。

| 表 | 主键与主要列 | 索引、用途 | 业务说明 |
|---|---|---|---|
| agents | PK(tenant_id,agent_id)，cert_status,active_cert_fingerprint,last_seen_at,boot_id,last_seq,agent_version,os,arch,capabilities,peer_ip,local_ips | (tenant_id,last_seen_at) 在线筛选 | 管理端的终端登记和在线快照，也用于检查证书关联的终端是否有效。 |
| agent_boots | PK(tenant_id,agent_id,boot_id)，first_seen_at | 已见 boot 防止旧实例覆盖 | 记录终端已见过的启动周期，识别迟到的旧进程心跳。 |
| agent_status | PK(tenant_id,agent_id)，received_revision,applied_revision,apply_status,error_code,queue,updated_at | 与 agents 在接收心跳时同事务更新 | 展示终端收到了哪版策略、实际生效到哪版，以及队列积压和错误。 |
| component_status | PK(tenant_id,agent_id,component_id)，instance_id,state,health,data_state,actual_version,applied_revision,last_probe_at,last_event_at,error_code,error_message | 每次新心跳完整替换本终端快照 | 展示每个组件的版本、运行状态、健康探针和数据通道状态。 |
| agent_policies | PK(tenant_id,agent_id,revision)，policy,issued_at,issued_by | 锁 agents 行比较 expected_revision 再插入，最高 revision 为当前目标 | 保留逐版本下发记录，供审计、故障追溯和重新发布历史配置。 |
| events | PK(tenant_id,agent_id,event_id)，component_id,instance_id,source_event_id,type,schema_version,asset_id,sequence,observed_at,agent_timestamp,received_at,data,content_sha256 | UNIQUE(tenant_id,agent_id,component_id,source_event_id)，索引 (tenant_id,type,received_at DESC) 和 (tenant_id,agent_id,asset_id,observed_at DESC) | 保存可追溯的事件原始业务记录，供调查、资产关联和去重。 |
| detection_results | PK/FK(tenant_id,agent_id,event_id)，type,verdict,severity,rule_or_threat,action,scan_id,received_at | (tenant_id,type,verdict,received_at DESC)，与原始事件同事务写入。IPS 提取 signature_id 和 verdict，AV 提取 threat_name、action、scan_id | 保存从原事件提取的检测结果，供 IPS、AV 结果筛选和检索。 |
| files | PK(tenant_id,agent_id,file_id)，event_id,kind,size_bytes,sha256,content_type,object_key,status,completed_at | (tenant_id,agent_id,event_id)，关联同终端事件 | 保存证据元数据和原始事件归属，控制调查人员能否读取对象。 |
| file_uploads | PK(upload_id)，tenant_id,agent_id,file_id,offset,status,expires_at | 同一文件最多一个上传中会话，按 expires_at 清理 | 保存一次文件上传的进度、状态和有效期，供断线续传及超时清理。 |
| file_chunks | PK(upload_id,offset)，size_bytes,sha256,staging_object_key | 用于确认重复分块和合并 | 保存各分块的位置、校验值和暂存对象，供重复块确认和文件合并。 |
| artifacts | PK(tenant_id,artifact_id)，descriptor,signature,object_key,published_at | 制品不可变，签名和包哈希随元数据保存 | 登记可分发的插件程序、规则与签名库，供端侧下载及验签。 |

平台计算 content_sha256 时按 JSON 对象结构比较语义，键顺序和空白变化视为相同，数组顺序保留。若源 ID 唯一键冲突且 event_id 不同，返回 ID_CONFLICT。终端 SQLite 建议 WAL、synchronous=FULL，持久化下列数据。

| 表 | 键和字段 | 业务说明 |
|---|---|---|
| outbox | event_id PK，sequence UNIQUE，event_json,queued_at,attempts,next_attempt_at，索引(next_attempt_at,sequence) | 端侧待发事件队列。保存离线积压和重试进度，平台确认后按 event_id 出队。 |
| source_events | PK(component_id,source_event_id)，event_id,source_digest,delivery_state,created_at，用于插件重放及文件关联 | 保存插件源事件到 Agent 事件的映射。插件重放时复用原编号，证据可据此找到关联的平台事件。 |
| policy_state | 单行，received_policy,active_policy,received_revision,applied_revision,apply_status,last_error | 保存待应用策略、已生效策略和失败原因。Agent 重启后据此续做策略应用或恢复原配置。 |
| local_meta | key PK，value，保存终端事件序列 | 保存终端事件序列，确保重启后继续分配 sequence，供平台判断事件先后。 |
| upload_queue | file_id PK，event_id,staged_path,size_bytes,sha256,kind,content_type,upload_id,offset,state,next_attempt_at | 记录本地证据暂存位置、关联事件和上传会话偏移。断线或重启后按该进度恢复上传。 |
| event_dead_letter | event_id PK，event_json,error_code,failed_at | 保存平台永久拒收的事件及原因，供研发排查结构校验失败或标识冲突。 |

终端在一个事务中分配 sequence、写 outbox 和源 ID 映射。重复源 ID 内容一致返回原 event_id，内容不同永久拒绝。平台回执后同一事务更新关联映射并删除 outbox 条目。建议原始事件及检测结果保留 90 天、附件 30 天、本地死信 7 天。源 ID 映射保留 90 天，仍有待发事件或待上传文件时延长。首轮联调的端上重放窗口为 7 天，超过窗口的积压仍保留并告警，运维清理按租户留存策略执行。过期附件先撤销读取权限再删除对象，过期原始事件清理前须处理关联文件及派生结果。平台清理孤儿暂存对象时以会话状态和过期时间为准。

## 7. 双方联调验收

| 场景 | 预期 |
|---|---|
| 新终端首次心跳、旧心跳乱序 | 平台识别证书所属终端，快照只采纳当前启动的新 seq |
| Agent 停摆超过离线阈值 | 终端显示离线。Agent 在线但插件连续探测失败时独立显示插件不可达 |
| 下发 IPS 从 detect 到 block、AV 从 report 到 quarantine | 平台可见收到和应用两个 revision，分别由 IPS 和 AV 模拟结果验证动作 |
| 制品签名失败、组件拒绝配置 | 应用版本不推进，平台可见错误及组件实际版本 |
| IPS 与 AV 批量上报、断网后恢复 | 数据按结构解析并入库，相同 batch 重传无重复检测记录 |
| 平台提交后响应丢失、Agent 收到回执前重启 | 端上重传，平台按原 event_id 返回 duplicate |
| 附件分块上传中断 | 查询 offset 恢复，完成后文件归属同租户、同终端和原事件 |
| 内容哈希不一致、越权上传、证据禁用 | 文件无法进入 available，平台保留审计记录 |
| 停止或卸载组件 | 平台状态与实际进程一致，Agent 继续发送心跳并处理已有队列 |

## 附录 A. Telemetry/Report 联调报文

~~~json
{
  "batch_id": "9cc9e5de-ecc1-4474-97ea-8fc2fb55f01c",
  "events": [
    {
      "event_id": "6d959129-32bb-4fac-a774-f173254d1fbd",
      "component_id": "aidr",
      "instance_id": "d59559e9-37d2-430c-8f5c-491767500675",
      "sequence": 42,
      "timestamp": "2026-11-26T02:00:01Z",
      "source_event_id": "ips-42",
      "observed_at": "2026-11-26T02:00:00Z",
      "type": "ips.result",
      "schema_version": 1,
      "data": {
        "signature_id": "IPS-001",
        "signature_name": "联调规则命中",
        "ruleset_version": "12",
        "engine_version": "1.2.0",
        "severity": "high",
        "direction": "inbound",
        "transport": "tcp",
        "src_ip": "192.0.2.10",
        "dst_ip": "198.51.100.20",
        "src_port": 53640,
        "dst_port": 443,
        "verdict": "blocked"
      }
    },
    {
      "event_id": "4de0a8c4-254d-4137-90e9-21c9709808e3",
      "component_id": "aidr",
      "instance_id": "d59559e9-37d2-430c-8f5c-491767500675",
      "sequence": 43,
      "timestamp": "2026-11-26T02:00:02Z",
      "source_event_id": "av-43",
      "observed_at": "2026-11-26T02:00:01Z",
      "type": "av.result",
      "schema_version": 1,
      "data": {
        "scan_id": "38414631-89cb-4351-ac44-d2fcd9c1e908",
        "trigger": "realtime",
        "engine_version": "1.2.0",
        "signature_version": "30",
        "file_name": "sample.bin",
        "size_bytes": 512,
        "verdict": "malicious",
        "threat_name": "联调模拟样本",
        "action": "quarantined",
        "action_status": "succeeded"
      }
    }
  ]
}
~~~

## 附录 B. AI 运行时能力的两段数据契约

本附录扩展第 4 节的 Event 类型。插件发送下方本机事件报文。Agent 经本机 gRPC 接收、认证、校验和落盘后返回本地 ACK，并补齐 event_id、component_id、instance_id、sequence、timestamp。Agent 按同一 Telemetry/Report 批量格式上报，平台入库后按 event_id 回执。IPS 和 AV 继续使用各自的结果类型。以下 AI 交互事件使用统一的 trace_id 关联。


**插件上报 AI 资产事件的 StringValue.value**

~~~json
{
  "type": "aidr.asset.observed",
  "schema_version": 1,
  "source_event_id": "asset-42",
  "observed_at": "2026-11-26T02:00:00Z",
  "asset_id": "runtime-42",
  "data": {
    "kind": "agent",
    "display_name": "报表助手",
    "discovery_source": "runtime",
    "authorization": "approved"
  }
}
~~~

### B.1 通用关联结构

- asset_id 为插件持久维护的资产标识。平台资产主键为 (tenant_id,agent_id,component_id,asset_id)。插件重启保留映射。同一实体通过不同探针发现时，平台按授权的外部标识建立关联。
- data.trace_id、data.operation_id 为 UUID。一次 AI 请求或一次自主任务共享 trace_id，每个提示词检查、工具调用、数据流转、运行时响应分配独立 operation_id。parent_operation_id? 为同一 trace 的直接父操作，根节点省略。平台关联键为 (tenant_id,agent_id,component_id,trace_id,operation_id)，逻辑父节点允许后到。
| 字段 | 类型 | 枚举值及业务说明 |
|---|---|---|
| data.policy_revision | int | 作出决策时采用的有效策略版本 |
| data.rule_ids | string[] | 命中的规则编号，可为空 |
| data.decision | enum | allow（策略允许）、block（策略要求阻断）、audit（策略要求记录） |
| data.enforcement | enum | allowed（已放行）、blocked（已阻断）、would_block（审计模式下命中阻断条件）、failed（执行失败）、unknown（实际结果未知） |
| data.error_code | string | 执行失败时必填，标识失败原因 |

旁路只能报告 would_block 或 unknown。

- 用户标识使用租户内可关联的受控引用 principal_ref:string。提示词、工具参数、文件正文、Token 和完整目标 URL 通过事件字段白名单过滤。常规上报使用脱敏摘要、分类、数量和受控引用。所有字符串大小遵循第 4 节。

### B.2 事件类型和字段

| type | 插件 data 必填及可选字段 | 发出时机 | 业务说明 |
|---|---|---|---|
| aidr.asset.observed | kind：ai_tool（AI 工具）、agent（智能体）、mcp_server（MCP 服务器）, display_name:string, discovery_source:inventory/process/browser/runtime, authorization：approved（已获授权）、unapproved（未获授权）、unknown（授权状态未知）, owner_ref?:string, owner_source?:authoritative/inferred, parent_asset_id?:string, deployment_ref?:string | 初次发现或资产状态变化。asset_id 必填，MCP 多关联通过下述关系事件表达 | 登记员工使用的 AI 工具、部署的智能体和 MCP 服务。authorization 表示是否获准使用，owner_ref 用于找到负责人，owner_source 标识归属是权威确认还是推断。 |
| aidr.asset.relationship | target_asset_id:string, relation:uses_mcp/uses_ai_tool, state:observed/removed | 观测或解除连接关系。asset_id 为源资产，removed 表示观测到解除 | 记录资产之间的使用关系。例如源资产是智能体 A，target_asset_id 是 MCP B，uses_mcp 表示 A 使用 B。 |
| aidr.prompt.decision | trace_id,operation_id,parent_operation_id?,policy_revision,rule_ids,decision,enforcement,error_code?, channel：user_input（用户输入）、retrieved_content（检索内容）、tool_output（工具输出）, category:direct_injection/indirect_injection/jailbreak/model_manipulation/none, model_ref:string, principal_ref?:string, summary?:string | 受控检查点做出判定且知道执行结果后。异步检测可报告 audit | 记录提示词从哪里进入、命中哪类风险、由哪个策略版本判定。decision 表示判定结果，enforcement 用于核实是否实际拦截。 |
| aidr.agent.operation | trace_id,operation_id,parent_operation_id?,policy_revision,rule_ids,decision,enforcement,error_code?, kind:tool_call/data_read/network_request, target_ref:string, phase：requested（操作已发起）、completed（操作已结束）, outcome?：succeeded（执行成功）、denied（被拒绝）、failed（执行失败）, duration_ms?:int | 调用前 requested，结束后 completed。两阶段分别生成 source_event_id，operation_id 相同。completed 必须含 outcome | 记录工具调用、数据读取或网络请求的全过程。requested 还原调用意图，completed 确认执行结果和耗时，target_ref 定位访问对象。 |
| aidr.intent.finding | trace_id,operation_id,parent_operation_id?,policy_revision,rule_ids, category:permission_boundary/data_boundary/unexpected_tool_sequence, severity:low/medium/high/critical, related_operation_ids:UUID[], summary:string | 运行时或事后关联命中。关联操作最多 50 条，至少一条 | 解释为什么一串操作构成意图偏离。related_operation_ids 指向支撑该结论的步骤，便于调查时回看。 |
| aidr.data.decision | trace_id,operation_id,parent_operation_id?,policy_revision,rule_ids,decision,enforcement,error_code?, action:upload/share/process, destination_ref:string, destination_kind:internal/external_model/external_other, classifications:string[], bytes?:int | 在受控数据出口完成一次判定。classifications 使用平台分类代码，数组为空表示未命中分类 | 说明哪些敏感数据拟被上传、共享或处理，目标是哪里，以及本机是否实际拦截。classifications 表示命中的敏感数据类别。 |
| aidr.response.result | trace_id,operation_id,parent_operation_id?,policy_revision, action_id:UUID, trigger_event_id?:UUID, target_ref:string, action:pause_agent/revoke_token/revoke_tool_permission, status:succeeded/failed/unsupported, error_code?:string | 执行完成后发出，失败及不支持须有 error_code。trigger_event_id 在已知 Agent 分配的事件 ID 时填写 | 记录暂停、撤销 Token 或工具权限的实际结果。action_id 用于关联处置任务和执行回执，trigger_event_id 回查触发原因。 |

平台按资产主键更新资产快照，连接关系按源、目标和关系类型更新。资产及关系回放或乱序时，优先按 Agent sequence 更新快照。所有运行时事件保留原始到达记录，按 trace 和父操作还原链路。operation 的 requested 和 completed 都保留。意图发现按 related_operation_ids 关联证据。运行时取证附件引用触发事件 event_id，续传沿用第 5 节。确需留存提示词或工具调用详情时，须先开启租户授权，按证据文件权限和留存期限处理。

### B.3 阻断和处置交互

1. 平台通过心跳返回版本化策略。Agent 按第 3 节校验、持久化并通过本机 Configuration/Apply 下发。建议新增 ai_runtime 对象，字段为 mode:audit/enforce、enabled_checks:string[]（prompt/intent/data/access）、rules_artifact_id:string、enforcement_hooks:string[]（model_request/tool_call/data_egress）、on_local_error:allow/block、response_actions:string[]（pause_agent/revoke_token/revoke_tool_permission）。这些字段纳入能力校验。平台在缺少对应拦截点、执行器能力或授权时拒绝发布对应 enforce 或自动处置配置。
2. 模型请求、工具调用和数据出口由具备相应拦截能力的本机运行时在执行前判定。插件或运行时读取当前有效策略并决定放行或拦截。事件流承载判定与执行结果，Agent 落盘和云端 ACK 无需进入同步决策路径。暂时离线时沿用本机生效策略。规则引擎不可用时按 on_local_error 执行并记录结果。只有收到实际阻断结果，平台才展示为已阻断。
3. 本地自动处置由已授权的运行时执行器处理，执行器以 action_id 幂等，完成后通过 aidr.response.result 回传。暂停的目标为运行时实例，撤销的目标为凭据或授权引用。撤销依赖目标系统授权，平台和插件能力声明应具体到动作与目标类型。
4. 平台人工处置若纳入首期，建议扩展独立任务交互。管理后台 POST /admin/agents/{agent_id}/actions，创建任务，201 确认持久化。平台通过 Control/Connect 发送 ActionDispatch 帧，body 为任务对象。重连后补发尚未完成且未过期的任务。Agent 按本机策略和目标归属校验并执行，重复任务复用执行结果。最终结果经事件通道上报，服务端入库后将任务标记 succeeded/failed/unsupported，未执行的过期任务标记 expired。处置任务存储以 (tenant_id,agent_id,action_id) 为主键。action_id 重用且正文变化返回 409。同一个 action_id 的结果重放沿用原 source_event_id。执行器需在接收任务时检查 expires_at，使用平台心跳时间估算时钟偏差。


**POST /admin/agents/{agent_id}/actions 请求**

~~~json
{
  "action_id": "00000000-0000-4000-8000-000000000008",
  "action": "pause_agent",
  "target_ref": "agent-runtime-42",
  "trace_id": "00000000-0000-4000-8000-000000000009",
  "expires_at": "2026-11-27T02:00:00Z"
}
~~~

201 确认平台已持久化任务。业务结果通过 aidr.response.result 上报。

### B.4 产品和厂商联调前需要确认的信息

| 待确认 | 需要明确到的程度 | 首轮联调口径 |
|---|---|---|
| 资产覆盖及归责 | 支持的浏览器、桌面工具、Agent 框架、MCP 配置与连接来源，所有者从哪个权威系统同步 | 先覆盖能稳定识别的来源。归属和授权无法确认时分别省略 owner_ref、标记 unknown |
| 提示词与数据拦截点 | 哪些入口能看到模型输入、检索内容、工具输出和数据出口，哪些入口能暂停请求 | 分能力声明 audit 或 enforce。合格用例同时验证判定和实际执行结果 |
| 意图和权限基线 | 谁定义任务目的、可访问资源、允许工具及跨步骤关联窗口，缺少任务上下文如何呈现 | 优先验证显式权限边界和可关联的操作链。缺少上下文时保留观察事实 |
| 敏感数据范围 | 分类目录、业务例外、可信目的地、脱敏规则、允许保留哪些证据 | 先用模拟数据。原始内容按租户证据授权单独开启 |
| 处置权限 | 暂停的目标和恢复方式，Token 与工具权限由谁签发和撤销，人工处置是否首期开放 | 先接可验证的执行器并对未支持动作回传 unsupported |
| 时延与失败行为 | 每个拦截点的延时预算、本机故障时放行还是阻断、离线运行期限 | 用联调压测确认时延预算。策略显式发布 on_local_error |
| 安全与留存 | 谁能查询、导出、下载取证记录，保存多久，租户删除如何落实 | 沿用第 5、6 节证据授权及留存基线，产品确认后按租户覆盖 |
