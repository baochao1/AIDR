# AIDR 管理端接入演示指南

## 开始演示

[打开 Windows 演示启动脚本](../Start-AIDR-Enrollment-Demo.cmd)。双击运行，出现每条 PASS 后按 Enter 继续。演示机须能从命令行运行 Go 和 Python 3.11。脚本会构建程序、启动管理端和 Agent，并在退出时清理演示进程和临时数据。

**演示范围**：当前启动脚本运行的是已验证的 HTTPS/mTLS 注册与心跳 Demo。正式 gRPC 协议见[端管对接规范](aidr-endpoint-platform-v1.md#正式-grpc-服务与编码)，待实现并完成端到端验收后再演示 gRPC 链路。

## 案例：终端注册、首次报到与断线恢复

开场讲解：“这是一台新终端。我们先检查平台如何拒绝无身份或未注册的请求，再让 Agent 完成注册和首次报到，最后重启管理端，验证原 Agent 能恢复报到。”

| 步骤 | 现场操作 | 屏幕上的验收结果 | 讲解词 |
|---|---|---|---|
| 1. 拒绝无证书连接 | 启动脚本，停在 PASS 1/5 | TLS 拒绝未提交客户端证书的连接 | 平台在 TLS 握手阶段验证客户端证书。 |
| 2. 拒绝未注册终端 | 按 Enter，停在 PASS 2/5 | 未注册终端心跳被拒绝，错误注册凭据也被拒绝 | 证书认证通过后，终端还需要完成平台注册。 |
| 3. 注册并报到 | 按 Enter，停在 PASS 3/5 | 终端 demo-01 注册成功，平台显示首次心跳时间 | Agent 完成注册后发送首条心跳，平台把状态持久保存。 |
| 4. 验证权限 | 按 Enter，停在 PASS 4/5 | Agent 证书调用管理员查询接口被拒绝 | 终端身份只具备终端接口权限。 |
| 5. 管理端重启 | 按 Enter，停在 PASS 5/5 | 同一 Agent 恢复心跳，平台记录新的 checkin_at | 注册绑定已持久化，服务重启后终端可以继续报到。 |

完成后按 Enter 查看平台最后一次保存的状态，再按 Enter 关闭窗口。平台记录的 checkin_at 为 UTC。终端没有加载插件，因此 components 为空。

## Agent 首次关联、认证和重连的字段

以下字段对应本演示的 HTTPS/mTLS 接口。正式 gRPC 的证书身份、`Hello`、`Welcome` 和心跳字段见 [端管对接规范](aidr-endpoint-platform-v1.md)。

### 认证材料

| 材料或字段 | 用途 |
|---|---|
| 管理端 HTTPS 地址、可信 CA | Agent 用可信 CA 校验管理端证书，并校验证书是否匹配地址中的主机名或 IP。管理端也配置用于验证客户端证书的可信 CA。 |
| Agent 客户端证书和配对私钥 | Agent 在 TLS 握手中出示证书并证明持有私钥。私钥保留在本地。管理端从证书的 `Subject.CommonName` 读出终端 ID，例如 `demo-01`，并要求 `Subject.OrganizationalUnit` 只有一个值 `agent`。 |
| 一次性注册凭据 | 首次注册时放入请求体。管理端校验后会持久化客户端证书指纹，后续心跳按该指纹校验。 |

### 请求与响应

| 阶段 | 请求字段 | 成功响应 |
|---|---|---|
| 首次注册 `POST /v1/agents/enroll` | 请求头 `Content-Type: application/json`。请求体是一个 JSON 字符串，例如 `"example-token"`。终端 ID 取自客户端证书。 | `200 OK`，响应体为空。同一张已绑定的证书重复调用时也返回 `200 OK`。 |
| 首次和后续报到 `POST /v1/agents/checkin` | 请求头 `Content-Type: application/json`。请求体包括 `checkin_at`（管理端会将其覆盖为实际接收时间）、`received_revision`（已接收策略版本）、`applied_revision`（已应用策略版本）、`components`（以组件 ID 为键的状态对象）。本演示两项版本均为 `0`，组件为空对象 `{}`。 | `200 OK`，响应是当前目标策略，字段为 `revision` 和 `components`。本演示没有下发策略，返回 `{"revision":0,"components":null}`。策略数组元素字段为 `id`、`version`、`desired_state`、`artifact_id`。 |
| 管理端重启后恢复报到 | 同一个运行中的 Agent 用原证书重新建立 mTLS 连接，继续发送同样结构的 `POST /v1/agents/checkin`。Agent 已在内存记录注册成功，管理端从原数据库读取证书绑定。 | `200 OK`，返回当前目标策略。管理端保存新的 UTC `checkin_at`。 |
| 管理员查询 `GET /v1/admin/agents/{id}` | 路径参数 `id` 为终端 ID，例如 `demo-01`。使用 `OU=admin` 的客户端证书，请求体为空。 | `200 OK`，响应为保存的 `checkin_at`、`received_revision`、`applied_revision`、`components`。演示最后打印的 JSON 就是这个查询结果。 |

心跳请求的 `components` 有数据时，每个组件包含 `state`，并按实际情况携带 `version`、`error`、`pid`、`ipc_transport`、`desired_version`、`data`。`data` 包含 `state`，以及可能出现的 `last_event_at` 和 `error`。

缺少客户端证书时，连接在 TLS 握手阶段失败。角色不匹配或终端尚未绑定时，接口返回 `403`。注册凭据错误时，注册接口返回 `400`，响应正文为 `invalid enrollment token`。

## 命令行复跑

在仓库根目录执行以下命令。自动模式连续运行五项验证，现场逐项暂停时在 Python 命令末尾加 --present。

~~~powershell
go build -o build/aidr-enrollment-demo.exe ./cmd/supervisor-demo
python scripts/aidr-enrollment-demo.py build/aidr-enrollment-demo.exe
~~~
