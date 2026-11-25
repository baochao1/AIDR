# AIDR 数据契约与 Agent 处理流程

厂商 AIDR 负责本地采集、识别和检测。终端 Agent 负责生命周期、数据接入、可靠外发。管理平台负责资产归并、风险展示和分析。

## 1. 控制面、数据面和文件通道

| 通道 | 谁发起 | 请求与返回 | 当前 Demo |
|---|---|---|---|
| 健康 Health/Check | Agent 调插件 | 请求空 service，插件返回 SERVING 或 NOT_SERVING | 已实现。反映组件能否提供能力 |
| 生命周期 | Agent 向管理端 check-in | 平台返回制品版本与 running、stopped、absent；Agent 上报实际版本、状态和错误 | 已实现。Agent 管理进程和制品 |
| 配置 Configure | 拟由 Agent 调插件 | 请求配置 revision、采集范围、日志白名单、脱敏策略、检测规则版本。响应 revision、applied/rejected 和错误码 | 建议扩展，尚无此 RPC |
| 结构化事件 Events/Stream | Agent 开流，插件有事件时发送 | Agent 先发 ready，插件发一条 JSON 文本，Agent 本地落盘后回复 event_id | 通用文本流已实现。业务分类和 Schema 待定稿 |
| 附件上传 | 拟由 Agent 上传 | 插件声明文件，Agent 按策略和权限校验，通过独立 HTTPS 入口上传 | 设计方案，未实现 |

Health 只判断组件当前能否工作。发现风险走 Events，也可能没有新发现但组件仍健康。生命周期控制通过管理策略下发。以后增加 Configure 时，配置拒绝应保留上一次有效配置。事件消息只承载数据，不能通过 payload 请求执行命令、读取任意文件或修改策略。

### Events 是长流，插件主动发送事件

gRPC 可以保持长连接。这里的 `Events/Stream` 是一次可以持续打开的双向 RPC。**Agent 主动连接插件并打开流；流建立后，由插件在发现新数据时主动发送事件。** 没有新事件时可以等待，无需 Agent 定时逐条查询，也无需插件反向连接 Agent。

例如，AIDR 发现一个 Skill 时：

1. Agent 向插件打开 `Events/Stream`，先发送 `ready`，表示准备接收。
2. 插件发现 Skill 后，沿已经打开的流发送 `aidr.skill.discovered`。
3. Agent 将事件写入终端 SQLite，再沿同一条流返回 `event_id`。当前协议一条一确认，插件收到确认后再发下一条。
4. Agent 另走 HTTPS 批量上报。插件收到 `event_id` 表示终端已落盘，管理端查到同一编号表示平台已持久接收。

长流可以因进程退出、连接故障或组件升级而中断。当前 Demo 在组件启动时建立采集流，断开后会在同一个实例上重试。重连前后 Agent 的事件序号连续。插件端未确认事件的重放与源 ID 去重尚未实现。已在终端落盘的事件由 Agent 队列负责独立续传。

## 2. 统一的事件结构

插件发出的 StringValue.value 是一个 UTF-8 JSON 对象文本。建议每条包含 type、schema_version、source_event_id、observed_at、asset_id 和 data。source_event_id 是厂商生成的关联编号，当前管理端按 Agent 生成的 event_id 去重。插件的 observed_at 是观测时间，Agent 另记录接收时间和序列号。

| type | data 建议字段 | 平台用途 |
|---|---|---|
| aidr.agent.discovered | name、kind、version、discovery_source | 登记本地智能体资产 |
| aidr.agent.log_observed | agent_asset_id、level、category、message_summary、redacted | 关联智能体、查看已脱敏日志摘要 |
| aidr.skill.discovered | agent_asset_id、skill_name、version、sha256、source_kind | 登记 Skill 及其归属和版本 |
| aidr.skill.scan_result | skill_asset_id、rule_set_version、verdict、severity、findings | 关联 Skill，展示检测结论。每条 finding 带 rule_id、category、summary |
| aidr.mcp.discovered | agent_asset_id、server_name、transport、source_kind | 登记 MCP 服务及所属智能体 |
| aidr.mcp.risk_detected | mcp_asset_id、rule_id、risk_category、severity、summary | 关联 MCP 服务并展示风险原因 |
| aidr.evidence.declared | file_id、related_source_event_id、kind、size_bytes、sha256 | 附件通道上线后关联证据，事件内只放元数据 |

建议 verdict 使用 clean、suspicious、risk、error，severity 使用 info、low、medium、high。首轮只提交已授权、脱敏的摘要。完整日志、用户内容、凭据和大体积证据不放在普通事件里。当前 Demo 单条 payload 上限为 1,024 字节。

## 3. 案例：发现 Skill，再上报检测结果

插件先发送发现事件：

    {
      "type":"aidr.skill.discovered","schema_version":1,
      "source_event_id":"vendor-test-101","observed_at":"2026-11-26T02:00:00Z",
      "asset_id":"skill-42",
      "data":{"agent_asset_id":"assistant-21","skill_name":"report-helper","version":"1.0","source_kind":"installed"}
    }

Agent 确认终端落盘后，插件收到 event_id。随后插件发检测结果，沿用同一个 asset_id：

    {
      "type":"aidr.skill.scan_result","schema_version":1,
      "source_event_id":"vendor-test-102","observed_at":"2026-11-26T02:00:02Z",
      "asset_id":"skill-42",
      "data":{
        "skill_asset_id":"skill-42","rule_set_version":"demo-1",
        "verdict":"suspicious","severity":"medium",
        "findings":[{"rule_id":"SK-001","category":"permission_review","summary":"权限范围需要复核"}]
      }
    }

