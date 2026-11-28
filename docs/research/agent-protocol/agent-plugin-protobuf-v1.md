# AIDR Agent（底座）↔ 能力插件 Protobuf 协议

本文件从《AIDR 终端能力插件对接规范 V1.0 更新版》第 6 章整理，只保留 **Agent（底座）与插件之间的交互 PB 契约**。原 docx 是唯一出处，本文是其 PB 部分的便于检索版本；字段号、枚举取值以原文档及 `api/aidr_plugin.proto` 为准。

本文件与 `endpoint-platform-v2.md` 边界不同：后者只覆盖 Agent ↔ 管理端；Agent ↔ 插件的本机接入由终端实现决定，不纳入端管协议。

## 1 通道与调用方式

| 项目 | 约定 |
|---|---|
| 传输 | gRPC over UDS（本地域套接字），protobuf 二进制，不经网络端口；排障可临时切 localhost TCP，生产不使用 |
| 拨号地址 | Agent 通过启动参数 `--address` 注入；Linux 示例 `unix:///tmp/aidr-plugin-<id_sanitized>.sock`，Windows 为 `%TEMP%/aidr/` 下同名文件，`id` 中的 `-` 替换为 `_` |
| 方法全名 | `aidr.plugin.v1.AIDRPlugin/<RpcName>`，是 gRPC 请求里的 `:path`，不与拨号地址拼成 URL |
| 连接方向 | 插件被动 `listen`，Agent 主动拨号；所有 RPC 共用同一条连接 |
| 在用 RPC | `GetStatus`、`Shutdown`、`ScanAssets`、`SubscribeEvents`、`ExecuteAction`、`UpdatePolicy` |

端到端典型顺序：`Dial → GetStatus → SubscribeEvents（建流）→ ScanAssets → UpdatePolicy → policy.ack → risk.alert / toolchain.step → ExecuteAction → action.receipt → Shutdown`。

## 2 权威契约 aidr_plugin.proto（全文）

厂商必须原样引入，`api/aidr_plugin.proto` 是唯一契约文件：不得拆分为多个 proto、不得改字段号、不得新增私有 message。破坏性变更必须新开 `aidr.plugin.v2`，不得复用字段号；废弃字段用 `reserved` 保留，新增字段使用新编号。

~~~proto
syntax = "proto3";
package aidr.plugin.v1;
option go_package = "aidr-client/api;pluginpb";

service AIDRPlugin {
  rpc GetStatus(GetStatusRequest) returns (GetStatusResponse);
  rpc Shutdown(ShutdownRequest) returns (ShutdownResponse);
  rpc ScanAssets(ScanAssetsRequest) returns (ScanAssetsResponse);
  rpc SubscribeEvents(SubscribeEventsRequest) returns (stream AIDREvent);
  rpc ExecuteAction(ExecuteActionRequest) returns (ExecuteActionResponse);
  rpc UpdatePolicy(UpdatePolicyRequest) returns (UpdatePolicyResponse);
}

message GetStatusRequest {}

message GetStatusResponse {
  string plugin_id = 1;
  string version = 2;
  string status = 3;              // ready / busy / degraded
  repeated string capabilities = 4;
}

message ShutdownRequest { string reason = 1; }
message ShutdownResponse { bool ok = 1; }

message ScanAssetsRequest { repeated string scopes = 1; }
message ScanAssetsResponse { repeated Asset assets = 1; }

message SubscribeEventsRequest { repeated string event_types = 1; }

message ExecuteActionRequest {
  string action_id = 1;
  string action_type = 2;         // block_process / revoke_token / quarantine_file
  int32 process_pid = 3;
  string process_path = 4;
  string session_id = 5;
  string token_hint = 6;          // 令牌摘要/掩码，禁止明文
  string file_path = 7;
  string chain_id = 8;
  string step_id = 9;
  string reason = 10;
  string request_id = 11;
}

message ExecuteActionResponse {
  string action_id = 1;
  bool success = 2;
  string receipt = 3;
}

