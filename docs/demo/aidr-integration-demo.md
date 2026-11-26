# 逐步终端演示

对领导或现场演示，优先使用 [Windows Agent 图形 Demo](windows-agent-demo.md)。本页保留同一闭环的终端版，方便录制运行日志和复现现场结果。

在项目根目录，使用已构建的 Windows amd64 二进制：

    python scripts/aidr-walkthrough.py --pause

每屏确认实际结果后暂停，按 Enter 继续。去掉 --pause 可自动执行。顺序为空白终端注册、安装并启动、管理端已落库、停止、卸载。脚本使用临时管理端、Agent、测试 CA 和本机 HTTPS 地址，Go 样例组件默认用原生 IPC。

如果需要了解手工启动、制品发布及策略命令，见 [README](../../README.md#手动运行)。交给厂商的启动规范、数据样例和验收责任集中在[厂商接入要求](../integration/aidr-vendor-handoff.md)。
