# Supervisor 与第三方 AIDR 组件架构设计

## 1. 目标与职责

Windows、Linux 和 macOS 终端部署 Supervisor（下文称 Agent），由管理平台追加、升级或移除独立的能力组件。厂商 AIDR 负责识别本地智能体、观测日志、发现 Skill/MCP 并产生检测或风险结果。Agent 负责验证软件来源、管理进程、接收并可靠传递数据。管理平台负责策略、制品分发、接收记录及后续的资产与风险视图。

厂商交付独立程序时，程序直接实现本机组件协议。厂商只交付 SDK 时，由独立适配器进程加载 SDK，并对 Agent 提供相同接口。适配器持有厂商 SDK 资源，Agent 核心独立发布。

## 2. 部署与模块边界

~~~mermaid
flowchart LR
    subgraph Platform[管理侧]
      Admin[管理员 CLI 或 Demo 界面]
      Server[管理端 API]
      Registry[已签名制品仓库]
      MS[(策略、状态、事件 SQLite)]
      Admin -->|管理员 mTLS 发布制品和策略| Server
      Server --> Registry
      Server --> MS
    end
    subgraph Device[终端]
      Control[Agent 管理客户端]
      Coordination[组件协调器与进程监督]
      Packages[验签与版本目录]
      State[(Agent 状态与队列 SQLite WAL)]
      IPC[本机 gRPC 客户端]
      Adapter[独立 AIDR 适配器 EXE]
      SDK[厂商 SDK 或原生能力]
      Control --> Coordination
      Coordination --> Packages
      Coordination --> State
      Coordination --> IPC
      IPC -->|Health 与 Events · TLS| Adapter
      Adapter --> SDK
      IPC -->|事件入队| State
    end
    Control -->|出站 mTLS 注册、check-in、下载| Server
    State -->|Agent 出站上传 · HTTPS mTLS + zstd| Server
~~~

| Demo 模块 | 主要职责 | 当前实现 |
|---|---|---|
| 入口与身份引导 | 启动管理端或 Agent，签名、发布、下发策略及查询状态，生成角色隔离的测试凭据 | [main.go](../../../cmd/supervisor-demo/main.go)、[bootstrap.go](../../../cmd/supervisor-demo/bootstrap.go) |
| 管理端与制品仓库 | 接收注册、保存期望策略、提供签名包、记录实际状态与接收事件 | [http.go](../../../internal/demo/http.go)、[events.go](../../../internal/demo/events.go) |
| Agent 管理客户端 | 独立执行 check-in 和异步下载，处理收到的策略 revision | [agent_control.go](../../../internal/demo/agent_control.go) |
| 制品校验和版本存储 | 校验 Ed25519 签名、摘要和平台，检查 ZIP 路径，按版本内容隔离安装。启动、复用和回滚前复检文件摘要 | [artifact.go](../../../internal/demo/artifact.go) |
| Agent 协调器和进程运行时 | 计算期望与实际状态差异，执行安装、启停、升级、回滚和卸载。每进程独立探活，故障预算限流 | [agent.go](../../../internal/demo/agent.go)、[Windows Job Object](../../../internal/demo/process_windows.go)、[Unix 进程组](../../../internal/demo/process_unix.go) |
| 组件协议与本机传输 | TLS 实例证书与 token、Health、双向 EventStream。Windows 命名管道，Linux/macOS Unix socket，TLS 回环 TCP 作兼容选项 | [component.go](../../../internal/component/component.go)、[Events Proto](../../../api/proto/component/v1/events.proto)、[原生 IPC 说明](native-ipc.md) |
| 事件接入与投递 | 终端落盘后回复事件 ID，流断开后同实例重连，队列批量上传，管理端持久去重 | [events.go](../../../internal/demo/events.go)、[state.go](../../../internal/demo/state.go) |
| 演示与验证 | 图形化展示能力、进程和采集状态，实测两种语言的组件与端到端生命周期 | [Windows Agent Console](../../../scripts/aidr-agent-console.py)、[端到端脚本](../../../scripts/e2e.py) |

## 3. 关键链路

**控制与制品。** 管理端登记签名包并为终端发布带 revision 的期望状态。Agent 通过出站 check-in 接收策略，验证制品后安装并启动组件。升级先准备新版本，新实例探活成功后提交；失败则验证旧版本并尝试恢复。普通日志和业务数据留在组件数据目录。已接收 revision、最近已应用 revision、实际版本与运行状态分别上报。

**本机 IPC。** Agent 启动厂商适配器时提供实例地址、连接方式和凭据。插件在该地址监听一个 gRPC 服务端，同一连接承载 Health 和 Events。Agent 主动探活、打开事件流，插件发现事件后沿流发送。Windows 和 Unix 原生模式在插件侧不需要 TCP 监听端口，管理端 HTTPS 仍独立运行。

**事件交付。** Agent 为插件 payload 添加可信的组件实例信息和事件 ID，写入终端 SQLite 后回复 ACK。随后独立批量 zstd 压缩，经 mTLS 上传。管理端以终端身份和事件 ID 去重并持久化，返回成功后 Agent 清理对应队列记录。当前保留厂商 payload 原文，AIDR 分类解析和附件上传见后续方案。