message UpdatePolicyRequest {
  string policy_id = 1;
  string version = 2;
  string effect = 3;              // observe / enforce
  repeated string agent_ids = 4;
  repeated string user_ids = 5;
  repeated string session_ids = 6;
  repeated string asset_ids = 7;
  repeated PolicyRule rules = 8;
  string checksum = 9;
  int64 effective_at_ms = 10;
  int64 expires_at_ms = 11;
}

message UpdatePolicyResponse { bool accepted = 1; }

message Asset {
  string asset_id = 1;
  string name = 2;
  string type = 3;
  string path_or_url = 4;
  string version = 5;
  string vendor = 6;
  string model_name = 7;
  string process_name = 8;
  int32 process_pid = 9;
  string user_id = 10;
  string container_id = 11;
  repeated string risk_tags = 12;
  int64 first_seen_ms = 13;
  int64 last_seen_ms = 14;
}

message Finding {
  string id = 1;
  string severity = 2;
  string desc = 3;
  string file = 4;
}

message PolicyRule {
  string rule_id = 1;
  string category = 2;
  string action = 3;
  string condition = 4;          // 简单 KV 字符串，复杂条件后续扩 pb
  int32 priority = 5;
  bool enabled = 6;
}

message AIDREvent {
  string event_id = 1;
  string event_type = 2;
  int64 timestamp = 3;
  string risk_level = 4;
  string plugin_id = 5;
  int64 received_at_ms = 6;
  string user_id = 7;
  string session_id = 8;
  string asset_id = 9;
  string agent_id = 10;
  string agent_name = 11;
  string tenant_id = 12;
  string trace_id = 13;
  string session_reason = 14;
  string behavior_type = 15;
  string event_kind = 16;
  string rule_id = 17;
  string rule_name = 18;
  string attack_type = 19;
  string action = 20;
  string severity = 21;
  string alert_id = 22;
  string process_name = 23;
  int32 process_pid = 24;
  string process_path = 25;
  string command_line = 26;          // 已脱敏
  string prompt_digest = 27;
  string tool_name = 28;
  string target_url = 29;
  string target = 30;
  string parent_event_id = 31;
  int64 occurred_ms = 32;
  int64 event_time_ms = 33;
  string description = 34;
  repeated string evidence = 35;     // 已脱敏
  repeated string risk_tags = 36;
  string chain_id = 37;
  string step_id = 38;
  string parent_step_id = 39;
  bool relation_confirmed = 40;
  string step_type = 41;
  string tool_type = 42;
  string server_name = 43;
  string args_digest = 44;
  string result_digest = 45;
  string result_status = 46;
  int64 started_ms = 47;
  int64 ended_ms = 48;
  int32 step_count = 49;
  int64 total_duration_ms = 50;
  string end_status = 51;
  int32 risk_event_count = 52;
  string scan_id = 53;
  string target_type = 54;
  string target_name = 55;
  string conclusion = 56;
  string report_digest = 57;
  int64 finished_ms = 58;
  repeated Finding findings = 59;
  string policy_id = 60;
  string policy_version = 61;
  string policy_result = 62;
  string effect = 63;
  int64 effective_at_ms = 64;
  string action_id = 65;
  string action_type = 66;
  string action_result = 67;
  string code = 68;
  int64 executed_at_ms = 69;
  string status = 70;
  string state = 71;
  int32 uptime_s = 72;
  int64 total_events = 73;
  int32 dropped_count = 74;
  int32 queue_size = 75;
  double cpu_percent = 76;
  int64 mem_bytes = 77;
  string message = 78;
  string error_msg = 79;
}
~~~

Stub 生成（在包含 `api/aidr_plugin.proto` 的工程根目录执行）：

~~~bash
protoc -I. --go_out=. --go-grpc_out=. api/aidr_plugin.proto
python -m grpc_tools.protoc -I. --python_out=. --grpc_python_out=. api/aidr_plugin.proto
protoc -I. --java_out=. --grpc-java_out=. api/aidr_plugin.proto
protoc -I. --cpp_out=. --grpc_out=. --plugin=protoc-gen-grpc=$(which grpc_cpp_plugin) api/aidr_plugin.proto
~~~

## 3 各 RPC 字段说明

