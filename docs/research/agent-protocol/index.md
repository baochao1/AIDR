# Agent 认证与数据面协议调研任务

状态：已交付首批三个固定产品的调研，并补充 Wazuh 4.14.0 正式版与独立 wazuh-agent 实验实现。验证结果和依赖部署的边界见下表。

## 调研产出

| 交付物 | 覆盖范围 |
|---|---|
| [六个参考实现](design-comparison.md) | 固定版本来源与采用关系 |
| [协议与组件复用选型](design-optimization.md) | 认证、插件 IPC 与制品更新的复用路径 |
| [AIDR 协议和第三方插件数据契约](endpoint-platform-v2.md) | 从认证、管控、心跳、遥测到插件、文件的字段、枚举、报文与错误处理 |
| [类似产品和项目清单](candidates.md) | AIDR 已有参考、补充候选、首批产品及固定版本 |
| [Elastic Agent / Fleet](elastic/elastic.md) | 注册与访问凭据、Fleet 控制接口、Elasticsearch Bulk 数据面 |
| [FleetDM / osquery](fleetdm/fleetdm.md) | 节点注册和认证、日志及查询结果上报、文件分块上传 |
| [FleetDM / Orbit](fleetdm-orbit/fleetdm-orbit.md) | Orbit 注册、节点密钥、主机身份签名、SCEP 与 WebSocket |
| [Elkeid](elkeid/elkeid.md) | 双端 TLS、服务发现、gRPC 上报及文件流、AgentCenter 到 Kafka |
| [Elkeid 采集字段附录](elkeid/elkeid-fields.md) | 53 种记录、926 个唯一类型与字段配对 |
| [Wazuh 4.14.0](wazuh/wazuh.md) | 正式 Agent 与 Manager 的 TLS 注册、密钥、事件与控制消息 |
| [独立 wazuh-agent 实现](wazuh-agent/wazuh-agent.md) | AIDR 原先引用的实验 Agent，包含 HTTP 注册、JWT、事件批次与命令 |

## 认证过程与本地 Demo

每个产品或实现单独成目录，含一份从接口规格提炼的认证过程文档和一个可运行的本地 demo。demo 使用 Python 标准库（Elkeid 另用 `cryptography` 生成临时证书），不连接真实服务。

| 产品/实现 | 认证过程 | 可运行 Demo |
|---|---|---|
| Elastic Agent / Fleet | [认证过程](elastic/authentication.md) | [demo.py](elastic/demo.py) |
| Elkeid | [认证过程](elkeid/authentication.md) | [demo.py](elkeid/demo.py) |
| FleetDM osquery | [认证过程](fleetdm/authentication.md) | [demo.py](fleetdm/demo.py) |
| FleetDM Orbit | [认证过程](fleetdm-orbit/authentication.md) | [demo.py](fleetdm-orbit/demo.py) |
| Wazuh 4.14.0 | [认证过程](wazuh/authentication.md) | [demo.py](wazuh/demo.py) |
| wazuh-agent 实验实现 | [认证过程](wazuh-agent/authentication.md) | [demo.py](wazuh-agent/demo.py) |

在对应目录下运行 `python demo.py`;全部断言通过时输出 `PASS`。demo 只复现凭据流转与协议边界，覆盖范围和简化项见各认证过程文档的末尾说明。

首批范围以代码可溯源、Agent 与接收端配对为准。与 AIDR 对照时分别映射认证注册、平台遥测和文件传输。AIDR 插件进程内外的通信另见 [数据契约](../../archive/integration/aidr-data-contract.md)，平台接口基线见 [端管对接规范](../../archive/integration/aidr-endpoint-platform-v1.md)。各产品文档保留其尚未核实的字段、支路和运行时行为。

| 复核对象 | 复核与修订结果 | 运行条件或协议边界 |
|---|---|---|
| 候选清单 | 核对三个产品的四个提交与 Elastic 两个版本标签 | 深入调研采用首批三个版本基线，其他条目保留为候选 |
| Elastic | 交叉核对注册枚举、check-in 和 Bulk 重试。补充 Kibana 注册令牌管理及固定 System 1.63.0 syslog 样本。Fleet Server 的 apikey 包及四项注册、check-in 和 ACK API 测试通过 | 动态字段按集成包、策略和输出配置决定。未运行完整集群 |
| Elkeid | 交叉核对 Protobuf、Kafka、文件流和采集矩阵。补充服务发现及证书部署。使用双方生成代码执行 Protobuf 互操作测试通过。复核确认上游展示矩阵的 5058 与固定 Volume 生产者存在差异 | 插件映射为开放字典。2001、2003、5101 只核实接收分支或管理端用例，未定位同版本的发送实现。未运行完整 TLS 和 Kafka 部署 |
| FleetDM | 交叉核对密钥存储和重注册、管理端密钥集合、分块、Orbit 及 SCEP。固定 osquery 样本为 5.18.1。提取固定提交的节点密钥请求头解析函数 | 完整服务测试的源码下载发生 Git RPC 中断，本机 Docker Linux Engine 未运行。实际 osquery 版本由部署通道决定 |
| Wazuh | 独立核对 4.14.0 标签与提交、启动与心跳、Agent 侧持久化和实验实现的命令字段 | 正式版专有采集载荷与完整下行控制、实验实现的生产端配对及在线联调未验证 |

