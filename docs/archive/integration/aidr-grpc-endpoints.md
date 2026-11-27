# AIDR gRPC 端点与 Python、Go、Java 实现示例

## 1. 双方的端点

本机只有厂商 EXE 监听一个 gRPC 地址。Agent 是客户端，双向流共用同一条连接。Go 适配器可选 [原生 IPC](../architecture/native-ipc.md)，Windows 使用命名管道，Linux/macOS 使用 Unix socket。下文 Python 和 Java 片段演示已经保留的 TLS 回环 TCP 兼容路径。

| 连接 | 发起方 | 监听方 | 示例 |
|---|---|---|---|
| 组件控制和事件 | Agent 主动连接 | 厂商 EXE，监听 DEMO_ENDPOINT | 原生模式：Windows Named Pipe / Linux、macOS Unix socket。兼容模式：127.0.0.1:53180 等动态回环地址。TLS gRPC |
| 管理与上传 | Agent，临时源端口 | 管理服务器，HTTPS 监听地址 | Agent → https://127.0.0.1:8443，Demo mTLS |
| 厂商到管理端 | 无直接连接 | 无 | 由 Agent 队列代为上传 |

53180 是兼容 TCP 模式的示例端口。原生模式不占用插件 TCP 监听端口。实际地址在每次组件启动时由 Agent 通过 DEMO_ENDPOINT 传入。组件侧证书和私钥通过 DEMO_IPC_CERT、DEMO_IPC_KEY 传入 PEM 内容。Agent 信任当前实例的证书，并发送 metadata instance-token，值对应 DEMO_TOKEN。组件对下面的两个调用均需验证 token。

| 本机方法 | 请求与响应 | 说明 |
|---|---|---|
| /grpc.health.v1.Health/Check | HealthCheckRequest → HealthCheckResponse | Agent 请求空 service；厂商准备完成后回 SERVING |
| /demo.component.v1.Events/Stream | 双向流，两个方向都是 google.protobuf.StringValue | Agent 首先发送 ready；厂商回一条事件；Agent 落盘后回 event_id；厂商再发下一条 |

管理端的 POST /v1/agents/events 是单独的 HTTPS 接口，厂商适配器无需实现它。组件接收 event_id 表示终端落盘，平台送达另由管理端查询确认。

## 2. Python 实现

依赖 grpcio、grpcio-health-checking 和 grpcio-tools。用仓库的 events.proto 生成 events_pb2_grpc：

    python -m grpc_tools.protoc -I api/proto/component/v1 --python_out=. --grpc_python_out=. api/proto/component/v1/events.proto

厂商服务端的核心代码如下。生产适配器把固定的测试事件 换成 SDK 的事件，并在 SDK 初始化成功后提供 SERVING。将 Health 原型与 Events 服务一起注册在同一监听端口。

    import os
    from concurrent.futures import ThreadPoolExecutor
    import grpc
    from google.protobuf.wrappers_pb2 import StringValue
    from grpc_health.v1 import health_pb2, health_pb2_grpc
    import events_pb2_grpc

    def check_token(context):
        values = [v for k, v in context.invocation_metadata() if k == "instance-token"]
        if values != [os.environ["DEMO_TOKEN"]]:
            context.abort(grpc.StatusCode.UNAUTHENTICATED, "invalid instance token")

    class Health(health_pb2_grpc.HealthServicer):
        def Check(self, request, context):
            check_token(context)
            return health_pb2.HealthCheckResponse(status=health_pb2.HealthCheckResponse.SERVING)

    class Events(events_pb2_grpc.EventsServicer):
        def Stream(self, requests, context):
            check_token(context)
            ready = next(requests, None)
            if ready is None or ready.value != "ready":
                context.abort(grpc.StatusCode.INVALID_ARGUMENT, "ready required")
            yield StringValue(value='{"type":"aidr.integration_test"}')
            ack = next(requests, None)
            if ack is not None and ack.value:
                print("Agent persisted:", ack.value, flush=True)

    server = grpc.server(ThreadPoolExecutor(max_workers=4))
    health_pb2_grpc.add_HealthServicer_to_server(Health(), server)
    events_pb2_grpc.add_EventsServicer_to_server(Events(), server)
    credentials = grpc.ssl_server_credentials(((
        os.environ["DEMO_IPC_KEY"].encode(), os.environ["DEMO_IPC_CERT"].encode(),
    ),))
    server.add_secure_port(os.environ["DEMO_ENDPOINT"], credentials)
    server.start()
    server.wait_for_termination()

若用 Python 写联调客户端，打开一个安全 channel，同时复用给两个 Stub。以下片段中的 persist_event 是客户端自己的落盘函数，成功后生成并返回 ID：

    from queue import Queue
    outgoing = Queue()
    outgoing.put(StringValue(value="ready"))
    metadata = (("instance-token", instance_token),)
    credentials = grpc.ssl_channel_credentials(root_certificates=instance_cert_pem)
    with grpc.secure_channel(endpoint, credentials) as channel:
        check = health_pb2_grpc.HealthStub(channel).Check(
            health_pb2.HealthCheckRequest(), metadata=metadata, timeout=2)
        if check.status != health_pb2.HealthCheckResponse.SERVING:
            raise RuntimeError("adapter not ready")
        try:
            replies = events_pb2_grpc.EventsStub(channel).Stream(
                iter(outgoing.get, None), metadata=metadata, timeout=10)
            for event in replies:
                outgoing.put(StringValue(value=persist_event(event.value)))
        finally:
            outgoing.put(None)