### 3.1 GetStatus（状态与健康检查）

`aidr.plugin.v1.AIDRPlugin/GetStatus`，Agent → 插件，Unary。连接建立后首次调用，运行期可周期调用；5 秒内未返回 `ready` 视为启动失败。

| 字段 | 类型 | 编号 | 必填 | 含义与约束 |
|---|---|---|---|---|
| `GetStatusRequest` | — | — | — | 空请求；Agent 连接后首先调用 |
| `GetStatusResponse.plugin_id` | string | 1 | 是 | 须与 `manifest.id` 一致 |
| `GetStatusResponse.version` | string | 2 | 是 | `x.y.z`，与 `manifest.version` 一致 |
| `GetStatusResponse.status` | string | 3 | 是 | `ready` / `busy` / `degraded`，不得恒返回 ready |
| `GetStatusResponse.capabilities` | repeated string | 4 | 是 | 与 `manifest.capabilities` 一致或为其子集 |

### 3.2 Shutdown（优雅退出）

`aidr.plugin.v1.AIDRPlugin/Shutdown`，Agent → 插件，Unary。客户端退出 / 升级 / 卸载 / 人工停止时调用；插件须在 2 秒内完成 flush 与 GracefulStop。

| 字段 | 类型 | 编号 | 必填 | 含义与约束 |
|---|---|---|---|---|
| `ShutdownRequest.reason` | string | 1 | 是 | `client_exit` / `upgrade` / `uninstall` / `manual` |
| `ShutdownResponse.ok` | bool | 1 | 是 | 是否已开始优雅退出；Agent 随后仍会关闭连接 |

### 3.3 ScanAssets（资产扫描）

`aidr.plugin.v1.AIDRPlugin/ScanAssets`，Agent → 插件，Unary。资产只走本信道，不进事件流；`scopes` 为空表示全量，无资产返回空数组而非 nil。

| 字段 | 类型 | 编号 | 必填 | 含义与约束 |
|---|---|---|---|---|
| `ScanAssetsRequest.scopes` | repeated string | 1 | 否 | 扫描范围（`asset_id` / 路径 / 类型）；空表示全量 |
| `ScanAssetsResponse.assets` | repeated Asset | 1 | 是 | 资产清单；无资产时返回空数组 |

`Asset`：

| 字段 | 类型 | 编号 | 必填 | 含义与约束 |
|---|---|---|---|---|
| `asset_id` | string | 1 | 是 | 稳定唯一 ID，哈希生成，跨上报保持一致 |
| `name` | string | 2 | 是 | 资产可读名称 |
| `type` | string | 3 | 是 | `model` / `agent` / `skill` / `mcp-server` / `tool` / `rag` / `ai-saas` / `ai-gateway` / `local-app` / `cli-agent` |
| `path_or_url` | string | 4 | 是 | 本地路径，或端点 URL / IP:Port |
| `version` | string | 5 | 否 | 资产版本 |
| `vendor` | string | 6 | 否 | 生产厂商 |
| `model_name` | string | 7 | 否 | 模型名称 |
| `process_name` | string | 8 | 否 | 关联进程名 |
| `process_pid` | int32 | 9 | 否 | 关联进程 PID |
| `user_id` | string | 10 | 否 | 触发用户 |
| `container_id` | string | 11 | 否 | 容器 ID |
| `risk_tags` | repeated string | 12 | 否 | `shadow_ai` / `no_auth` / `over_privileged` / `changed` |
| `first_seen_ms` | int64 | 13 | 是 | 首次发现时间（Unix 毫秒） |
| `last_seen_ms` | int64 | 14 | 是 | 最近发现时间（Unix 毫秒） |

### 3.4 SubscribeEvents（事件推送）

`aidr.plugin.v1.AIDRPlugin/SubscribeEvents`，Agent 发起调用（Server Streaming），数据方向插件 → Agent。事件生成即推送，`critical` / `high` 不得聚合延迟；同一时刻只允许一条事件流，断流 2 秒重连。

