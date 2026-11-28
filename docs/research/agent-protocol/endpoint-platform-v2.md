# AIDR Agent 与管理端协议

Agent 首次上线顺序为 **远程认证 → 心跳与接收策略 → 下载并校验制品 → 上报遥测与上传附件**。本稿只覆盖 Agent 与管理端及部署侧的交互；Agent 与插件的本机接入细节由 AIDR 终端实现决定，不在本稿范围。各表的来源列注明 AIDR 定义出处和可比产品的线上字段。「Elkeid 对应」列以固定提交 `c906c512` 为准，🟩 表示已有对应字段或机制，⬜ 表示在对应源码或调研范围内未核实。写有“对照”的项表示行为相近，线上名称、取值或作用域按表内说明。产品版本及上游源码见各节末尾的来源链接。

| 认证发生在哪一步 | 谁发起、谁验证 | 使用的凭据 |
|---|---|---|
| 第 1.1 节，Agent 首次接入或重连管理端 | Agent 主动连接管理端，双方在 TLS 握手中验证证书 | 部署侧提供的客户端证书与管理端证书 |

## 1. AIDR Agent → 管理端认证与上线

**AIDR 独立实现客户端和服务端，参考 Elkeid 固定提交 `c906c512` 的流程。** 部署侧用现成 CA 签发证书，两端启动代码把证书交给 Go TLS 和 grpc-go。双向 TLS 通过后，Agent 发首包，管理端按首包里的 `agent_id` 登记在线连接。Elkeid 的源码只用于核对参考行为。

### 1.1 证书认证

#### 部署提供哪些配置

下表中的参数名对应本节 AIDR Go 示例。配置文件的外层格式由实际接入工程确定。示例采用部署路径读取 PEM。部署侧先完成签发，Agent 启动后直接建连。

| 谁使用 | 配置参数 | 启动时怎么用 |
|---|---|---|
| Agent | `address`、`serverName` | `address` 决定拨到哪里，`serverName` 必须匹配服务端证书中的 DNS 名 |
| Agent | `clientCertFile`、`clientKeyFile` | 加载设备的客户端证书和匹配私钥。握手时出示证书，私钥只在本机使用 |
| Agent | `serverCAFile` | 用这份 CA 验证管理端证书 |
| 管理端 | `listener`、`serverCertFile`、`serverKeyFile` | 在指定地址监听，用服务端证书和私钥配置整个 gRPC 服务 |
| 管理端 | `clientCAFile` | 用这份 CA 验证 Agent 证书，配置 `RequireAndVerifyClientCert` 要求客户端证书 |

#### AIDR 开发者要写的启动代码

下面这段代码**由 AIDR 开发者自己编写**，不是 Go 库的改动：Agent 端开发人员在 Agent 工程内实现 `dialAgentCenter`，管理端开发人员在管理端工程内实现 `serveAgentCenter`（两端各自实现，示例为便于对照放在同一片段）。代码只调用 Go 标准库 `crypto/tls`、`crypto/x509` 和 grpc-go 的公开 API，把部署配置里的证书路径加载后交给库；TLS 握手和证书验证由 Go TLS 标准库在拨号/监听时自动完成，开发者不需要也不应该改 Go 库本身。

Agent 在首次拨号前调用 `dialAgentCenter`，管理端在开始监听前调用 `serveAgentCenter`。这两处负责读取部署配置并把凭据交给库。握手和验证过程全部在库内部，业务代码只在返回值里拿到结果（连接成功或错误）。

~~~go
// 本文件是 AIDR 开发者编写的业务启动代码（非 Go 库改动）。
// Agent 端在 Agent 工程内实现，管理端在管理端工程内实现；
// 示例为便于对照放在同一片段，实际交付时按端拆分到各自工程。
package endpoint

import (
	"context"
	"crypto/tls" // Go 标准库：TLS 握手与证书验证由它完成
	"crypto/x509"
	"fmt"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// 共用辅助函数：Agent 端和管理端各自需要一份（或放入两端共用的内部包）。
// 只是把部署配置里的 PEM 文件读成库需要的类型，不含端特定逻辑。
func loadTLSFiles(certFile, keyFile, caFile string) (tls.Certificate, *x509.CertPool, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("load certificate and key: %w", err)
	}

	pem, err := os.ReadFile(caFile)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("read CA file: %w", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pem) {
		return tls.Certificate{}, nil, fmt.Errorf("no valid CA certificate in %s", caFile)
	}
	return cert, roots, nil
}

// ========== Agent 端代码（Agent 工程内实现，Agent 开发人员负责） ==========
// Agent 在首次拨号前调用本函数。它构造客户端 TLS 配置并交给 grpc-go，
// 实际握手与证书验证由 Go TLS 标准库在 DialContext 内部完成。
func dialAgentCenter(ctx context.Context, address, serverName,
	clientCertFile, clientKeyFile, serverCAFile string,
) (*grpc.ClientConn, error) {
	cert, serverCAs, err := loadTLSFiles(clientCertFile, clientKeyFile, serverCAFile)
	if err != nil {
		return nil, err
	}

	// 客户端侧 TLS 配置：
	// - Certificates 出示部署侧签发的客户端证书；
	// - RootCAs/ServerName 由 Go TLS 标准库验证管理端证书。
	cfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      serverCAs,
		ServerName:   serverName,
	}
	return grpc.DialContext(ctx, address,
		grpc.WithTransportCredentials(credentials.NewTLS(cfg)),
		grpc.WithBlock(),
	)
}

// ========== 管理端代码（管理端工程内实现，管理端开发人员负责） ==========
// 管理端在开始监听前调用本函数。它构造服务端 TLS 配置并挂在 gRPC 服务器上，
// 对 Agent 证书的要求和验证由 Go TLS 标准库在 server.Serve 内部完成。
func serveAgentCenter(listener net.Listener, serverCertFile, serverKeyFile,
	clientCAFile string, registerTransfer func(grpc.ServiceRegistrar),
) error {
	cert, clientCAs, err := loadTLSFiles(serverCertFile, serverKeyFile, clientCAFile)
	if err != nil {
		return err
	}

	// 服务端侧 TLS 配置：
	// - Certificates 出示管理端证书；
	// - RequireAndVerifyClientCert + ClientCAs 要求并验证 Agent 证书。
	cfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    clientCAs,
	}
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(cfg)))
	registerTransfer(server)
	return server.Serve(listener)
}
~~~

- **Agent 调用** `dialAgentCenter` 时，传入带超时的 `ctx`、拨号地址、服务端证书上的 DNS 名、客户端证书和私钥路径，以及签发服务端证书的 CA 路径。阻塞拨号成功后，用返回的 `conn` 创建 AIDR 根据协议定义生成的 Transfer 客户端。
- **管理端调用** `serveAgentCenter` 时，传入监听器、服务端证书和私钥路径，以及签发客户端证书的 CA 路径。`registerTransfer` 的实现调用生成代码的 `pb.RegisterTransferServer`，传入已初始化的 Transfer 处理器。TLS 配在 `grpc.NewServer` 上，业务处理器和在线连接池无需加载证书。
- 客户端的 `RootCAs` 和 `ServerName` 启用 Go 的标准服务端验证。它承担 Elkeid 固定版客户端验证回调的职责。服务端的 `RequireAndVerifyClientCert` 和 `ClientCAs` 直接使用同样的 Go TLS 选项。 [Elkeid 客户端配置][elkeid-dial] · [Elkeid 服务端配置][elkeid-tls]

#### Agent 用加载好的 TLS 配置发送请求（示例片段）

下面片段说明 Agent 端开发人员拿到 `dialAgentCenter` 返回的 `conn` 之后怎么发请求。要点：TLS 凭据已经在拨号时交给 grpc-go，之后通过这条 `conn` 打开的**每一条 RPC 路径**（包括 `/grpc.Transfer/Transfer`）都自动走这条已认证的 TLS 连接；业务代码发送的只是 Protobuf 消息，不需要再附加证书或 token。“开放路径”指的是调用生成代码的客户端方法，gRPC 按方法名把请求送到对应 RPC 路径。

~~~go
// ========== Agent 端代码（示例片段，Agent 工程内实现） ==========

// 1. 拨号。返回时 TLS 握手与双向证书验证已完成（grpc.WithBlock 阻塞等待）。
conn, err := dialAgentCenter(ctx, cfg.Address, cfg.ServerName,
	cfg.ClientCertFile, cfg.ClientKeyFile, cfg.ServerCAFile)
if err != nil {
	return fmt.Errorf("dial agent center: %w", err)
}

