# Agent 协议调研候选清单

候选来源：

- [Supervisor 实施资料](../../../origin/supervisor-demo-implementation.md)
- [开源 EDR 选型报告](../../../origin/开源EDR框架选型评估报告-终版.html)
- [插件 SDK 集成设计](../../../origin/统一客户端第三方插件SDK集成设计方案.html)

- **核实**：深挖引用已通过 GitHub 官方仓库的 Git 提交接口核实
- **Elastic**：两个提交与各仓库的 `v8.17.0` tag 核对一致
- **其余候选**：沿用 AIDR 已记录的固定引用，早期选型材料中的未锁定项目标为待定

## 首批字段级调研

| 产品及类型 | 组件与固定源码 | 和 AIDR 的可比点 | 调研边界 |
|---|---|---|---|
| Elastic Agent / Fleet，受管采集和终端安全 Agent | [Agent v8.17.0](https://github.com/elastic/elastic-agent/tree/96f2b9fc0dbddfcaac1c55e0e0bc9aed3bf6badb)、[Fleet Server v8.17.0](https://github.com/elastic/fleet-server/tree/d05ee7b30ed62dba0cad7b31ff2ea003897cde82)、[Elastic Agent Client v7.16.0](https://github.com/elastic/elastic-agent-client/tree/86f524ddc40d8aae5a01244ae223fb87f8b33265) | Agent 注册、管理面与数据输出分离、由管理端协调组件 | 从 Fleet Server 的 `AgentEnroll`、`AgentCheckin`、`AgentAcks` 追踪管理面。数据面以实际输出组件和默认 Elasticsearch 路径为切入点，相关依赖按 Agent 固定版本解析。组件本地 IPC 未纳入本次调研。 |
| FleetDM，终端管理和 osquery 遥测 | [Fleet 服务、Orbit 和 osquery 脚手架 · 3807bd7](https://github.com/fleetdm/fleet/tree/3807bd79fedbd25424ae55738b03cc8a4ae28a80) | 终端注册、运行期身份、采集与管理职责分工 | 从 Fleet 服务的 osquery API 和 Orbit API 入手，分别标出调用方、凭据与数据接收逻辑。涉及 osquery 外部运行时的字段依据，以固定提交所引用的版本核对。 |
| Elkeid，主机安全 Agent | [Agent、AgentCenter 和 Manager · c906c51](https://github.com/bytedance/Elkeid/tree/c906c512d20c0d56f2b779b8558ed69abae718c8) | 终端安全事件、插件采集与管理任务 | 分别检查终端至 AgentCenter 的连接、上报及下行消息和 AgentCenter 至管理侧的边界。本次只从部署配置核实认证启用条件，Agent 与插件的本地通道未纳入调研。 |

这三组分别覆盖受管采集、终端管理和主机安全。Fleet Server 属于 Elastic 产品组。FleetDM 是独立产品，包含 Fleet 服务与 Orbit 等组件。

## 已引用的补充候选

| 项目及类型 | 固定源码 | 适用范围与优先级 |
|---|---|---|
| Wazuh，主机安全与遥测 | [4.14.0 · e0707d7](https://github.com/wazuh/wazuh/tree/e0707d735cc60b38da2723376a2f0faeffe697bb)、[独立 Agent · ca21132](https://github.com/wazuh/wazuh-agent/tree/ca211324af5c88caf77fcd57cab0e98475f8e4ce) | 已纳入，见下方链接。正式版用同一固定提交核对 Agent 和 Manager。独立 Agent 为 AIDR 原有队列参考，另查 HTTP 批次与认证。[正式版](wazuh/wazuh.md) · [独立 Agent](wazuh-agent/wazuh-agent.md) |
| MiNiFi C++，边缘采集 | [1c87f39](https://github.com/apache/nifi-minifi-cpp/tree/1c87f39c13e45c226bf11442af5f454f77fa6db9) | C2 心跳和采集配置，适用于采集 Agent 对照。服务端与具体数据出口随部署选型确定。 |
| Mender，受管设备 OTA | [Mender 4.0.0 · 146951a](https://github.com/mendersoftware/mender/tree/146951aa650064af0c28b1e42eb729bcd7aa9155)、[Mender Artifact · e56270c](https://github.com/mendersoftware/mender-artifact/tree/e56270c4a7cc4a028c11252b78cd1f3fbb0359df) | 设备更新、提交状态、制品签名与封装。适合比较设备管理和软件更新。 |
| HashiCorp go-plugin，插件运行库 | [v1.6.3 · 5b05b3e](https://github.com/hashicorp/go-plugin/tree/5b05b3e78c6c3e4e0b8f21a9921cbd739ba72f2a) | 本地进程握手、RPC 与重新附加，适用于插件协议。 |
| Nomad，工作负载编排 | [v1.9.5 · 0b7bb8b](https://github.com/hashicorp/nomad/tree/0b7bb8b60758981dae2a78a0946742e09f8316f5) | 既有调研关注驱动的任务生命周期、状态恢复和检查。 |
| Go supervisord，进程监管 | [ea8e753](https://github.com/ochinchina/supervisord/tree/ea8e753278fc6df10b766026258483837aeae508) | 进程启动、存活检测与失败阈值，适用于运行时管理。 |

## 选型报告中的其他候选

以下候选在 [开源 EDR 选型报告](../../../origin/开源EDR框架选型评估报告-终版.html)出现，AIDR 既有调研未记录固定版本，暂作后续备选。

| 项目 | 源码或官方入口 | 与 AIDR 的可比点 |
|---|---|---|
| Velociraptor | [源码](https://github.com/Velocidex/velociraptor) | 受管终端取证、事件采集、远程任务和数据回传。 |
| OpenEDR | [源码](https://github.com/ComodoSecurity/openedr) | Windows 端点安全遥测与检测组件。 |
| osquery | [源码](https://github.com/osquery/osquery) | 终端状态查询、结果日志与远程插件接口，亦是 FleetDM 数据链路的依赖。 |
| Sysmon | [官方文档](https://learn.microsoft.com/en-us/sysinternals/downloads/sysmon) | Windows 事件来源，适合比较本地采集能力。 |
| Falco | [源码](https://github.com/falcosecurity/falco) | 运行期安全事件采集与输出，适合比较遥测源。 |
| OSSEC | [源码](https://github.com/ossec/ossec-hids) | 主机入侵检测 Agent 和管理服务。 |
| Rustinel | [官方技术资料](https://docs.rustinel.io/how-it-works) | Windows EDR 对照。字段级调研所需的固定协议来源待查。 |