| 字段 | 类型 | 编号 | 必填 | 含义与约束 |
|---|---|---|---|---|
| `SubscribeEventsRequest.event_types` | repeated string | 1 | 否 | 过滤类型，覆盖下文全部在用 `event_type`；空表示全部 |

`AIDREvent`：79 个字段全部平铺，无嵌套 payload；不同事件按 `event_type` 填写相关字段，未用字段留空。`plugin_id` / `received_at_ms` 可由 Agent 接收时补齐。

| 字段 | 类型 | 编号 | 必填 | 含义与约束 |
|---|---|---|---|---|
| `event_id` | string | 1 | 是 | UUID，全局唯一，幂等去重与关联用 |
| `event_type` | string | 2 | 是 | 事件类型，取值见第 4 节 |
| `timestamp` | int64 | 3 | 是 | Unix 毫秒，事件生成时间 |
| `risk_level` | string | 4 | 是 | `critical` / `high` / `medium` / `low` / `none` / `info` |
| `plugin_id` | string | 5 | 条件必填 | 来源插件 id；插件可留空，由 Agent 补齐 |
| `received_at_ms` | int64 | 6 | 否 | Agent 接收时间（毫秒），由 Agent 补齐 |
| `user_id` | string | 7 | 条件必填 | 触发用户；无法获取填 `__unknown__` 并在 `session_reason` 说明 |
| `session_id` | string | 8 | 条件必填 | 智能体运行会话 ID，链路与告警归因关键字段 |
| `asset_id` | string | 9 | 条件必填 | 关联 AI 资产 ID，与 `ScanAssets` 返回值一致 |
| `agent_id` | string | 10 | 建议 | 智能体标识（Agent / CLI / IDE 编程助手） |
| `agent_name` | string | 11 | 建议 | AI Agent 名称，便于展示与审计 |
| `tenant_id` | string | 12 | 可选 | 多租户标识 |
| `trace_id` | string | 13 | 可选 | 链路追踪 ID，与 APM 串联 |
| `session_reason` | string | 14 | 条件必填 | 无法归因时的会话原因说明 |
| `behavior_type` | string | 15 | 条件必填 | `file` / `process` / `network` / `toolcall` / `prompt` / `modelcall` |
| `event_kind` | string | 16 | 否 | 行为动作细分：`read` / `write` / `delete` / `exec` / `connect` 等 |
| `rule_id` | string | 17 | 否 | 触发规则 ID，与 `PolicyRule.rule_id` 对应 |
| `rule_name` | string | 18 | 否 | 规则名称 |
| `attack_type` | string | 19 | 否 | `prompt_injection` / `tool_abuse` / `dlp` 等 |
| `action` | string | 20 | 否 | 规则动作：`allow` / `record` / `alert` / `block` / `mask` |
| `severity` | string | 21 | 否 | 严重级别，与 `risk_level` 配合使用 |
| `alert_id` | string | 22 | 否 | 告警 ID，同一风险的多次告警可关联 |
| `process_name` | string | 23 | 否 | 关联进程名 |
| `process_pid` | int32 | 24 | 否 | 关联进程 PID |
| `process_path` | string | 25 | 否 | 关联进程可执行文件路径 |
| `command_line` | string | 26 | 否 | 命令行原文，必须脱敏，禁止明文凭据 |
| `prompt_digest` | string | 27 | 否 | 提示词 sha256 摘要，禁止传提示词原文 |
| `tool_name` | string | 28 | 否 | 工具 / MCP 名称 |
| `target_url` | string | 29 | 否 | 访问 URL，须去除查询参数中的敏感值 |
| `target` | string | 30 | 否 | 目标对象标识（文件 / 进程 / 资产） |
| `parent_event_id` | string | 31 | 否 | 父事件 ID，用于事件归并 |
| `occurred_ms` | int64 | 32 | 否 | 行为实际发生时间（毫秒） |
| `event_time_ms` | int64 | 33 | 否 | 插件生成事件时间（毫秒） |
| `description` | string | 34 | 否 | 事件描述，须脱敏 |
| `evidence` | repeated string | 35 | 否 | 证据片段列表，必须脱敏、最小化、可复核 |
| `risk_tags` | repeated string | 36 | 否 | `shadow_ai` / `no_auth` / `over_privileged` / `changed` |
| `chain_id` | string | 37 | 条件必填 | 工具执行链路 ID，`toolchain.*` 必填 |
| `step_id` | string | 38 | 条件必填 | 链路步骤 ID，`toolchain.*` 必填 |
| `parent_step_id` | string | 39 | 否 | 父步骤 ID，用于还原父子与委派关系 |
| `relation_confirmed` | bool | 40 | 否 | 父子关系是否已确认 |
| `step_type` | string | 41 | 否 | `tool_call` / `file_io` / `network` 等 |
| `tool_type` | string | 42 | 否 | `mcp` / `builtin` / `skill` / `shell` |
| `server_name` | string | 43 | 否 | MCP Server 名称 |
| `args_digest` | string | 44 | 否 | 工具参数摘要，禁止传原文 |
| `result_digest` | string | 45 | 否 | 工具返回结果摘要 |
| `result_status` | string | 46 | 否 | 步骤结果：`ok` / `failed` / `timeout` |
| `started_ms` | int64 | 47 | 否 | 步骤开始时间（毫秒） |
| `ended_ms` | int64 | 48 | 否 | 步骤结束时间（毫秒） |
| `step_count` | int32 | 49 | 否 | 链路步骤总数，`toolchain.summary` 填写 |
| `total_duration_ms` | int64 | 50 | 否 | 链路总耗时（毫秒） |
| `end_status` | string | 51 | 否 | 链路结束状态：`completed` / `failed` / `aborted` |
| `risk_event_count` | int32 | 52 | 否 | 链路内风险事件数量 |
| `scan_id` | string | 53 | 否 | 供应链扫描任务 ID |
| `target_type` | string | 54 | 否 | 扫描对象类型：`skill` / `mcp` / `dependency` |
| `target_name` | string | 55 | 否 | 扫描对象名称 |
| `conclusion` | string | 56 | 否 | 扫描结论：可使用 / 谨慎使用 / 不建议使用 |
| `report_digest` | string | 57 | 否 | 扫描报告摘要值 |
| `finished_ms` | int64 | 58 | 否 | 扫描完成时间（毫秒） |
| `findings` | repeated Finding | 59 | 否 | 问题项列表，见下表 |
| `policy_id` | string | 60 | 否 | 关联策略 ID，`policy.ack` 必填 |
| `policy_version` | string | 61 | 否 | 策略版本 |
| `policy_result` | string | 62 | 否 | 策略生效结果：`applied` / `rejected` / `partial` |
| `effect` | string | 63 | 否 | 策略生效模式：`observe` / `enforce` |
| `effective_at_ms` | int64 | 64 | 否 | 策略生效时间（毫秒） |
| `action_id` | string | 65 | 否 | 关联处置 ID，`action.receipt` 必填 |
| `action_type` | string | 66 | 否 | 处置类型，与 `ExecuteActionRequest.action_type` 一致 |
| `action_result` | string | 67 | 否 | 处置结果：`success` / `failed` / `skipped` |
| `code` | string | 68 | 否 | 错误码，见第 5 节 |
| `executed_at_ms` | int64 | 69 | 否 | 处置执行时间（毫秒） |
| `status` | string | 70 | 否 | 插件状态 `ready` / `busy` / `degraded`，`plugin.status` 必填 |
| `state` | string | 71 | 否 | 扩展状态描述 |
| `uptime_s` | int32 | 72 | 否 | 插件运行时长（秒） |
| `total_events` | int64 | 73 | 否 | 累计产生事件数 |
| `dropped_count` | int32 | 74 | 否 | 累计丢弃事件数 |
| `queue_size` | int32 | 75 | 否 | 当前事件队列长度 |
| `cpu_percent` | double | 76 | 否 | CPU 使用率（0~100） |
| `mem_bytes` | int64 | 77 | 否 | 内存占用（字节） |
| `message` | string | 78 | 否 | 事件补充说明或错误消息，须脱敏 |
| `error_msg` | string | 79 | 否 | 错误详情，`plugin.error` 必填 |