// 2. 用这条连接创建协议生成代码里的客户端。
//    此后它发出的所有 RPC 都自动复用这条已认证的 TLS 连接。
transferClient := pb.NewTransferClient(conn)

// 3. 打开双向流。生成代码内部会请求 RPC 路径 "/grpc.Transfer/Transfer"，
//    请求帧仍走上面这条 TLS 连接，无需再配任何凭据。
stream, err := transferClient.Transfer(ctx, grpc.UseCompressor("snappy"))
if err != nil {
	return fmt.Errorf("open transfer stream: %w", err)
}

// 4. 发送首包：取出 agent_id 等上线信息（第 1.2 节字段表）。
err = stream.Send(&pb.PackagedData{
	AgentID:      agentID,
	IntranetIpv4: intranetIPv4,
	Hostname:     hostname,
	Version:      buildVersion,
	Product:      productName,
})
if err != nil {
	return fmt.Errorf("send first packet: %w", err)
}

// 5. 之后同一个循环里继续收发：
//    - stream.Send(...) 发心跳批次、事件（第 2 节 Heartbeat、第 4 节 EventBatch）
//    - stream.Recv()   收管理端下行（第 2 节 HeartbeatReply、PolicyUpdate）
//    消息仍是普通 Protobuf，TLS 加密传输由 Go 标准库在底层完成。
~~~

#### 拨号时执行证书认证

AIDR 业务代码调用上面的 `dialAgentCenter`，成功后取得连接。Elkeid 源码中的对应调用如下。 [客户端调用][elkeid-send]

~~~go
conn, err := connection.GetConnection(ctx)
~~~

`specified_region` 缺省为 `default`。先请求服务发现 `GET /registry/detail?name=hids_svr_grpc&count=10`，用响应中的 `data[].ip` 和 `port` 拨号。尝试失败后按区域回退到私网、公网静态地址。单次拨号超时 15 秒。 [地址选择][elkeid-dial]

服务发现、私网、公网三个分支只提供候选地址，都调用 `grpc.DialContext(..., dialOptions...)`。下面是服务发现分支里的实际拨号代码。 [三条拨号分支][elkeid-dial]

~~~go
c, err = grpc.DialContext(context, addr, dialOptions...)
if err == nil {
    conn.Store(c)
    NetMode.Store("sd")
    atomic.StoreInt32(&retries, 0)
    return c, nil
}
~~~

| 场景 | 何时验证证书 |
|---|---|
| 首次连接，或切换候选地址新建连接 | `grpc.WithBlock()` 等待连接就绪。Agent 校验管理端证书，管理端校验 Agent 证书。验证失败的候选连接无法进入拨号成功分支 |
| 已有 `Ready` 连接 | 直接复用已经认证的连接。新开的 RPC、同一流上的首包和心跳都通过这条连接发送 |
| 返回缓存的 `Idle` 连接对象 | `GetConnection` 此时可以先返回对象。后续 RPC 触发底层建连时，gRPC 使用对象已有的 TLS 凭据，证书验证在建连时发生 |
| 断线后重新建连 | 重拨使用同一套 TLS 配置。Agent 再打开 Transfer 并发送首包，AgentCenter 重新登记在线连接 |

证书认证发生在新建底层连接时。业务消息复用已认证的连接，无需逐条附证书、JWT 或 Cookie。前置的服务发现 HTTP 请求使用各自的连接配置。

#### 服务端证书配在监听器

AIDR 示例通过 `grpc.NewServer(grpc.Creds(...))` 配置监听器。作为来源对照，Elkeid 的 `runServer` 先取得含双向 TLS 的凭据，将其作为 gRPC **服务器选项**。下面是同一函数内的实际代码节选。启动服务时调用 `runServer(true, ...)`，因此走 `enableCA` 分支。 [监听器配置与服务注册][elkeid-tls]

~~~go
if enableCA {
	ct := credential(crtFile, keyFile, caFile)
	if ct == nil {
		ylog.Errorf("RunServer", "####GET_CREDENTIAL_ERROR")
		os.Exit(-1)
	}
	opts = append(opts, grpc.Creds(ct))
}
server := grpc.NewServer(opts...)
pb.RegisterTransferServer(server, &grpc_handler.TransferHandler{})
~~~

同一个函数随后调用 `server.Serve(lis)` 在 TCP 监听器上接收连接。**证书配置挂在 `grpc.NewServer(opts...)` 上。** 注册在这个服务实例上的 Transfer 和文件 RPC 都经由它接受连接，无需给每个接口再配置一份证书。这里的 `GlobalGRPCPool` 是业务在线连接池。它在握手通过、gRPC 分派请求、Transfer 收到首包之后，才保存 `agent_id → 在线流` 的关系，也无需配置证书。若另外启动一个独立的 gRPC 监听器，该监听器的 TLS 配置要在自己的启动入口提供。 [服务端监听][elkeid-tls] · [业务连接池登记][elkeid-transfer]

Elkeid 上游实际的客户端和服务端 TLS 配置代码已单独摘录，用于查证：[Elkeid 上游实际 TLS 配置代码](elkeid/elkeid-tls-config.md)。

### 1.2 Agent 接入与上线

| Agent 启动数据 | 来源与用途 |
|---|---|
| `agent_id` | 优先读取 `SPECIFIED_AGENT_ID`。否则尝试由云实例、DMI 或网卡标识派生。进入备用路径时读取 `/etc/machine-id`、工作目录 `machine-id`，最后才生成 UUID。非显式配置分支会尝试写回工作目录的 `machine-id`。固定版部分硬件材料分支可能留下空 ID，详见源码。 [ID 初始化][elkeid-id] |
| 主机信息、版本、产品名 | 本机模块提供主机名和 IP，构建注入版本，Agent 代码定义产品名。数据随首包发送。 [Agent 批次组装][elkeid-send] |

#### 打开 Transfer 双向流

连接就绪后，Agent 调用生成的客户端打开 Transfer。 [实际客户端调用][elkeid-send]

~~~go
client, err = proto.NewTransferClient(conn).Transfer(subCtx, grpc.UseCompressor("snappy"))
~~~

完整路径由生成代码传给 gRPC。 [生成的客户端源码](https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/agent/proto/grpc.pb.go)

~~~go
stream, err := c.cc.NewStream(ctx, &_Transfer_serviceDesc.Streams[0], "/grpc.Transfer/Transfer", opts...)
~~~

#### 首包字段与样例

Elkeid **没有专门的首包或注册消息**：首包就是 Agent 开流后第一个非空批次，之后每个批次结构完全相同。`agent/transport/transfer.go` 的发送策略为：`handleSend` 用 100 毫秒定时器轮询 `buffer.ReadEncodedRecords()`，缓存为空不发；取到记录后将记录列表、本地 ID（`agent.ID`）、私网和公网 IPv4/IPv6（`host.*` 原子变量）、主机名（`host.Name`）、Agent 版本和产品名组装成 `PackagedData` 发送。发送失败仅记日志，记录对象回收进缓冲池，该批数据不重排队。连接级重试在外层 `startTransfer`：取不到连接等 5 秒重试，超过 5 次退出；流中断后等 5 秒重开。 [Agent 批次构造][elkeid-send] · [消息定义][elkeid-proto] · [服务端首包处理][elkeid-transfer]

`PackagedData`（proto 字段号：`records` 1、`agent_id` 2、`intranet_ipv4` 3、`extranet_ipv4` 4、`intranet_ipv6` 5、`extranet_ipv6` 6、`hostname` 7、`version` 8、`product` 9）。其中每条记录是 `EncodedRecord{data_type int32 #1, timestamp int64 #2, data bytes #3}`，`data` 是编码（传输层配 snappy 压缩）后的 `Payload{fields map<string,string> #1}` 字节。下面是字段名采用 Proto 定义的可读示例。示例批次里的 `records[0].data` 以 Base64 展示 Protobuf 字节，解码后是 `Payload.fields={ "state": "running" }`。实际心跳按生产端填入其他可用字段。

~~~json
{
  "records": [
    {
      "data_type": 1000,
      "timestamp": "1795744800",
      "data": "ChAKBXN0YXRlEgdydW5uaW5n"
    }
  ],
  "agent_id": "agent-001",
  "intranet_ipv4": ["192.0.2.2"],
  "extranet_ipv4": [],
  "intranet_ipv6": [],
  "extranet_ipv6": [],
  "hostname": "test-host",
  "version": "1.2.3",
  "product": "elkeid-agent"
}
~~~

这里的 `timestamp` 是 Unix 秒，Proto 类型是 `int64`，JSON 展示用十进制字符串。

#### 服务端登记与返回数据