## 3. Go 实现

项目现有 Go 代码可直接作为参考。厂商服务端监听和注册两个服务的代码在 [component.go](../../../internal/component/component.go)，双向流的 token 校验与 ready → event → ACK 在 [events.go](../../../internal/component/events.go)。Go Mock 的启动入口在 [cmd/mock-go/main.go](../../../cmd/mock-go/main.go)。

服务端的关键操作是用 net.Listen("tcp", endpoint) 监听，加载 tls.X509KeyPair，创建 grpc.NewServer，注册 Health 和 Events，再调用 Serve。当前项目的 Events 使用手工 ServiceDesc，采用代码生成时也可以用 Events 的生成注册函数。

Agent 客户端真实代码位于同一 events.go 的 Collect。核心步骤如下，省略错误处理时用于理解流程：

    // DialPinned 使用本次实例证书验证服务端。
    conn, err := component.DialPinned(endpoint, certPEM)
    if err != nil { return err }
    defer conn.Close()

    if err := component.Probe(ctx, healthpb.NewHealthClient(conn), token); err != nil {
        return err
    }
    return component.Collect(ctx, conn, token, persistEvent)

persistEvent 接收一条 UTF-8 payload，在状态库提交事件后返回 event_id。Collect 负责在同一 channel 上打开双向流、发送 ready、接收事件并回传 ID。厂商实现只需对接上述 RPC，无需启动另一个 Agent 服务端。

## 4. Java 实现

依赖 grpc-netty-shaded、grpc-protobuf、grpc-stub 和 grpc-services。用 Maven 或 Gradle 的 Protobuf 插件与 protoc-gen-grpc-java 从 events.proto 生成 EventsGrpc。标准健康接口使用 io.grpc.health.v1 的 HealthGrpc。以下是连接与流的核心片段；类名以本地生成代码为准。

厂商服务端使用 NettyServerBuilder 绑定 DEMO_ENDPOINT，并将实例 PEM 加载为服务端 TLS：

    var address = new InetSocketAddress("127.0.0.1", portFromDemoEndpoint);
    var cert = new ByteArrayInputStream(certPem.getBytes(StandardCharsets.UTF_8));
    var key = new ByteArrayInputStream(keyPem.getBytes(StandardCharsets.UTF_8));
    var tls = GrpcSslContexts.forServer(cert, key).build();
    var health = new HealthStatusManager();
    // 完成 SDK 初始化后才设置默认服务为 SERVING。
    health.setStatus("", HealthCheckResponse.ServingStatus.SERVING);

    var server = NettyServerBuilder.forAddress(address)
        .sslContext(tls)
        .intercept(tokenInterceptor) // 拒绝缺失或错误的 instance-token
        .addService(health.getHealthService())
        .addService(eventsService)
        .build().start();

eventsService 继承 EventsGrpc.EventsImplBase 并重写 stream。返回一个 StreamObserver<StringValue> 用于接收 Agent 消息。第一次 onNext 校验 ready，随后调用 response.onNext(StringValue.of(eventJson)) 发送一条事件。下一次 onNext 读取 ACK 的 getValue，拿到非空 ID 后再发下一条。对非法 ready 调用 response.onError(Status.INVALID_ARGUMENT.asRuntimeException())。流取消或出错时释放对应的订阅资源。tokenInterceptor 在 interceptCall 中从 Metadata 读取 instance-token，认证失败调用 call.close(Status.UNAUTHENTICATED, new Metadata())。

Java 联调客户端使用 NettyChannelBuilder 连接相同地址，并信任 Agent 为本实例配置的证书。两个 Stub 复用这个 channel：

    var trusted = new ByteArrayInputStream(certPem.getBytes(StandardCharsets.UTF_8));
    var channel = NettyChannelBuilder.forAddress("127.0.0.1", portFromDemoEndpoint)
        .sslContext(GrpcSslContexts.forClient().trustManager(trusted).build())
        .build();
    var metadata = new Metadata();
    metadata.put(Metadata.Key.of("instance-token", Metadata.ASCII_STRING_MARSHALLER), token);
    var auth = MetadataUtils.newAttachHeadersInterceptor(metadata);
    var healthResult = HealthGrpc.newBlockingStub(channel).withInterceptors(auth)
        .withDeadlineAfter(2, TimeUnit.SECONDS)
        .check(HealthCheckRequest.getDefaultInstance());

    // healthResult.getStatus() 等于 SERVING 后再打开流。
    var requests = EventsGrpc.newStub(channel).withInterceptors(auth).stream(responseObserver);
    requests.onNext(StringValue.of("ready"));

responseObserver 的 onNext 接收事件，在 persistEvent 成功后通过 requests.onNext(StringValue.of(eventId)) 回 ACK。实际项目应安全地把请求流引用交给观察者，再发送 ready。退出时完成请求流并关闭 channel。

以上 Python 和 Java 片段用于解释接口方向及消息顺序。当前仓库实跑过 Go 的 Health 和 EventStream，以及 Python Mock 的 Health。本文 Python 双向流片段、Java 片段需在厂商选定版本后编译联调。组件生命周期收尾详见[厂商接入说明](aidr-vendor-handoff.md)。