`Finding`（`AIDREvent.findings`，供应链扫描结果使用）：

| 字段 | 类型 | 编号 | 必填 | 含义与约束 |
|---|---|---|---|---|
| `id` | string | 1 | 是 | 问题项 ID |
| `severity` | string | 2 | 是 | 问题严重级别 |
| `desc` | string | 3 | 是 | 问题描述，须脱敏 |
| `file` | string | 4 | 否 | 关联文件路径 |

### 3.5 ExecuteAction（处置执行）

`aidr.plugin.v1.AIDRPlugin/ExecuteAction`，Agent → 插件，Unary。策略命中或人工处置时调用；必须返回回执，失败原因由 `receipt` 与 `action.receipt` 事件的 `code` / `message` 承载。

| 字段 | 类型 | 编号 | 必填 | 含义与约束 |
|---|---|---|---|---|
| `action_id` | string | 1 | 是 | 全局唯一，回执原样带回 |
| `action_type` | string | 2 | 是 | `block_process` / `revoke_token` / `quarantine_file` |
| `process_pid` | int32 | 3 | 条件必填 | `block_process` 时必填 |
| `process_path` | string | 4 | 否 | 目标进程路径，便于复核 |
| `session_id` | string | 5 | 条件必填 | `revoke_token` 时必填 |
| `token_hint` | string | 6 | 条件必填 | 令牌摘要 / 掩码，禁止明文凭据 |
| `file_path` | string | 7 | 条件必填 | `quarantine_file` 时必填 |
| `chain_id` | string | 8 | 否 | 关联工具执行链路 ID |
| `step_id` | string | 9 | 否 | 关联链路步骤 ID |
| `reason` | string | 10 | 是 | 处置原因，关联 `rule_id` 与事件说明 |
| `request_id` | string | 11 | 否 | 管理台请求 ID，用于审计串联 |
| `ExecuteActionResponse.action_id` | string | 1 | 是 | 原样带回 |
| `ExecuteActionResponse.success` | bool | 2 | 是 | 是否执行成功 |
| `ExecuteActionResponse.receipt` | string | 3 | 是 | 成功如 `Ok:process_blocked`；失败含原因，错误码见事件 `code` |