| 时点 | AgentCenter 具体动作 | Agent 在网上看到什么 |
|---|---|---|
| 收到 `/grpc.Transfer/Transfer` 请求和第一条 `PackagedData` | gRPC 分派到 `TransferHandler.Transfer`。服务端先取并发配额，再调用 `stream.Recv()` 读第一条消息。取出 `agent_id`、对端 IP:端口，记录服务端创建时间，为这条流创建下行队列与取消上下文，按 `agent_id` 加入 `GlobalGRPCPool`。重复 ID 加入失败时，当前 RPC 返回错误 | 加入池是服务端内部操作，无单独注册 ACK、Token 或连接 ID 响应。服务端随后处理首批记录，继续在原流上收发。 [Transfer 处理函数][elkeid-transfer] |
| 首次收到并成功解析 `data_type=1000` 心跳 | `parseAgentHeartBeat` 初始化在线详情，用**服务端时间**刷新 `last_heartbeat_time`。首次初始化时调用 `PostLatestConfig(agentID)`，把 ID 放入配置队列；队列工作线程用 agent_id 和在线详情向管理端取当前配置，再将 `Command` 送入该连接的下行队列 | 成功取到并发出配置时，Agent 在同一条流上 `Recv()` 到一个 `Command`。这是异步业务下行，取配置失败会在服务端记录错误，不能把它当作 TLS 或上线确认。 [心跳消费][elkeid-heartbeat-consume] · [配置队列和下行][elkeid-pool] |
| 流退出 | Transfer 清理在线池条目并释放心跳指标和连接配额 | Agent 下次建连重新做 TLS，然后重新发送首个业务批次。 [Transfer 清理][elkeid-transfer] |

下面是 Elkeid 固定提交中的实际 Go 方法。它运行在双向 TLS 已验证的 gRPC 流上，依赖同文件中的 recvData 和 sendData，以及 Elkeid 的 GlobalGRPCPool。 [上游实现][elkeid-transfer]

~~~go
func (h *TransferHandler) Transfer(stream pb.Transfer_TransferServer) error {
	//Maximum number of concurrent connections
	if !GlobalGRPCPool.LoadToken() {
		err := errors.New("out of max connection limit")
		ylog.Errorf("Transfer", err.Error())
		return err
	}
	defer func() {
		GlobalGRPCPool.ReleaseToken()
	}()

	//Receive the first packet and get the AgentID
	data, err := stream.Recv()
	if err != nil {
		ylog.Errorf("Transfer", "Transfer error %s", err.Error())
		return err
	}
	agentID := data.AgentID

	//Get the client address
	p, ok := peer.FromContext(stream.Context())
	if !ok {
		ylog.Errorf("Transfer", "Transfer error %s", err.Error())
		return err
	}
	addr := p.Addr.String()
	ylog.Infof("Transfer", ">>>>connection addr: %s", addr)

	//add connection info to the GlobalGRPCPool
	ctx, cancelButton := context.WithCancel(context.Background())
	createAt := time.Now().UnixNano() / (1000 * 1000 * 1000)
	connection := pool.Connection{
		AgentID:     agentID,
		SourceAddr:  addr,
		CreateAt:    createAt,
		CommandChan: make(chan *pool.Command),
		Ctx:         ctx,
		CancelFuc:   cancelButton,
	}
	ylog.Infof("Transfer", ">>>>now set %s %v", agentID, connection)
	err = GlobalGRPCPool.Add(agentID, &connection)
	if err != nil {
		ylog.Errorf("Transfer", "Transfer error %s", err.Error())
		return err
	}
	defer func() {
		ylog.Infof("Transfer", "now delete %s ", agentID)
		GlobalGRPCPool.Delete(agentID)
		releaseAgentHeartbeatMetrics(agentID)
	}()

	//Process the first of data
	handleRawData(data, &connection)

	//Receive data from agent
	go recvData(stream, &connection)
	//Send command to agent
	go sendData(stream, &connection)

	//stop here
	<-connection.Ctx.Done()
	return nil
}
~~~