## 与 AIDR 的接口对照

| 对照点 | 三种实现的证据 |
|---|---|
| 凭据作用域 | Elastic 将注册、控制和 Elasticsearch 写入凭据分开。FleetDM 区分注册密钥和节点密钥，Orbit 自有节点凭据。Elkeid 的远程连接身份由双端 TLS 校验承载 |
| 遥测确认 | Elastic Bulk 返回逐条状态。FleetDM 日志接口按批确认，文件上传按块确认。Elkeid 遥测流的服务端 Command 不等同于 Kafka 持久化回执 |
| 业务字段边界 | Elastic 数据流由集成包和处理管道决定。FleetDM 查询行由 SQL 列决定。Elkeid `data_type` 是开放数值，插件载荷使用字符串映射，已知类型见采集附录 |

## 1. 整理类似产品与项目清单

- 从 [现有调研记录](../../archive/research/research.md) 和 [参考材料目录](../../../origin/) 提取当前项目已引用或评估的项目，再补充具有可比性的 EDR、终端管理、遥测采集或其他受管 Agent。
- 清单逐项记录产品、组件、类型、与 AIDR 的可比之处、源码或官方协议入口、固定版本或提交，以及可核实的范围。同一产品的 Agent、管理服务和接收服务分别标明，关联到同一产品条目。
- 说明哪些候选进入后续调研及原因。对仅覆盖插件或进程管理的组件，记录其适用范围。

## 2. 调研认证模块

- 按产品梳理首次注册、凭据签发、后续认证、续期或轮换、吊销及重新注册。逐步记录发起方、时序、前置状态、接口或 RPC、认证材料、请求与响应、错误结果和重试条件。
- 分别追踪 Agent 端和管理端的处理。记录凭据的生成、读取、校验和存储，身份绑定、状态迁移、失败处理及对应代码入口。涉及独立签发服务时标明责任归属。
- 提供可与协议定义对照的请求、响应及错误示例，使用占位值或脱敏数据。

## 3. 调研数据面交互协议

- 从 Agent 追踪至实际数据接收端，说明连接方向、传输协议、接口或主题、连接模式、消息编码和版本协商，以及数据面与管理面的边界。
- 逐消息记录封装、事件载荷、批次、确认与错误。核实压缩、顺序、重试、去重、背压和断线恢复等行为，给出代码或协议依据。

## 4. 编制字段与枚举字典

- 覆盖任务 2 和 3 的请求、响应、错误、头部或元数据，以及所有嵌套结构。每个字段记录完整路径、线上名称、类型、字段编号（如适用）、必填性、默认值、约束、单位、语义、脱敏示例、版本及来源。区分缺失、空值和零值的语义。
- 每个枚举逐值列出名称、线上取值或编号、含义和触发条件，并核实保留值及未知值的处理。区分协议定义、实现实际接受的值与配置允许的值。
- 每条结论链接到对应版本的源码、协议定义或官方文档，并给出文件路径与符号。无法核实的内容标为未验证，说明缺失的证据。源码与文档冲突时记录版本、差异和证据。

字段表格采用统一口径：每行给出线上名称、类型与可核实的约束，来源统一放在该节末尾的源码链接中，不逐字段重复版本列。未标注必填性的字段按对应 schema 的 `omitempty` 或零值规则解释；未标注默认值的字段以消费端实际零值行为为准；单位、枚举与缺失语义只在有源码依据时写明。

## 子代理执行与验收

1. 主代理派发项目清单任务，由独立复核子代理检查覆盖范围、项目归属和版本来源。复核完成后固定后续调研清单及版本。
2. 按产品并行派发认证与数据面调研。由其他子代理独立检查接口、交互方向、双方处理逻辑及源码证据。
3. 派发字段和枚举复核任务。复核子代理从同版本的 schema、序列化定义和处理代码独立反查交付物，检查遗漏字段、嵌套类型、枚举值及实际校验。发现差异时退回原子代理修订，再复核受影响条目。
4. 主代理汇总项目清单、逐项目认证时序和接口、数据面协议及字段与枚举字典。保留每项复核结论、修订结果及未验证项。所有已确认结论均须能追溯到对应版本的证据。

实施前若产品范围、数据面边界或可用资料存在影响结论的歧义，由主代理集中向用户确认。