### 3.6 UpdatePolicy（策略下发）

`aidr.plugin.v1.AIDRPlugin/UpdatePolicy`，Agent → 插件，Unary。校验 `checksum` 后原子替换规则；生效后经事件流回送 `policy.ack`；校验失败保留上一版本并回 `accepted=false`。

| 字段 | 类型 | 编号 | 必填 | 含义与约束 |
|---|---|---|---|---|
| `policy_id` | string | 1 | 是 | 策略唯一 ID |
| `version` | string | 2 | 是 | 策略版本，用于幂等与回滚 |
| `effect` | string | 3 | 是 | `observe` / `enforce` |
| `agent_ids` | repeated string | 4 | 否 | 适用智能体 ID 列表 |
| `user_ids` | repeated string | 5 | 否 | 适用用户 ID 列表 |
| `session_ids` | repeated string | 6 | 否 | 适用会话 ID 列表 |
| `asset_ids` | repeated string | 7 | 否 | 适用资产 ID 列表 |
| `rules` | repeated PolicyRule | 8 | 是 | 规则列表 |
| `checksum` | string | 9 | 是 | 策略完整性校验值（sha256） |
| `effective_at_ms` | int64 | 10 | 否 | 生效时间（毫秒） |
| `expires_at_ms` | int64 | 11 | 否 | 过期时间（毫秒），0 表示不过期 |
| `UpdatePolicyResponse.accepted` | bool | 1 | 是 | 是否接受并生效 |

`PolicyRule`：

| 字段 | 类型 | 编号 | 必填 | 含义与约束 |
|---|---|---|---|---|
| `rule_id` | string | 1 | 是 | 规则 ID，事件 `rule_id` 回引 |
| `category` | string | 2 | 是 | `file` / `command` / `process` / `network` / `prompt` / `tool` / `dlp` |
| `action` | string | 3 | 是 | `allow` / `record` / `alert` / `block` / `mask`（`observe` / `enforce` 是 `effect`，不是 action） |
| `condition` | string | 4 | 否 | 简单 KV 条件字符串，复杂条件后续扩 PB |
| `priority` | int32 | 5 | 否 | 优先级，数值越大越优先 |
| `enabled` | bool | 6 | 是 | 是否启用 |