后续管理端推配置时，通过 `agent_id` 找到这条连接，把命令放进它的 `CommandChan`，由 `sendData` 调用 `stream.Send()` 发给 Agent。下面示例是 Agent 解码后看到的 `Command`，只展示插件名和版本，取值为示例。完整配置还可包含包类型、摘要、签名、下载 URL 等字段。Agent 收到后按名称索引配置并进入同步处理。服务端内部等待的是 `stream.Send()` 的结果，插件是否实际应用成功需另看状态。 [连接池发送][elkeid-pool] · [Command 字段](elkeid/elkeid.md#字段字典) · [Agent 接收][elkeid-send]

~~~json
{
  "configs": [
    {
      "name": "example-plugin",
      "version": "1.2.3"
    }
  ]
}
~~~

| 场景 | Elkeid 行为 |
|---|---|
| 首包发送前断开 | 服务端读取首包失败，本次连接未进入在线连接池 |
| 同一个 `agent_id` 已经在线 | 连接池返回冲突，保留正在使用的连接 |
| 已接入的流断开 | 清理连接和心跳指标。Agent 重连时重新握手并发送首包 |

[elkeid-server-config]: https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/server/agent_center/common/init.go
[elkeid-dial]: https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/agent/transport/connection/connection.go
[elkeid-tls]: https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/server/agent_center/grpctrans/grpc_server.go
[elkeid-id]: https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/agent/agent/id.go
[elkeid-proto]: https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/agent/proto/grpc.proto
[elkeid-transfer]: https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/server/agent_center/grpctrans/grpc_handler/transfer_handler.go
[elkeid-send]: https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/agent/transport/transfer.go

[elkeid-heartbeat-consume]: https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/server/agent_center/grpctrans/grpc_handler/rawdata_worker.go
[elkeid-pool]: https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/server/agent_center/grpctrans/pool/pool.go

## 2. Agent ↔ 管理端心跳与管控

**业务契约。** 第 1 节的 TLS 和 Transfer 首包完成认证及上线。下表记录 AIDR 管控需要传递的业务字段，如何映射到 Elkeid `PackagedData.records` 和 `Command` 见本节 Proto 草案。表中及下文的 JSON 示例是消息内容的可读表示；**正式线上的封装以下面的 Proto 草案为准**。整数必须在 0–2^53−1 范围内。网络错误、UNAVAILABLE、RESOURCE_EXHAUSTED 采用 1–60 秒带抖动的指数退避。

#### 数据结构（Proto 草案，参照 Elkeid Transfer 实现）

照 Elkeid `agent/proto/grpc.proto` 的实现模式设计：**单条双向 `Transfer` 流承载心跳与管控**，上行帧携带设备身份字段（与 Elkeid `PackagedData` 逐字段对应），下行帧承载指令（对应 Elkeid `Command`）；业务负载不再用 `data_type` 判别，改用类型化消息和 `oneof` 分派。遥测批次按第 4 节约定走独立的 `Telemetry` 流（对应 Elkeid `Transfer` + `FileExt` 的多 service 布局）。

~~~proto
syntax = "proto3";

package aidr.endpoint.v1;

// Elkeid 对应标注说明（对照固定提交 c906c512 的 agent/proto/grpc.proto）：
//   [Elkeid 有]   = 上游已有同名字段或同职责机制，注释给出对应消息/字段；
//   [Elkeid 无]   = 上游未见对应，AIDR 新增。
// Elkeid 原型 service：
//   rpc Transfer(stream PackagedData) returns (stream Command) {}   [Elkeid 有]
//   message PackagedData { records #1, agent_id #2, intranet_ipv4 #3,
//     extranet_ipv4 #4, intranet_ipv6 #5, extranet_ipv6 #6,
//     hostname #7, version #8, product #9 }
//   message Command { task Task #2, configs repeated Config #3 }
service Transfer {
  rpc Transfer(stream Uplink) returns (stream Downlink) {} // [Elkeid 有] 同为单条双向流，消息名与分派方式不同
}

// [Elkeid 无] 遥测独立 service。布局方式对照 Elkeid 的双 service（Transfer + FileExt）。
service Telemetry {
  rpc Report(stream EventBatch) returns (stream EventBatchReply) {}
}

// ========== 上行帧：对应 Elkeid PackagedData [Elkeid 有] ==========
// 设备身份 8 个字段每帧都带，与 Elkeid PackagedData 逐字段对应；
// Elkeid 的 records 批次在 AIDR 改为 oneof 类型化负载，
// 不再用 data_type 判别和 EncodedRecord{data_type #1, timestamp #2, data bytes #3}。
message Uplink {
  string agent_id = 1;               // [Elkeid 有] PackagedData.agent_id #2
  repeated string intranet_ipv4 = 2; // [Elkeid 有] PackagedData.intranet_ipv4 #3
  repeated string extranet_ipv4 = 3; // [Elkeid 有] PackagedData.extranet_ipv4 #4
  repeated string intranet_ipv6 = 4; // [Elkeid 有] PackagedData.intranet_ipv6 #5
  repeated string extranet_ipv6 = 5; // [Elkeid 有] PackagedData.extranet_ipv6 #6
  string hostname = 6;               // [Elkeid 有] PackagedData.hostname #7
  string version = 7;                // [Elkeid 有] PackagedData.version #8
  string product = 8;                // [Elkeid 有] PackagedData.product #9

  oneof payload {
    // [Elkeid 有对应机制] 心跳藏在 records 里，借 data_type=1000 的 EncodedRecord 分派；
    // AIDR 改为独立类型化消息，携带 boot_id/seq/策略版本/组件状态。
    Heartbeat heartbeat = 10;
    // [Elkeid 无] 策略回执。对账机制对照 Elkeid Task.token #4。
    PolicyAck policy_ack = 11;
  }
}

// ========== 下行帧：对应 Elkeid Command{task #2, configs #3} [Elkeid 有] ==========
message Downlink {
  oneof payload {
    // [Elkeid 无] 逐心跳确认。Elkeid 无 ack_seq，首个心跳触发 PostLatestConfig 下发配置。
    HeartbeatReply heartbeat_reply = 1;
    // [Elkeid 有对应机制] 对应 Command.configs（repeated Config）的角色。
    Policy policy_update = 2;
  }
  // [Elkeid 有但 AIDR 首轮不设] Elkeid Command.task（Task{data_type #1,
  // object_name #2, data #3, token #4}）承载通用任务；AIDR 首轮只下发生命周期目标，
  // 通用任务与插件热配置需独立协议。
}

// ========== 心跳 [Elkeid 有对应机制：data_type=1000 EncodedRecord，藏于 records] ==========
message Heartbeat {
  string boot_id = 1;                  // [Elkeid 无] UUID，每次 Agent 进程启动生成
  int64 seq = 2;                       // [Elkeid 无] 同一次启动从 1 递增
  string timestamp = 3;                // [Elkeid 有对照] EncodedRecord.timestamp #2；
                                       // Elkeid 为 int64 Unix 秒，AIDR 用 RFC3339 字符串
                                       // 对齐本节 JSON 示例，冻结前可改 int64
  int64 received_revision = 4;         // [Elkeid 无] 已保存的策略版本，初始 0
  int64 applied_revision = 5;          // [Elkeid 无] 已应用的策略版本，初始 0
  repeated ComponentState components = 6; // [Elkeid 无本结构] Agent/插件状态在 Elkeid 另行分派
}

message ComponentState {
  string component_id = 1;             // [Elkeid 无] Elkeid 用插件名（Config.name）定位
  string state = 2;                    // [Elkeid 有对照] 上游有状态枚举，取值集合独立；
                                       // AIDR 取 running / stopped / absent / degraded / failed
  string version = 3;                  // [Elkeid 无] 可选：实际运行或已安装版本
  string error_code = 4;               // [Elkeid 无] failed 时必填
}

// ========== 心跳回执 [Elkeid 无整条消息] ==========
message HeartbeatReply {
  int64 ack_seq = 1;                   // [Elkeid 无] 已接收的心跳 seq
  string server_time = 2;              // [Elkeid 无本字段] Elkeid 心跳时间用服务端 Unix 秒刷新
  int32 heartbeat_interval_s = 3;      // [Elkeid 无] 下次心跳间隔（秒）
  int32 offline_after_s = 4;           // [Elkeid 无] 离线阈值（秒）
  int64 desired_revision = 5;          // [Elkeid 无] 目标策略版本
  Policy policy = 6;                   // [Elkeid 有对应机制] 首个心跳触发配置下发
                                       //（PostLatestConfig → Command.configs）
}

// ========== 策略（管控）[Elkeid 有对应机制：Command.configs] ==========
message Policy {
  int64 revision = 1;                  // [Elkeid 无] Elkeid Config.version #3 是语义版本
                                       // 字符串，无整数发布版本
  repeated ComponentTarget components = 2; // [Elkeid 有对照] Command.configs 数组
}

message ComponentTarget {
  string component_id = 1;             // [Elkeid 有对照] Config.name #1 定位目标
  string desired_state = 2;            // [Elkeid 无] running / stopped / absent；
                                       // Elkeid 靠下发/撤销配置隐式表达，无三值目标
  string artifact_id = 3;              // [Elkeid 无] 非 absent 时必填；Elkeid 由
                                       // Config.download_urls #6 直接给地址，
                                       // AIDR 只传 ID，下载细节走第 3 节 Artifact RPC
  // [Elkeid 有但 AIDR 不入管控流] Config 的 sha256 #4、signature #5、
  // type #2、detail #7 属于下载与校验细节，AIDR 移到 Artifact RPC。
}

// ========== 策略回执 [Elkeid 无整条消息；对账机制对照 Task.token #4] ==========
message PolicyAck {
  int64 revision = 1;                  // [Elkeid 无] 按 revision 对账（Elkeid 按 token）
  string status = 2;                   // [Elkeid 无] received / applied / rejected / failed
  string error_code = 3;               // [Elkeid 无] rejected 或 failed 时必填：
                                       // INVALID_POLICY / COMPONENT_APPLY_FAILED
}

// ========== 遥测批次 [Elkeid 有对照：PackagedData.records 批量提交] ==========
message EventBatch {
  string batch_id = 1;                 // [Elkeid 无] UUID，批次对账；原批重传保持值
  repeated TelemetryEvent events = 2;  // [Elkeid 有对照] records 数组批量承载；
                                       // AIDR 限 1–200 条，单条至多 16 KiB
}

message TelemetryEvent {
  string type = 1;                     // [Elkeid 有对照] EncodedRecord.data_type #1 判别
                                       // 记录类型（int32）；AIDR 用插件 AIDREvent.event_type
  int32 schema_version = 2;            // [Elkeid 无] 当前固定 1，对应 aidr.plugin.v1
  string source_event_id = 3;          // [Elkeid 无] 插件源事件 ID = AIDREvent.event_id
  string observed_at = 4;              // [Elkeid 有对照] EncodedRecord.timestamp #2 为
                                       // Unix 秒；AIDR 用插件 timestamp（Unix 毫秒）转 UTC RFC3339
  string asset_id = 5;                 // [Elkeid 无] 对应 AIDREvent.asset_id / Asset.asset_id
  string data_json = 6;                // [Elkeid 有对照] EncodedRecord.data #3 编码后的
                                       // Payload{fields map<string,string> #1}；AIDR 用 JSON object 文本，
                                       // 字段名与类型沿用 AIDREvent 未提升到信封的字段
  // Agent 附加字段（均 [Elkeid 无]，平台按租户+Agent+event_id 去重）
  string event_id = 7;                 // 本地落盘 UUID，与 source_event_id 不同
  string component_id = 8;             // 插件 plugin_id；插件留空时由 Agent 补齐
  string instance_id = 9;
  int64 sequence = 10;                 // 本地持久化序号，从 1
  string timestamp = 11;               // 接收时间，UTC RFC3339（Elkeid 无接收时间字段）
}

// ========== 遥测回执 [Elkeid 无整条消息：上游无逐条/逐批回执] ==========
message EventBatchReply {
  string batch_id = 1;                 // [Elkeid 无] 对应请求批次
  repeated EventResult results = 2;    // [Elkeid 无] 逐事件回执
}

message EventResult {
  string event_id = 1;                 // [Elkeid 无] 对应请求中的一条事件
  string status = 2;                   // [Elkeid 无] accepted / duplicate / rejected / retry
  string error_code = 3;               // [Elkeid 无] rejected / retry 时必填
}
~~~

| kind / 方向 | body 字段与语义 | 来源与产品字段对照 | Elkeid 对应 |
|---|---|---|---|
| Heartbeat → | `boot_id` UUID，每次 Agent 进程启动生成，`seq` 整数，同一次启动从 1 递增。 `timestamp` UTC，`received_revision`、`applied_revision` 为已保存、已应用的策略版本，初始 0，`components` 为组件状态数组 | AIDR。Elkeid 借 `data_type=1000` 心跳记录更新在线信息，Wazuh 4.14.0 使用 keepalive，Elastic Check-in 接收 `components`。 | 🟩 有 `data_type=1000` 心跳，以 Unix 秒记录时间。Agent 及插件状态另外分派，未见本行 `boot_id`、`seq` 和策略版本。 [心跳](elkeid/elkeid.md#字段字典) |
| HeartbeatReply ← | `ack_seq` 整数，对应已接收心跳。 `server_time`、`heartbeat_interval_s`、`offline_after_s`、`desired_revision` 分别表示服务端时间、下次心跳间隔（秒）、离线阈值（秒）和目标策略版本。可选 `policy`，策略正文见下 | AIDR。Elastic Check-in 通过 `ack_token` 对账，响应可附带 `actions`；AIDR 用 `ack_seq` 和 `policy`。 | ⬜ 未见 `ack_seq` 或逐心跳确认。首个 Agent 心跳可触发配置下发。 [心跳处理](elkeid/elkeid.md#认证身份和连接生命周期) |
| PolicyUpdate ← | `revision` 正整数，发布版本。 `components` 组件目标数组，每项 `component_id` string、`desired_state` 枚举，非 absent 时必填 `artifact_id` string（获授权制品），absent 时省略 | AIDR 生命周期字段。Elastic `actions[].data.policy.revision` 同样使用整数版本。Elkeid `Config` 有插件版本和下载信息。 | 🟩 有 `Command.configs` 和 `Config.name/version`。未见本行整数 revision 或三值 desired_state。 [Proto][elkeid-proto] |
| PolicyAck → | `revision` 正整数。 `status` 枚举。 `error_code` string，仅 rejected 或 failed 必填，取 `INVALID_POLICY` 或 `COMPONENT_APPLY_FAILED` | AIDR 精简回执。Elastic `events[].action_id/error` 上报动作结果，Elkeid `Task.token` 用于任务对账。 | 🟩 有按 `Task.token` 对账的任务模型。未见本行 PolicyAck 和四态语义。 [Proto][elkeid-proto] |

组件状态数组每项包含 `component_id` string、`state` string、可选 `version` string（实际运行或已安装版本）和 `error_code` string（failed 时必填）。`component_id` 对应插件 `GetStatusResponse.plugin_id`，`version` 对应 `GetStatusResponse.version`。插件运行时状态按如下方式写入 `state`：插件 `ready` → `running`，`busy` → `running`，`degraded` → `degraded`，插件异常或 `plugin.error` → `failed`；插件 `code` 映射为本稿 `error_code`（`ERR_CHECKSUM` → `INVALID_POLICY`，其余不可重试失败 → `COMPONENT_APPLY_FAILED`）；`ERR_BUSY` 由 Agent 按插件约定退避重试，不写 `failed`。插件 `plugin.heartbeat` 的 `cpu_percent` / `mem_bytes` / `queue_size` 等统计不进 `ComponentState`，按第 4 节遥测原样上报。首轮只下发生命周期目标，任意命令和插件热配置需独立协议。

| 枚举 | 值与业务语义 | 来源与产品字段对照 | Elkeid 对应 |
|---|---|---|---|
| desired_state | `running` 安装并启动，`stopped` 安装但停止，`absent` 卸载 | AIDR 生命周期目标。参考产品未验证同一组三值。 | ⬜ 未见相同三值。Elkeid 使用 `Command.configs` 下发组件配置。 [Proto][elkeid-proto] |
| state | `running` 就绪，`stopped` 已安装且停止，`absent` 未安装，`degraded` 在运行但能力降级，`failed` 安装、启动、停止或卸载未达到目标 | AIDR 本稿的精简状态；`degraded` 对应插件 `GetStatusResponse.status` / `plugin.status` 的 `degraded`。Elastic Check-in `status` 为 `starting/online/degraded/error`，业务含义不同。 | 🟩 有状态枚举，AIDR 取值另行定义。 [Elkeid 心跳与状态](elkeid/elkeid.md#字段字典) |
| PolicyAck.status | `received` 已持久保存，待执行。 `applied` 所有目标已达到。 `rejected` 校验失败，从未接收该版本。 `failed` 已接收但应用失败，具体组件状态随心跳上报 | AIDR 本稿规定接收、应用与失败分支。Elastic 有独立 ACK 入口，但没有本表四值。 | ⬜ 未见该四态策略回执。 [Proto][elkeid-proto] |

设备身份取自第 1 节 Transfer 首包。以下是 AIDR 心跳业务请求和回执数据示例。

~~~json
[
  {
    "kind": "Heartbeat",
    "body": {
      "boot_id": "00000000-0000-4000-8000-000000000001",
      "seq": 1,
      "timestamp": "2026-11-27T02:00:00Z",
      "received_revision": 0,
      "applied_revision": 0,
      "components": []
    }
  },
  {
    "kind": "HeartbeatReply",
    "body": {
      "ack_seq": 1,
      "server_time": "2026-11-27T02:00:00Z",
      "heartbeat_interval_s": 30,
      "offline_after_s": 90,
      "desired_revision": 4,
      "policy": {
        "revision": 4,
        "components": [
          {
            "component_id": "aidr-vendor",
            "desired_state": "running",
            "artifact_id": "aidr-vendor-1.4.0"
          }
        ]
      }
    }
  }
]
~~~

Agent 保存后回 `{ "revision": 4, "status": "received" }`，完成制品校验和插件探活后回 `{ "revision": 4, "status": "applied" }`。后续心跳的 applied_revision 为 4。

`PolicyAck` 的来源是插件侧的 `policy.ack` 事件与 `UpdatePolicyResponse.accepted`：Agent 下发 `UpdatePolicyRequest.version` 时使用 `Policy.revision` 的十进制字符串，插件回送 `policy_version` 后 Agent 转回整数填入 `PolicyAck.revision`。`policy_result=applied` → `applied`；`rejected` 或 `accepted=false` → `rejected` + `INVALID_POLICY`；`partial` → `failed` + `COMPONENT_APPLY_FAILED`。插件事件中的原始 `code` / `message` 保留在第 4 节遥测 `data` 中供定位。插件 `action.receipt` 不占用本节的 `PolicyAck`，按第 4 节遥测上报。

| 异常 | 处理 |
|---|---|
| 心跳回执丢失 | 重连后沿用 boot_id，原 seq 和正文重发。平台确认重复序号，重复帧不刷新在线计时 |
| 旧策略重复推送 | 版本低于已接收版本时忽略。同版本同内容重发已知状态，同版本不同内容拒绝 |
| 策略非法或应用失败 | 分别回 rejected 或 failed。applied_revision 保持上次完整成功版本 |
| 超过 offline_after_s 无新心跳 | 平台标为离线。Agent 重连后发送新心跳并重报实际组件状态 |
| 连接首包读取失败 | 按第 1 节的 Elkeid Transfer 行为处理 |

**来源**：[Elastic Fleet Check-in / ACK](elastic/elastic.md) · [Elkeid 心跳和指令](elkeid/elkeid.md) · [Wazuh 4.14.0 启动与 keepalive](wazuh/wazuh.md) · [实验 wazuh-agent 命令](wazuh-agent/wazuh-agent.md)。本节采用 AIDR 帧名；PolicyAck 和组件字段为精简联调口径，参考 Fleet 的控制回执与 Elkeid 的长流。

## 3. Agent 下载与校验插件制品

Agent 从已授权策略取得 artifact_id，主动访问制品服务。复用签名更新组件的选型参考 Fleet Orbit 使用的 TUF。若部署采用 TUF，直接交由其客户端处理签名元数据、过期、版本回滚和目标摘要。本节定义 AIDR 自有的下载接口，供 TUF 集成评估前使用。下载与第 5 节上传的类型化消息定义见本节末尾的 Proto 草案；冻结时与第 2 节的 Transfer/Telemetry 合入同一个 proto 文件。

制品下载走 `/aidr.endpoint.v1.Artifact/Download`，一元请求、服务端流响应。客户端发 `ArtifactRequest`，服务端先发一个 `ArtifactHeader`，再按偏移升序发送 `ArtifactChunk`。

| 消息字段 | 类型与校验 | 来源与产品字段对照 | Elkeid 对应 |
|---|---|---|---|
| ArtifactRequest.body.artifact_id | string，策略引用的不可变制品 ID | AIDR 制品下载契约。Orbit 通过 TUF 校验和获取更新目标。 | ⬜ 未见该 RPC 字段。Elkeid 通过 `Config.download_urls` 提供下载地址。 [Proto][elkeid-proto] |
| ArtifactHeader.body.descriptor.artifact_id | string，必须匹配请求 | AIDR 制品下载契约。Orbit 通过 TUF 校验和获取更新目标。 | ⬜ 固定版下载配置未核实对应字段。 [Proto][elkeid-proto] |
| descriptor.manifest.id / version | string，目标组件 ID 与制品版本 | Elkeid `Config.name` / `Config.version` 对应插件名与版本；AIDR 在签名 manifest 中使用 `id` / `version`。 | 🟩 `Config.name` / `Config.version` 对应目标名和版本。 [Proto][elkeid-proto] |
| descriptor.manifest.os / arch | string，首轮 windows / amd64 | AIDR 制品下载契约。Orbit 通过 TUF 校验和获取更新目标。 | ⬜ 固定版下载配置未核实对应字段。 [Proto][elkeid-proto] |
| descriptor.manifest.entrypoint | string，ZIP 根目录下的可执行文件相对路径 | AIDR 制品下载契约。Orbit 通过 TUF 校验和获取更新目标。 | ⬜ 固定版下载配置未核实对应字段。 [Proto][elkeid-proto] |
| descriptor.manifest.runtime | string，`process` 表示由 Agent 管理的独立进程 | AIDR 制品下载契约。Orbit 通过 TUF 校验和获取更新目标。 | ⬜ 固定版下载配置未核实对应字段。 [Proto][elkeid-proto] |
| descriptor.sha256 / size / key_id | string、非负整数、string。分别是 ZIP 摘要、字节数及可信发布公钥 ID | Elkeid `Config.sha256` 校验包内容；Fleet Orbit TUF 校验目标摘要和大小。AIDR `key_id` 定位发布公钥。 | 🟩 有 `Config.sha256`。未见同组 `size` 和 `key_id`。 [Proto][elkeid-proto] |
| ArtifactHeader.body.signature | string，Ed25519 签名的 Base64，签名输入为 Descriptor 的归一化序列化 | AIDR 使用 Ed25519 描述签名。Fleet Orbit 使用 TUF 签名元数据，与 AIDR 签名输入不同。 | 🟩 有 `Config.signature` 字段。该字段存在，未据此确认 AIDR 的 Ed25519 签名输入兼容。 [Proto][elkeid-proto] |
| ArtifactChunk.body.offset | 非负整数，从 0 开始，必须等于已接收字节数 | AIDR 制品下载契约。Orbit 通过 TUF 校验和获取更新目标。 | ⬜ 固定版下载配置未核实对应字段。 [Proto][elkeid-proto] |
| ArtifactChunk.data | Protobuf bytes，原始 ZIP 字节，每帧最多 1 MiB | AIDR 制品下载契约。Orbit 通过 TUF 校验和获取更新目标。 | ⬜ 固定版下载配置未核实对应字段。 [Proto][elkeid-proto] |

首轮制品目标为 `os=windows`、`arch=amd64`。例如策略指定 aidr-vendor-1.4.0，Agent 下载并校验签名元数据、长度及摘要，再安装到版本独立目录并完成组件接入，确认目标状态后回 applied；组件接入细节不在本稿范围。失败时返回策略 failed 以及组件 error_code。网络中断时退避重下。制品不存在或当前身份不可见时返回 NOT_FOUND，Agent 记录 ARTIFACT_MISSING。摘要、签名、目标平台不匹配时丢弃临时包并报告 ARTIFACT_INVALID。启动失败时恢复上一个可运行实例。

#### 上传下载消息定义（Proto 草案）

本节下载与第 5 节上传共用一组类型化消息，放同一个 proto（package `aidr.endpoint.v1`），不拆多文件：同一条 mTLS 连接、消息仅十余条，冻结时与第 2 节的 `Transfer`/`Telemetry` 合入同一个 `endpoint.proto`。这两组 service 属于 Agent 与管理端之间的交互——制品由 AIDR 打包工具生成、平台托管，附件上传是 Agent 与管理端之间的行为，与第三方厂商无接口约定。

~~~proto
syntax = "proto3";

package aidr.endpoint.v1;

// Elkeid 对应标注说明（对照固定提交 c906c512 的 agent/proto/grpc.proto）：
//   [Elkeid 有]     = 上游已有同名字段或同职责机制；
//   [Elkeid 有对照] = 机制存在但类型/方向/取值不同；
//   [Elkeid 无]     = 上游未见对应，AIDR 新增。

// ========== 制品下载（本节） ==========
// 一元请求、服务端流：先发一条 ArtifactHeader，再按偏移升序发 ArtifactChunk。
// [Elkeid 无] Elkeid 无独立下载 RPC，下载地址由 Config.download_urls #6 提供。
service Artifact {
  rpc Download(ArtifactRequest) returns (stream DownloadFrame) {}
}

message ArtifactRequest {
  string artifact_id = 1; // [Elkeid 无] 策略引用的不可变制品 ID
}

// 服务端流帧：必发首帧 header，之后按偏移升序发数据块。
message DownloadFrame {
  oneof payload {
    ArtifactHeader header = 1; // 必发首帧
    ArtifactChunk chunk = 2;   // 后续数据帧
  }
}

message ArtifactHeader {
  Descriptor descriptor = 1; // [Elkeid 无]
  string signature = 2;      // [Elkeid 有对照] Ed25519 签名 Base64，签名输入为
                             // Descriptor 的归一化序列化；Elkeid 的 Config.signature #5
                             // 字段存在但签名输入未核实，不兼容
}

message Descriptor {
  Manifest manifest = 1; // [Elkeid 无]
  string sha256 = 2;     // [Elkeid 有对照] ZIP 摘要，小写十六进制；对应 Elkeid
                         // Config.sha256 #4（语义相同，用于下载校验）
  uint64 size = 3;       // [Elkeid 无] ZIP 字节数
  string key_id = 4;     // [Elkeid 无] 可信发布公钥 ID，定位 Ed25519 公钥
}

message Manifest {
  string id = 1;              // [Elkeid 有对照] 目标组件 ID；对应 Config.name #1
  string version = 2;         // [Elkeid 有对照] 制品版本；对应 Config.version #3
  string os = 3;              // [Elkeid 无] 首轮固定 windows
  string arch = 4;            // [Elkeid 无] 首轮固定 amd64
  string entrypoint = 5;      // [Elkeid 无] ZIP 根目录下可执行文件相对路径
  string runtime = 6;         // [Elkeid 无] process = Agent 管理的独立进程
  reserved 7, 8;              // [Elkeid 无] 字段号保留，本稿不含本机插件接口
}

message ArtifactChunk {
  uint64 offset = 1; // [Elkeid 无] 从 0 开始，必须等于已接收字节数
  bytes data = 2;    // [Elkeid 有对照] 原始 ZIP 字节，每帧最多 1 MiB；字节载荷对照
                     // Elkeid FileUploadRequest.data #2（方向相反：那是上传）
}

// ========== 附件上传（第 5 节） ==========
// 四个一问一答 RPC。附件就绪后 Agent 建上传任务，等关联事件在平台确认后再上传。
// 会话与设备绑定；相同 file_id 和文件元数据复用会话，同 ID 不同内容返回 ALREADY_EXISTS。
// [Elkeid 有对照] Elkeid 用 FileExt/Upload 单条文件字节流（FileUploadRequest.token #1
// 关联任务，流结束返回 FileUploadResponse.SUCCESS/FAILED）；AIDR 改为显式会话、
// 逐块偏移确认和块摘要。
service Evidence {
  rpc CreateUpload(CreateUploadRequest) returns (UploadSession) {} // [Elkeid 无] 显式建会话
  rpc GetUpload(GetUploadRequest) returns (UploadSession) {}       // [Elkeid 无] 查询服务端偏移后续传
  rpc PutChunk(UploadChunk) returns (UploadOffset) {}              // [Elkeid 有对照] 对照 Elkeid 的
                                                                   // token+data 逐帧上传，AIDR 增加偏移和块摘要
  rpc Complete(CompleteUploadRequest) returns (UploadComplete) {}  // [Elkeid 有对照] 对照 Elkeid 流结束的
                                                                   // SUCCESS/FAILED，AIDR 改为显式提交调用
}

message CreateUploadRequest {
  string file_id = 1;      // [Elkeid 有对照] UUID，文件标识；关联机制
                           // 对照 FileUploadRequest.token #1
  string kind = 2;         // [Elkeid 无] 首轮固定 scan_report（扫描报告）
  uint64 size_bytes = 3;   // [Elkeid 无] 实际字节数
  string sha256 = 4;       // [Elkeid 无] 整个文件 SHA-256；Elkeid 已核实的 sha256
                           // 在下载侧（Config.sha256 #4），上传侧无对应
  string event_id = 5;     // [Elkeid 无] UUID，平台关联事件
  string content_type = 6; // [Elkeid 无] application/octet-stream（二进制）/
                           // application/json（JSON）/ text/plain（纯文本）
}

message GetUploadRequest {
  string upload_id = 1; // [Elkeid 无]
}

// CreateUpload 和 GetUpload 的共同响应；GetUpload 不返回 event_id/content_type。
message UploadSession {
  string upload_id = 1;  // [Elkeid 无] UUID，上传会话标识
  string file_id = 2;    // [Elkeid 无] 对应请求的文件标识
  uint64 offset = 3;     // [Elkeid 无] 已持久化的下一字节（续传起点）
  uint64 size_bytes = 4; // [Elkeid 无] 文件总字节数
  uint32 chunk_size = 5; // [Elkeid 无] 单块字节上限（正整数）
  string expires_at = 6; // [Elkeid 无] UTC RFC3339，会话过期时间
  string status = 7;     // [Elkeid 有对照] uploading（可继续写入）/
                         // complete（已校验并提交）；对照 Elkeid
                         // FileUploadResponse.SUCCESS/FAILED（仅流结束一次性返回）
}

message UploadChunk {
  string upload_id = 1;    // [Elkeid 无]
  uint64 offset = 2;       // [Elkeid 无] 必须等于服务端已确认偏移
  string chunk_sha256 = 3; // [Elkeid 无] 块摘要，小写十六进制
  bytes data = 4;          // [Elkeid 有对照] 1–chunk_size 字节；对照
                           // FileUploadRequest.data #2（无偏移、无块校验）
}

message UploadOffset {
  uint64 offset = 1; // [Elkeid 无] 服务端已持久化的下一字节
}

message CompleteUploadRequest {
  string upload_id = 1; // [Elkeid 无]
}

message UploadComplete {
  string file_id = 1; // [Elkeid 无] 校验通过并已提交
  string status = 2;  // [Elkeid 有对照] 固定 complete；对照 Elkeid
                      // FileUploadResponse.SUCCESS/FAILED
}
~~~

**来源**：[Fleet Orbit TUF 实现说明](https://github.com/fleetdm/fleet/blob/3807bd79fedbd25424ae55738b03cc8a4ae28a80/orbit/README.md) · [Elkeid Config 下载字段](elkeid/elkeid.md#字段字典)。

## 4. Agent → 平台遥测

Agent 开 `/aidr.endpoint.v1.Telemetry/Report` 双向流，消息结构见第 2 节 Proto 草案的 `EventBatch` / `EventBatchReply`。Agent 从持久队列发送，平台确认持久化后才允许清队列。每批 1–200 条，单条至多 16 KiB，解压后每批至多 1 MiB。

遥测载荷来自插件 `AIDREvent`；插件侧 PB、事件类型与字段定义见 [Agent 底座 ↔ 插件 Protobuf 协议](agent-plugin-protobuf-v1.md)。本稿只定义端管信封和映射规则，不重复定义插件业务字段。

| 字段 | 类型与含义 | 来源与产品字段对照 | Elkeid 对应 |
|---|---|---|---|
| body.batch_id | UUID，批次对账。原批重传保持值，重新组批生成新值 | AIDR 批次 ID。Elkeid `PackagedData` 与 FleetDM/osquery 日志均可批量提交，批次跟踪字段不同。 | ⬜ 固定版 Transfer schema 未见该字段或枚举。 [Proto][elkeid-proto] |
| body.events[] | 事件信封对象：`type` string（取插件 `AIDREvent.event_type`）、`schema_version` 整数（当前固定 1，对应 `aidr.plugin.v1`）、`source_event_id` string（取插件 `AIDREvent.event_id`，同一终端同一插件内跨重启唯一，重传保持原值及正文）、`observed_at` UTC RFC3339（由插件 `timestamp` 毫秒值转换）、`asset_id` string（取插件 `AIDREvent.asset_id`，资产类事件必填）、`data` object（字段名与类型沿用 `AIDREvent` 未提升到信封的字段，按 type 校验），并附加下列 Agent 字段。事件类型清单以插件 PB 文档为准，不在本稿重列 | AIDR。Elkeid `PackagedData.records` 承载多条记录，Elastic Bulk 批次承载多条文档。 | 🟩 `PackagedData.records` 为批量记录。 [Proto][elkeid-proto] |
| events[].event_id | UUID，Agent 本地落盘 ID，与 `source_event_id` 不同；平台按租户、Agent 和 event_id 去重，重传保持原值 | AIDR。Elastic Bulk `_id` 可对照平台文档标识和逐项对账。 | ⬜ 固定版 Transfer schema 未见该字段或枚举。 [Proto][elkeid-proto] |
| events[].component_id / instance_id | string，Agent 指定的组件与实例；`component_id` 对应插件 `plugin_id`（插件留空时由 Agent 用其注册 ID 补齐），`instance_id` 为 Agent 本地实例标识，无插件对应字段 | AIDR 遥测契约。 | ⬜ 固定版 Transfer schema 未见该字段或枚举。 [Proto][elkeid-proto] |
| events[].sequence / timestamp | 整数和 UTC 时间，分别为 Agent 本地持久化序号（从 1）与接收时间；`timestamp` 对应插件 `received_at_ms` 的时间点（插件未填时用 Agent 读到该事件的本地时间） | AIDR 序号和接收时间。Elkeid `records[].timestamp` 和 Elastic `@timestamp` 可对照时间字段，时间来源与单位各有定义。 | 🟩 部分对应。`records[].timestamp` 为 Unix 秒，未见本行序号。 [Proto][elkeid-proto] |
| body.results[].event_id | UUID，对应请求中的一条事件 | AIDR。Elastic Bulk 用 `items[].create._id` 对账。 | ⬜ 固定版 Transfer schema 未见该字段或枚举。 [Proto][elkeid-proto] |
| body.results[].status | string，`accepted` 已在平台持久化，`duplicate` 已持久化相同事件，`rejected` 永久拒绝，`retry` 暂未确认 | AIDR 自有枚举。Elastic Bulk `items[].create.status` 逐项返回 HTTP 风格状态。 | ⬜ 固定版 Transfer schema 未见该字段或枚举。 [Proto][elkeid-proto] |
| body.results[].error_code | 仅 rejected / retry 必填。 `INVALID_EVENT`、`UNSUPPORTED_SCHEMA`、`EVENT_ID_CONFLICT` 为永久拒绝，`STORAGE_UNAVAILABLE` 为暂态 | AIDR 本稿明确永久/暂态错误。Elastic Bulk 各条失败含 `error` 详情。 | ⬜ 固定版 Transfer schema 未见该字段或枚举。 [Proto][elkeid-proto] |

#### 插件 AIDREvent → TelemetryEvent 映射

Agent 把一条插件 `AIDREvent` 转成一条 `TelemetryEvent`：六个字段（`event_id`、`event_type`、`timestamp`、`asset_id`、`plugin_id`、`received_at_ms`）提升到信封或由 Agent 补齐，其余字段进入 `data`。插件 proto 大版本升级时先提升 `schema_version`，再更新 `data` 校验规则。

| TelemetryEvent 字段 | 插件来源 | 转换与约束 |
|---|---|---|
| `type` | `AIDREvent.event_type` | 原值透传，不加工前缀 |
| `schema_version` | 插件 proto 大版本 | 当前 1，对应 `aidr.plugin.v1` |
| `source_event_id` | `AIDREvent.event_id` | UUID 原值；同一事件的原批重传保持该值 |
| `observed_at` | `AIDREvent.timestamp` | Unix 毫秒转 UTC RFC3339，保留毫秒，例如 `2026-11-27T02:00:00.000Z` |
| `asset_id` | `AIDREvent.asset_id` | 与插件 `Asset.asset_id` 为同一标识，Agent 不重新生成 |
| `data` | `AIDREvent` 其余字段 | JSON object；字段名、类型沿用插件 PB；未用字段省略，proto3 默认值省略。`data_json` 是这段 object 的序列化文本 |
| `component_id` | `AIDREvent.plugin_id` / `GetStatusResponse.plugin_id` | 插件留空时由 Agent 用其注册 ID 补齐 |
| `timestamp` | `AIDREvent.received_at_ms` 或 Agent 接收时间 | UTC RFC3339；插件未填时用 Agent 读到该事件的本地时间 |
| `event_id` / `sequence` | Agent 本地 | 无插件对应字段；分别是平台去重键与本地序号 |
| `instance_id` | Agent 本地 | 无插件对应字段 |

`data` 按 `event_type` 填写的字段分组如下，完整字段定义以插件 PB 文档为准：

| 事件分组 | `data` 中常见字段 |
|---|---|
| 通用归因 | `risk_level`、`user_id`、`session_id`、`agent_id`、`agent_name`、`tenant_id`、`trace_id`、`session_reason` |
| 行为 | `behavior_type`、`event_kind`、`rule_id`、`rule_name`、`attack_type`、`action`、`severity`、`alert_id`、`process_name`、`process_pid`、`process_path`、`command_line`、`prompt_digest`、`tool_name`、`target_url`、`target`、`evidence`、`description` |
| 工具链路 | `chain_id`、`step_id`、`parent_step_id`、`relation_confirmed`、`step_type`、`tool_type`、`server_name`、`args_digest`、`result_digest`、`result_status`、`started_ms`、`ended_ms`、`step_count`、`total_duration_ms`、`end_status`、`risk_event_count` |
| 扫描 `scan.result` | `scan_id`、`target_type`、`target_name`、`conclusion`、`report_digest`、`findings`（`id` / `severity` / `desc` / `file`） |
| 策略与处置回执 | `policy_id`、`policy_version`、`policy_result`、`effect`、`effective_at_ms`、`action_id`、`action_type`、`action_result`、`code`、`executed_at_ms`、`status`、`message` |
| 插件状态与心跳 | `status`、`state`、`uptime_s`、`total_events`、`dropped_count`、`queue_size`、`cpu_percent`、`mem_bytes`、`error_msg` |

`plugin.status` / `plugin.heartbeat` 同时用于第 2 节组件状态映射；`policy.ack` 同时用于 `PolicyAck`；`action.receipt` 只走本节遥测。插件 `ExecuteAction` 与 `UpdatePolicy` 是本机交互，不占用端管 Downlink RPC。

Agent 发送 EventBatch，body 如下；示例事件来自插件 `AIDREvent`（`event_type=scan.result`），`data` 字段沿用插件 PB 命名。

~~~json
{
  "batch_id": "00000000-0000-4000-8000-000000000090",
  "events": [
    {
      "type": "scan.result",
      "schema_version": 1,
      "source_event_id": "evt-6f3a9b21-8c4d-4e17-9a02-1b7c5d0e3f44",
      "observed_at": "2026-11-27T02:00:00.000Z",
      "asset_id": "a-7d21f0c4b9e4",
      "data": {
        "risk_level": "medium",
        "user_id": "zhangsan",
        "session_id": "sess-8f3e1c",
        "agent_id": "agent-qwen-01",
        "agent_name": "qwen-agent",
        "scan_id": "scan-42",
        "target_type": "skill",
        "target_name": "skill-7",
        "conclusion": "谨慎使用",
        "report_digest": "sha256:9f8e2c...",
        "findings": [
          {
            "id": "SK-01",
            "severity": "medium",
            "desc": "权限范围需要复核",
            "file": "skills/skill-7/SKILL.md"
          }
        ]
      },
      "event_id": "00000000-0000-4000-8000-000000000042",
      "component_id": "fangcun-observer",
      "instance_id": "instance-21",
      "sequence": 1,
      "timestamp": "2026-11-27T02:00:01Z"
    }
  ]
}
~~~

平台返回 EventBatchReply，body 如下。

~~~json
{
  "batch_id": "00000000-0000-4000-8000-000000000090",
  "results": [
    {
      "event_id": "00000000-0000-4000-8000-000000000042",
      "status": "accepted"
    }
  ]
}
~~~

收到 accepted 或 duplicate 清除对应待发项。rejected 停止发送并记录原因。retry、缺失结果或流中断时保留对应事件，维持 event_id 重传。响应里的未知状态视为未确认。批次帧无法解码时返回 INVALID_ARGUMENT，修正封装后再发送。重连上报独立于事件来源。

**来源**：[Elastic 8.17.0 Bulk 逐条结果](elastic/elastic.md) · [Elkeid 双向流及 Kafka 边界](elkeid/elkeid.md)。AIDR 使用自己的回执枚举和去重键。

## 5. Agent 附件上传

附件的来源与就绪方式由 AIDR 终端实现决定，不在本稿范围。Agent 在附件就绪后建立上传任务，等关联事件在平台确认后再上传。上传接口走 Agent 的 mTLS，方法前缀为 `/aidr.endpoint.v1.Evidence/`，对应 RPC 为 `CreateUpload → UploadSession`、`GetUpload → UploadSession`、`PutChunk → UploadOffset`、`Complete → UploadComplete`，消息结构见第 3 节 Proto 草案的 Evidence service。

| Evidence RPC | 请求字段 | 成功响应字段 | 来源与产品字段对照 | Elkeid 对应 |
|---|---|---|---|---|
| CreateUpload | file_id、kind、size_bytes、sha256 同声明，`event_id` UUID 为平台关联事件，`content_type` string，取 `application/octet-stream`（二进制）、`application/json`（JSON）或 `text/plain`（纯文本） | `upload_id` UUID，`offset` 非负整数（下一字节），`chunk_size` 正整数（块上限），`expires_at` UTC，`status` | FleetDM/osquery carve begin 有 `carve_id`、`carve_size`、`block_size`，返回 `session_id`；AIDR 使用 `file_id`、`size_bytes`、`chunk_size`、`upload_id`。 | ⬜ 固定版文件流未见该会话 RPC 或对应字段。 [Proto][elkeid-proto] |
| GetUpload | upload_id | offset、size_bytes、status、expires_at、file_id | AIDR 查询确认偏移。FleetDM/osquery 按 `block_id` 检查顺序。 | ⬜ 固定版文件流未见该会话 RPC 或对应字段。 [Proto][elkeid-proto] |
| PutChunk | upload_id、offset、`chunk_sha256` string、`data` Protobuf bytes，长度 1–chunk_size | offset（已持久化的下一字节） | FleetDM/osquery `block_id` + `data`；Elkeid 文件流传字节。AIDR 按字节 `offset` 确认并增加 `chunk_sha256`。 | 🟩 有带 `token` 的文件字节流。未见本行偏移、块摘要及确认偏移。 [Proto][elkeid-proto] |
| Complete | upload_id | file_id 和 status=complete，校验通过并已提交 | AIDR 显式提交；Elkeid 文件流结束后返回 `SUCCESS/FAILED`。 | 🟩 有流结束后的 `SUCCESS/FAILED`，未见单独 Complete 调用。 [Proto][elkeid-proto] |

status 为 `uploading`（会话可继续写入）或 `complete`（文件已校验并提交）。会话与设备绑定。相同 file_id 和文件元数据复用会话，同 ID 不同内容返回 ALREADY_EXISTS。

插件 `AIDREvent.evidence` 与 `data` 中的 `args_digest` / `result_digest` / `report_digest` 是脱敏后的字符串或摘要，随遥测一起上报，不等同于本节上传的附件；需要提交原文件时另走 Evidence RPC，`CreateUpload.event_id` 关联第 4 节事件的 `event_id`。

例：发送内容 `abc`，size_bytes=3，sha256 为 `ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad`。CreateUpload 得到 offset=0，发送 offset=0 的 3 字节块（块摘要与整文件摘要相同），收到 offset=3，再 Complete 得到 complete。

| 故障 | 处理 |
|---|---|
| 块偏移不符 | FAILED_PRECONDITION，GetUpload 查询服务端 offset 后续传 |
| 分块应答丢失 | 先 GetUpload，按服务端确认的 offset 恢复 |
| 会话过期 | FAILED_PRECONDITION，确认会话过期后用原 file_id 重建 |
| 块或全文件校验失败 | DATA_LOSS，不能标记完成。Agent 重新校验暂存文件，内容正确才重新上传 |

**来源**：[FleetDM/osquery 分块上传](fleetdm/fleetdm.md#文件采集上传) · [Elkeid 文件流](elkeid/elkeid.md)。AIDR 的 offset、SHA-256 和提交语义为本稿定义，Fleet 的 block_id 是分块机制参考。

## 联调验收

| 场景 | 核对点 |
|---|---|
| 遥测上报与对账 | Agent 发送 EventBatch，平台 accepted 后能按同一 event_id 查到事件；同一 event_id 重传返回 duplicate |
| 插件字段映射 | 插件 `AIDREvent` 上报后，`source_event_id` 等于插件 `event_id`，`data` 字段与插件 PB 同名；`policy.ack` 能按 `revision` 对上端管 `PolicyAck` |
| 断线重传 | 回执丢失或流中断后，原批以相同 batch_id 和 event_id 重传，已确认事件不重复计数 |
| 认证及上线 | 可信客户端证书完成 TLS，首包使 Agent 加入在线连接池。过期证书握手失败。重复在线 ID 产生冲突 |
| 心跳与管控 | 心跳 seq 和 ack_seq 对齐，策略 received 与 applied 可独立观测 |
| 文件 | 三字节上传中断后按确认偏移恢复。制品摘要错误时禁止安装并上报失败 |

第三方联调须以冻结后的本稿联调版本为准。上线前需确认部署证书材料、业务报文映射和制品更新组件选型，并用管理端与装载第三方制品的 Agent 做端到端验证。
