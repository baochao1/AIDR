# Supervisor 与 AIDR 插件 Demo

演示终端 Agent 动态安装、启动和卸载独立组件，并把组件事件送达管理端。Windows 图形界面以 Agent 和插件为主，配套的本地管理端用于验证下发、状态和数据接收。

| 读者 | 阅读入口 |
|---|---|
| 领导 | [三分钟汇报](docs/aidr-management-brief.md) |
| 第三方插件厂商 | [我方接入要求](docs/aidr-vendor-handoff.md) |
| 研发 | [研发接手指南](docs/engineering-guide.md) |
| 产品经理 | [产品范围与验收](docs/product-scope.md) |
| 演示操作 | [Windows Agent Demo 操作手册](docs/windows-agent-demo.md) |

完整目录和主文档归属见 [交付文档导航](docs/index.md)。

## 构建和回归

需要 Go 1.24.13、Python 3.13 和 PowerShell 7。脚本构建三个 Go 命令及独立 Python 制品，运行 A/B PoC、Go 测试、vet 和远程端到端场景。

```pwsh
./scripts/verify.ps1
```

Python 用于 Windows 演示界面、构建 Mock 和运行测试脚本。独立打包的 Python Mock 在终端运行时无需另装 Python。

## 手动运行

Windows 示例。Linux/macOS 把二进制名中的 `.exe` 去掉。

```pwsh
build/supervisor-demo.exe bootstrap
# 两个独立终端分别运行
build/supervisor-demo.exe server
build/supervisor-demo.exe agent -events
```

管理员端发布组件，并下发期望状态。

```pwsh
build/supervisor-demo.exe sign -id mock-a -version 1.0.0 -artifact mock-a-1 -ipc native -file build/mock-go.exe -out build/mock-a-1.json
build/supervisor-demo.exe publish -file build/mock-a-1.json
'{"revision":1,"components":[{"id":"mock-a","version":"1.0.0","desired_state":"running","artifact_id":"mock-a-1"}]}' | Set-Content -Encoding utf8NoBOM build/policy.json
build/supervisor-demo.exe policy -file build/policy.json
build/supervisor-demo.exe status
```

停止组件时递增 revision，将 `desired_state` 改成 `stopped`。启动时改回 `running`。卸载使用新 revision 和 `absent`，业务数据保留在 `.demo/agent/data`。升级时签名并发布新版本制品，通过新 revision 指定该版本。Mock 的 `3.0.0` 故意返回不健康，便于验证恢复旧版本。

`bootstrap` 创建一次性本地实验 CA，以及 server、admin、agent、signer 四套目录。管理端和终端运行时分别只需要自己的身份目录和 Supervisor Demo 二进制。示例使用回环地址、固定终端身份和短期证书。跨机器使用前需为服务器地址配置匹配的证书及相应网络监听，并按角色分别分发凭据。请保管本地私钥，整个 `.demo/` 目录已忽略。