## 4 event_type 清单与主要字段

| `event_type` | 用途 | `risk_level` 建议 | 主要字段 |
|---|---|---|---|
| `plugin.status` | 状态变化（ready/busy/degraded） | info | status / state / uptime_s |
| `plugin.heartbeat` | 周期心跳与资源统计 | info | uptime_s / total_events / dropped_count / queue_size / cpu_percent / mem_bytes |
| `plugin.error` | 插件异常与诊断 | medium | error_msg / code / message |
| `behavior.file` | 文件读写删除/权限变更 | low~high | behavior_type / process_name / process_path / target / description / evidence |
| `behavior.process` | 命令与子进程活动 | low~high | behavior_type / process_name / process_pid / command_line / evidence |
| `behavior.network` | 网络连接与异常外联 | low~high | behavior_type / target_url / target / process_pid / evidence |
| `behavior.toolcall` | 工具/MCP 调用（单步导航用） | medium~critical | tool_name / tool_type / server_name / args_digest / result_digest |
| `behavior.prompt` | 用户输入与模型输出交互 | medium~high | prompt_digest / session_id / user_id / description |
| `behavior.modelcall` | 模型调用与 Token 用量 | info | agent_name / tool_name / result_digest / started_ms / ended_ms |
| `toolchain.step` | Agent 工具执行链路单步记录 | info~high | chain_id / step_id / parent_step_id / step_type / relation_confirmed |
| `toolchain.summary` | 链路结束汇总（完成/异常/人工终止） | info~high | chain_id / step_count / total_duration_ms / end_status / risk_event_count |
| `risk.alert` | 威胁告警（提示词注入/工具滥用/DLP 等） | medium~critical | rule_id / attack_type / severity / alert_id / evidence / session_id |
| `scan.result` | Skill/MCP/依赖供应链扫描结论 | info~high | scan_id / target_type / target_name / conclusion / findings / report_digest |
| `policy.ack` | 策略生效确认 | info | policy_id / policy_version / policy_result / effect |
| `action.receipt` | 处置执行回执 | info | action_id / action_type / action_result / code / executed_at_ms |

## 5 错误码（与 PB 回执配套）

错误码通过 `AIDREvent.code` 与 RPC 响应（`accepted` / `success`）共同体现。

| 错误码 | 含义 | 处理建议 |
|---|---|---|
| `OK` | 成功 | 记录回执 |
| `ERR_SCHEMA` | `AIDREvent` 无法按固定 PB 解码 | 插件拒绝处理并按所属信道回 `accepted=false` / `success=false` |
| `ERR_CHECKSUM` | 策略校验失败 | 丢弃策略，回执说明，保留上一版本 |
| `ERR_UNSUPPORTED_ACTION` | 不支持的处置类型 | 返回失败，管理台不得标记为已执行 |
| `ERR_TARGET_NOT_FOUND` | 处置目标不存在 | 返回失败并附目标标识 |
| `ERR_BUSY` | 插件繁忙 | Agent 可退避重试，最多 3 次 |
| `ERR_INTERNAL` | 插件内部错误 | 上报 `plugin.error` 事件 |

## 6 Protobuf 文本格式样例

以下为原文各信道完整样例（protobuf 文本格式），方法全名见第 3 节。

~~~protobuf
// GetStatus
GetStatusRequest {}
GetStatusResponse {
  plugin_id: "fangcun-observer"
  version: "1.0.0"
  status: "ready"
  capabilities: "asset-scan"
  capabilities: "behavior-capture"
  capabilities: "dlp"
}

// Shutdown
ShutdownRequest { reason: "upgrade" }
ShutdownResponse { ok: true }