~~~mermaid
sequenceDiagram
    participant M as 管理端
    participant A as 终端 Agent
    participant P as 厂商插件
    A->>M: Check-in，发送已接收与已应用 revision
    M-->>A: 新策略和制品标识
    A->>M: 下载签名包
    A->>A: 验签、校验平台及安装路径
    A->>P: 启动进程，传入本机 IPC 参数
    A->>P: Health.Check，随后打开事件流
    P-->>A: 发送发现或检测事件
    A->>A: 事件持久化
    A->>P: 返回 event_id
    A->>M: 批量上传
    M-->>A: 持久接收确认
~~~

## 4. 初期方案和开源参考如何落到模块

最初的《统一客户端第三方插件 SDK 集成设计方案》提供了独立适配器与厂商 SDK 的集成思路，落到 **厂商组件运行时、启动契约和组件工作目录**。《开源 EDR 框架选型评估报告》提供了多能力隔离、版本切换、持久队列和过载处理的思路，落到 **协调器、制品管理、可靠事件队列和可观测性**。检测业务保持在厂商组件边界内。

下表中的开源参考是经固定源码审查后的设计映射。参考设计、参与 PoC 和实际链接库分别列出。

| 开源参考 | 参考的机制 | 映射到本项目哪一层 | 采用方式 |
|---|---|---|---|
| Elastic Agent | 期望组件模型、Coordinator、进程与服务运行时分层 | Agent 协调器、实际状态上报和平台进程适配 | 设计参考。当前只交付进程运行时 |
| Elastic Agent Client | 期望/观察状态和动作分别建模 | Agent 状态契约和组件协议分层 | 设计参考。本项目采用标准 Health 与自定义 Events |
| Fleet Server | 注册、check-in、策略与 ACK | 管理 API、mTLS 客户端、策略 revision 状态 | 设计参考。临时动作 ACK 尚未实现 |
| HashiCorp go-plugin | 子进程握手、gRPC、AutoMTLS、传输切换 | 本机 IPC 的 A/B 选型和实例连接设计 | 仅由 ipc-poc 链接。实际运行时采用自研启动协议与 grpc-go |
| Nomad | 运行时分离、进程句柄、恢复、等待和检查 | Windows/Unix 进程适配与插件健康监督 | 设计参考。未引入 Nomad Client |
| Go supervisord | 启停、探活阈值、连续失败后的恢复 | Health 探测、进程监视、重启退避和预算 | 设计参考 |
| Mender | 更新的准备、切换、提交与回滚阶段 | 版本目录与升级失败恢复 | 设计参考。完整事务日志和全阶段恢复待补 |
| Mender Artifact | 签名、版本化打包、制品校验 | manifest、Ed25519、ZIP 哈希及缓存复核 | 设计参考。本项目定义自己的制品格式 |
| Elkeid | 独立插件进程、消息通道和运行状态 | 第三方适配器、gRPC 本机通信和插件监督 | 设计参考 |
| Wazuh Agent | 持久队列、容量与批次配置 | SQLite 队列、背压、批次和上传重试 | 设计参考 |
| MiNiFi C++ | 心跳和控制通道 | Agent check-in 与离线后重连 | 补充设计参考 |
| FleetDM / Orbit | 软件包下载、安装和执行结果上报 | 签名发布、出站下载与状态展示 | 补充设计参考 |

固定版本、完整 commit、源码入口和许可信息统一见 [源码调研与来源记录](../research/research.md)。gRPC 选型、A/B 实测和后续平台适配见 [IPC ADR](../adr/0001-component-ipc.md)。

### 实际链接的主要开源库

| 库 | 模块用途 |
|---|---|
| grpc-go、protobuf-go | 本机 Health、Events、消息序列化与 TLS |
| Microsoft/go-winio、golang.org/x/sys | Windows 命名管道、Job Object，及平台底层接口 |
| modernc.org/sqlite | 管理端与终端状态、WAL、发送队列和接收去重记录 |
| klauspost/compress | Agent 上传前压缩事件批次，管理端解压 |
| grpcio、PyInstaller | Python Mock 和独立打包样例 |
| hashicorp/go-plugin | 独立 IPC PoC |

版本和许可证见 [依赖清单](dependencies.md) 与 [go.mod](../../../go.mod)。

## 5. 当前验收和下一阶段边界

Windows amd64 本机和 Linux amd64 WSL2 已验证安装、健康检查、双向事件、启停、卸载、升级失败回滚和断网续传。两端均验证两个原生 IPC 组件并行上报且插件进程没有 TCP socket。三端 amd64、arm64 Go 构建通过。macOS、ARM64 的原生运行仍待验收。

正式部署还需要服务身份和目录 ACL、跨账号 IPC、实例凭据一次性传递、配置下发及生效回执、完整升级事务恢复和多终端授权。AIDR 数据分类、资产关联和大文件受控上传见 [AIDR 数据契约](../integration/aidr-data-contract.md)。厂商开发步骤见 [厂商接入说明](../integration/aidr-vendor-handoff.md)，故障边界见 [加固与验收](../delivery/agent-hardening.md) 和 [验收报告](../delivery/acceptance.md)。