MCP 同理。先发 aidr.mcp.discovered，再用相同 MCP 资产 ID 发 aidr.mcp.risk_detected。智能体观测日志用 agent_asset_id 关联已有智能体，优先传类别、级别和脱敏摘要。

## 4. Agent 怎么识别、处理和外发

~~~mermaid
flowchart LR
    P[厂商 AIDR] -->|结构化 JSON 文本| A[Agent 认证实例并检查消息大小]
    A -->|当前按原文封装| Q[(终端 SQLite)]
    A -.->|正式方案按 type 和版本解析| V[Schema 校验与脱敏]
    V -.-> Q
    Q -->|提交后同一事件流返回 event_id| P
    Q -->|JSON 批次压缩和 HTTPS mTLS| M[管理端]
    M -->|持久去重并入库| D[(管理端数据库)]
    M -->|HTTP 200 确认后清除待发记录| Q
~~~

1. **确认来源。** Agent 使用本次实例的 TLS 连接和 token。组件身份来自启动记录，payload 中的自报身份不能覆盖 Agent 的封装字段。
2. **接收并检查。** 当前 Demo 检查大小并保留 payload 原文。正式方案拟在这里解析 JSON，根据 type 和 schema_version 选择 Schema，检查必填字段、允许采集范围及字段脱敏。处理非法事件时不回成功 ID；结构化错误回执需作为协议扩展。
3. **持久化后回执。** Agent 补齐 event_id、component_id、instance_id、外层 schema_version、timestamp、sequence，将整条记录写入 SQLite。成功后通过同一事件流返回 event_id。
4. **独立外发。** Agent 将多条记录组成 JSON 数组，zstd 压缩，通过 mTLS POST /v1/agents/events 发往管理端。管理端按终端身份和 event_id 持久去重，入库后返回 HTTP 200，Agent 才清理本批待发记录。离线继续入队，队列满时背压。
5. **平台关联。** 当前可按 event_id 查看原始事件及接收量。按 asset_id 关联智能体、Skill、MCP 和风险结果，以及生成资产和风险视图，是后续平台工作。

联调时厂商打印收到的 event_id，我方在管理端用同一个 ID 找到事件。前者证明终端已保存，后者证明平台已持久接收。

### 对同一条检测结果，Agent 和平台分别看什么

| 字段 | 来自哪里 | 怎么处理 |
|---|---|---|
| 外层 component_id、instance_id | Agent 自己管理的插件实例 | 用于识别和审计数据来源 |
| 外层 event_id | Agent 落盘时生成 | 本地 ACK、管理端去重和跨端对账使用同一个 ID |
| payload.type、payload.schema_version | 厂商 JSON 文本 | 当前按原文保存。正式解析时选择对应事件 Schema |
| payload.asset_id、payload.data.skill_asset_id | 厂商 JSON 文本 | 平台用终端身份和资产 ID 关联先前发现的 skill-42 |
| payload.data.verdict、severity、findings | 厂商检测结果 | 平台根据已校验字段展示可疑结论和风险原因 |

例如，第二条 Skill 检测结果在 Agent 外发批次中是下面这种记录。payload 仍然是字符串。这里为便于阅读省略了时间戳、规则和发现详情，实际外发保留原文。

    {
      "event_id":"agent-generated-102",
      "component_id":"aidr-vendor",
      "instance_id":"instance-21",
      "schema_version":1,
      "sequence":2,
      "payload":"{\"type\":\"aidr.skill.scan_result\",\"schema_version\":1,\"asset_id\":\"skill-42\",\"data\":{\"verdict\":\"suspicious\",\"severity\":\"medium\"}}"
    }

## 5. 大文件和原始日志怎么传

以下是后续方案。Events 只传声明和结果，文件字节通过独立通道上传。

~~~mermaid
sequenceDiagram
    participant P as 厂商插件
    participant A as Agent
    participant M as 管理端文件入口
    P->>A: Events 申报 file_id、大小、摘要和关联事件
    P->>P: 将获准文件写入组件专属 spool
    A->>A: 校验归属、文件类型、大小、摘要与上传权限
    A->>M: 经 HTTPS mTLS 上传文件字节
    M-->>A: 持久保存并返回附件引用
    A->>A: 更新附件状态并按保留策略清理文件
~~~

- 插件只提交受控 file_id 或约定的相对文件名。Agent 拒绝绝对路径、目录穿越、符号链接与重解析点。
- 采集和上传须符合管理策略，约束类型、大小、配额、保留期与脱敏要求。常规日志优先发摘要；原始片段仅在获得明确授权后上传。
- 以 file_id 做上传幂等。管理端在发现或检测记录上显示附件状态，文件未到时可显示「待上传」。文件确认后再清理本地 spool。

## 6. 厂商联调通过条件

1. 插件初始化成功，Health 返回 SERVING。
2. 插件发送本地智能体发现、日志摘要、Skill 发现及检测结果、MCP 发现及风险摘要样例。每条都在当前限制内。
3. 插件获得本地 event_id。我方在管理端查到相同 ID 和原文，并核对资产关联字段。当前 Demo 可人工对账。
4. 管理端离线后恢复，核对已 ACK 的待发记录最终可查询。
5. 新的配置 RPC、分类解析、文件上传和正式资产视图单独验收。当前文本事件流无需这些扩展即可联调。