// ScanAssets
ScanAssetsRequest { scopes: "agent"  scopes: "local-app" }
ScanAssetsResponse {
  assets: {
    asset_id: "a-7d21f0c4b9e4"
    name: "qwen-chat"
    type: "local-app"
    path_or_url: "C:/Apps/qwen/qwen.exe"
    version: "2.1.0"
    vendor: "Alibaba"
    model_name: "qwen-plus"
    process_name: "qwen.exe"
    process_pid: 8124
    user_id: "zhangsan"
    risk_tags: "shadow_ai"
    first_seen_ms: 1764057600000
    last_seen_ms: 1764144000000
  }
}

// SubscribeEvents（risk.alert）
SubscribeEventsRequest { event_types: "risk"  event_types: "behavior"  event_types: "toolchain" }
AIDREvent {
  event_id: "evt-6f3a9b21-8c4d-4e17-9a02-1b7c5d0e3f44"
  event_type: "risk.alert"
  timestamp: 1764144000000
  risk_level: "high"
  user_id: "zhangsan"
  session_id: "sess-8f3e1c"
  asset_id: "a-7d21f0c4b9e4"
  agent_id: "agent-qwen-01"
  agent_name: "qwen-agent"
  rule_id: "R-PROMPT-001"
  rule_name: "间接提示词注入检测"
  attack_type: "prompt_injection"
  action: "alert"
  severity: "high"
  alert_id: "alert-0031"
  process_name: "qwen.exe"
  process_pid: 8124
  prompt_digest: "sha256:9f8e2c..."
  tool_name: "shell"
  occurred_ms: 1764143999000
  description: "检测到工具返回结果中的间接提示词注入尝试"
  evidence: "cmd=curl http://example.invalid/ [masked]"
  chain_id: "chain-77c1"
  step_id: "step-3"
}

// ExecuteAction（含 action.receipt 事件）
ExecuteActionRequest {
  action_id: "act-9f8e"
  action_type: "block_process"
  process_pid: 8124
  process_path: "C:/Apps/qwen/qwen.exe"
  chain_id: "chain-77c1"
  step_id: "step-3"
  reason: "R-PROMPT-001 / 命中提示词注入阻断规则"
  request_id: "req-20261127-001"
}
ExecuteActionResponse {
  action_id: "act-9f8e"
  success: true
  receipt: "Ok:process_blocked"
}
AIDREvent { event_type: "action.receipt", action_id: "act-9f8e",
            action_result: "success", code: "OK", executed_at_ms: 1764144000123 }

// UpdatePolicy（含 policy.ack 事件）
UpdatePolicyRequest {
  policy_id: "pol-20261127-001"
  version: "3"
  effect: "enforce"
  agent_ids: "agent-qwen-01"
  session_ids: "sess-8f3e1c"
  rules: { rule_id: "R-PROMPT-001", category: "prompt", action: "block",
           condition: "attack_type=prompt_injection", priority: 90, enabled: true }
  rules: { rule_id: "R-FILE-001", category: "file", action: "alert",
           condition: "path=/etc/shadow", priority: 60, enabled: true }
  checksum: "sha256:9f8e2c..."
  effective_at_ms: 1764144000000
  expires_at_ms: 0
}
UpdatePolicyResponse { accepted: true }
AIDREvent { event_type: "policy.ack", policy_id: "pol-20261127-001",
            policy_version: "3", policy_result: "applied", effect: "enforce",
            effective_at_ms: 1764144000000 }
~~~

## 7 使用约束

- proto3 未赋值字段取默认值，插件按字段说明填写必填字段。
- 字段只增不改号；废弃字段用 `reserved` 保留；新增字段使用新编号。
- 资产数据只走 `ScanAssets`，不走事件通道；插件内部资产变化在下次扫描结果中体现。
- 事件统一使用 `AIDREvent` 固定 PB 平铺字段，不定义嵌套 payload 或私有载荷。
- 提示词、模型原文输出、凭据、密钥、个人隐私数据禁止进入事件字段 / `evidence`；仅传摘要或掩码值。
- 生产环境对本地 UDS 启用 mTLS；开发 / 排障可用 plaintext。
- 策略下发、告警事件、工具执行链路与处置执行均须可追溯（`policy_id` / `event_id` / `chain_id` / `action_id` 关联）。
