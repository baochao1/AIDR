# Elkeid 官方采集字段附录

本表将提交 `c906c512d20c0d56f2b779b8558ed69abae718c8` 第一张采集矩阵转为每个记录类型一行。原表有 927 个非空字段单元格。类型 611 的 `pid_tree` 重复出现，本表合并为一项，共 926 个不同的类型与字段配对。统计沿用上游矩阵的展示单元格，其中 37 行含 `data_type`（类型值本身）、16 行不含，按矩阵原样保留。字段集合沿用上游的展示命名，可能包含外层元数据和业务处理生成字段。实际 Agent→AgentCenter Protobuf 包含 `data_type` 与 `timestamp`、外层批次元数据，以及 `Payload.fields` 开放字符串映射。发布矩阵未声明 map 键的必填性、默认值或逐字段线上约束。Agent 心跳经源码核实的字段见主报告。 [固定提交原始数据矩阵](https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/docs/ElkeidData/raw_data_desc.md)

| 类型值 | 来源模块 | 事件 | 公开字段 |
|---|---|---|---|
| 1 | Driver Plugin | Write | `data_type`, `uid`, `exe`, `pid`, `ppid`, `pgid`, `tgid`, `sid`, `comm`, `nodename`, `sessionid`, `pns`, `root_pns`, `file`, `buf`, `argv`, `ppid_argv`, `pgid_argv`, `username`, `pod_name`, `exe_hash`, `pid_tree` |
| 2 | Driver Plugin | Open | `data_type`, `uid`, `exe`, `pid`, `ppid`, `pgid`, `tgid`, `sid`, `comm`, `nodename`, `sessionid`, `pns`, `root_pns`, `flags`, `mode`, `file`, `argv`, `ppid_argv`, `pgid_argv`, `username`, `pod_name`, `exe_hash`, `pid_tree` |
| 10 | Driver Plugin | Mprotect | `data_type`, `uid`, `exe`, `pid`, `ppid`, `pgid`, `tgid`, `sid`, `comm`, `nodename`, `sessionid`, `pns`, `root_pns`, `mprotect_prot`, `owner_pid`, `owner_file`, `vm_file`, `pid_tree`, `argv`, `ppid_argv`, `pgid_argv`, `username`, `pod_name`, `exe_hash` |
| 35 | Driver Plugin | Nanosleep | `data_type`, `uid`, `exe`, `pid`, `ppid`, `pgid`, `tgid`, `sid`, `comm`, `nodename`, `sessionid`, `pns`, `root_pns`, `sec`, `nsec`, `argv`, `ppid_argv`, `pgid_argv`, `username`, `pod_name`, `exe_hash`, `pid_tree` |
| 42 | Driver Plugin | Connect | `data_type`, `uid`, `exe`, `pid`, `ppid`, `pgid`, `tgid`, `sid`, `comm`, `nodename`, `sessionid`, `pns`, `root_pns`, `sa_family`, `dip`, `dport`, `sip`, `sport`, `res`, `argv`, `ppid_argv`, `pgid_argv`, `username`, `pod_name`, `exe_hash`, `pid_tree` |
| 43 | Driver Plugin | Accept | `data_type`, `uid`, `exe`, `pid`, `ppid`, `pgid`, `tgid`, `sid`, `comm`, `nodename`, `sessionid`, `pns`, `root_pns`, `sa_family`, `dip`, `dport`, `sip`, `sport`, `res`, `argv`, `ppid_argv`, `pgid_argv`, `username`, `pod_name`, `exe_hash`, `pid_tree` |
| 49 | Driver Plugin | Bind | `data_type`, `uid`, `exe`, `pid`, `ppid`, `pgid`, `tgid`, `sid`, `comm`, `nodename`, `sessionid`, `pns`, `root_pns`, `sa_family`, `sip`, `sport`, `res`, `argv`, `ppid_argv`, `pgid_argv`, `username`, `pod_name`, `exe_hash`, `pid_tree` |
| 59 | Driver Plugin | Execve | `data_type`, `uid`, `exe`, `pid`, `ppid`, `pgid`, `tgid`, `sid`, `comm`, `nodename`, `sessionid`, `pns`, `root_pns`, `argv`, `run_path`, `stdin`, `stdout`, `dip`, `dport`, `sip`, `sport`, `sa_family`, `pid_tree`, `tty`, `socket_pid`, `ssh`, `ld_preload`, `res`, `socket_argv`, `ppid_argv`, `pgid_argv`, `username`, `pod_name`, `exe_hash` |
| 60 | Driver Plugin | Exit | `data_type`, `uid`, `exe`, `pid`, `ppid`, `pgid`, `tgid`, `sid`, `comm`, `nodename`, `sessionid`, `pns`, `root_pns`, `argv`, `ppid_argv`, `pgid_argv`, `username`, `pod_name`, `exe_hash`, `pid_tree` |
| 62 | Driver Plugin | Kill | `data_type`, `uid`, `exe`, `pid`, `ppid`, `pgid`, `tgid`, `sid`, `comm`, `nodename`, `sessionid`, `pns`, `root_pns`, `target_pid`, `sig`, `argv`, `ppid_argv`, `pgid_argv`, `username`, `pod_name`, `exe_hash`, `pid_tree`, `target_argv` |
| 82 | Driver Plugin | Rename | `data_type`, `uid`, `exe`, `pid`, `ppid`, `pgid`, `tgid`, `sid`, `comm`, `nodename`, `sessionid`, `pns`, `root_pns`, `old_name`, `new_name`, `sb_id`, `argv`, `ppid_argv`, `pgid_argv`, `username`, `pod_name`, `exe_hash`, `pid_tree` |
| 86 | Driver Plugin | Link | `data_type`, `uid`, `exe`, `pid`, `ppid`, `pgid`, `tgid`, `sid`, `comm`, `nodename`, `sessionid`, `pns`, `root_pns`, `old_name`, `new_name`, `sb_id`, `argv`, `ppid_argv`, `pgid_argv`, `username`, `pod_name`, `exe_hash`, `pid_tree` |
| 101 | Driver Plugin | Ptrace | `data_type`, `uid`, `exe`, `pid`, `ppid`, `pgid`, `tgid`, `sid`, `comm`, `nodename`, `sessionid`, `pns`, `root_pns`, `ptrace_request`, `target_pid`, `addr`, `data`, `pid_tree`, `argv`, `ppid_argv`, `pgid_argv`, `username`, `pod_name`, `exe_hash`, `target_argv` |
| 112 | Driver Plugin | SetSid | `data_type`, `uid`, `exe`, `pid`, `ppid`, `pgid`, `tgid`, `sid`, `comm`, `nodename`, `sessionid`, `pns`, `root_pns`, `argv`, `ppid_argv`, `pgid_argv`, `username`, `pod_name`, `exe_hash`, `pid_tree` |
| 157 | Driver Plugin | Prctl | `data_type`, `uid`, `exe`, `pid`, `ppid`, `pgid`, `tgid`, `sid`, `comm`, `nodename`, `sessionid`, `pns`, `root_pns`, `option`, `new_name`, `argv`, `ppid_argv`, `pgid_argv`, `username`, `pod_name`, `exe_hash`, `pid_tree` |
| 165 | Driver Plugin | Mount | `data_type`, `uid`, `exe`, `pid`, `ppid`, `pgid`, `tgid`, `sid`, `comm`, `nodename`, `sessionid`, `pns`, `root_pns`, `pid_tree`, `dev`, `file_path`, `fstype`, `flag`, `argv`, `ppid_argv`, `pgid_argv`, `username`, `pod_name`, `exe_hash` |
| 200 | Driver Plugin | tkill | `data_type`, `uid`, `exe`, `pid`, `ppid`, `pgid`, `tgid`, `sid`, `comm`, `nodename`, `sessionid`, `pns`, `root_pns`, `target_pid`, `sig`, `argv`, `ppid_argv`, `pgid_argv`, `username`, `pod_name`, `exe_hash`, `pid_tree` |
| 231 | Driver Plugin | exit_group | `data_type`, `uid`, `exe`, `pid`, `ppid`, `pgid`, `tgid`, `sid`, `comm`, `nodename`, `sessionid`, `pns`, `root_pns`, `argv`, `ppid_argv`, `pgid_argv`, `username`, `pod_name`, `exe_hash`, `pid_tree` |
| 356 | Driver Plugin | MemfdCreate | `data_type`, `uid`, `exe`, `pid`, `ppid`, `pgid`, `tgid`, `sid`, `comm`, `nodename`, `sessionid`, `pns`, `root_pns`, `fd_name`, `flags`, `argv`, `ppid_argv`, `pgid_argv`, `username`, `pod_name`, `exe_hash`, `pid_tree` |
| 601 | Driver Plugin | DNS Query | `data_type`, `uid`, `exe`, `pid`, `ppid`, `pgid`, `tgid`, `sid`, `comm`, `nodename`, `sessionid`, `pns`, `root_pns`, `query`, `sa_family`, `dip`, `dport`, `sip`, `sport`, `opcode`, `rcode`, `argv`, `ppid_argv`, `pgid_argv`, `username`, `pod_name`, `exe_hash`, `pid_tree` |
| 602 | Driver Plugin | CreateFile | `data_type`, `uid`, `exe`, `pid`, `ppid`, `pgid`, `tgid`, `sid`, `comm`, `nodename`, `sessionid`, `pns`, `root_pns`, `file_path`, `dip`, `dport`, `sip`, `sport`, `sa_family`, `socket_pid`, `sb_id`, `argv`, `ppid_argv`, `pgid_argv`, `username`, `pod_name`, `exe_hash`, `pid_tree`, `socket_argv` |
| 603 | Driver Plugin | LoadModule | `data_type`, `uid`, `exe`, `pid`, `ppid`, `pgid`, `tgid`, `sid`, `comm`, `nodename`, `sessionid`, `pns`, `root_pns`, `ko_file`, `pid_tree`, `run_path`, `argv`, `ppid_argv`, `pgid_argv`, `username`, `pod_name`, `exe_hash` |
| 604 | Driver Plugin | CommitCred | `data_type`, `uid`, `exe`, `pid`, `ppid`, `pgid`, `tgid`, `sid`, `comm`, `nodename`, `sessionid`, `pns`, `root_pns`, `pid_tree`, `old_uid`, `res`, `argv`, `ppid_argv`, `pgid_argv`, `username`, `pod_name`, `exe_hash`, `old_username` |
| 605 | Driver Plugin | unlink | `data_type`, `uid`, `exe`, `pid`, `ppid`, `pgid`, `tgid`, `sid`, `comm`, `nodename`, `sessionid`, `pns`, `root_pns`, `file`, `argv`, `ppid_argv`, `pgid_argv`, `username`, `pod_name`, `exe_hash`, `pid_tree` |
| 606 | Driver Plugin | rmdir | `data_type`, `uid`, `exe`, `pid`, `ppid`, `pgid`, `tgid`, `sid`, `comm`, `nodename`, `sessionid`, `pns`, `root_pns`, `file`, `argv`, `ppid_argv`, `pgid_argv`, `username`, `pod_name`, `exe_hash`, `pid_tree` |
| 607 | Driver Plugin | call_usermode | `data_type`, `exe`, `argv`, `wait` |
| 608 | Driver Plugin | ReadFile | `data_type`, `uid`, `exe`, `pid`, `ppid`, `pgid`, `tgid`, `sid`, `comm`, `nodename`, `sessionid`, `pns`, `root_pns`, `file`, `sb_id`, `argv`, `ppid_argv`, `pgid_argv`, `username`, `pod_name`, `exe_hash`, `pid_tree` |
| 609 | Driver Plugin | WriteFile | `data_type`, `uid`, `exe`, `pid`, `ppid`, `pgid`, `tgid`, `sid`, `comm`, `nodename`, `sessionid`, `pns`, `root_pns`, `file`, `sb_id`, `argv`, `ppid_argv`, `pgid_argv`, `username`, `pod_name`, `exe_hash`, `pid_tree` |
| 610 | Driver Plugin | USB Event | `data_type`, `uid`, `exe`, `pid`, `ppid`, `pgid`, `tgid`, `sid`, `comm`, `nodename`, `sessionid`, `pns`, `root_pns`, `product_info`, `manufacturer`, `serial`, `action`, `argv`, `ppid_argv`, `pgid_argv`, `username`, `pod_name`, `exe_hash`, `pid_tree` |
| 611 | Driver Plugin | PrivilegeEscalation | `data_type`, `uid`, `exe`, `pid`, `ppid`, `pgid`, `tgid`, `sid`, `comm`, `nodename`, `sessionid`, `pns`, `root_pns`, `dpid`, `pid_tree`, `dcred`, `cred`, `argv`, `ppid_argv`, `pgid_argv`, `username`, `pod_name`, `exe_hash`, `dpid_argv` |
| 700 | Driver Plugin | ProcFileHook | `data_type`, `module_name` |
| 701 | Driver Plugin | SyscallHook | `data_type`, `module_name`, `syscall_number` |
| 702 | Driver Plugin | LkmHidden | `data_type`, `module_name` |
| 703 | Driver Plugin | InterruptsHook | `data_type`, `module_name`, `interrupt_number` |
| 1000 | Elkeid Agent | Agent Heartbeat | `kernel_version`, `arch`, `platform`, `platform_family`, `platform_version`, `idc`, `region`, `net_mode`, `rx_speed`, `tx_speed`, `cpu`, `rss`, `read_speed`, `write_speed`, `pid`, `fd_cnt`, `started_at`, `tx_tps`, `rx_tps`, `du`, `grs`, `nproc`, `load_1`, `load_5`, `load_15`, `running_procs`, `total_procs`, `boot_at`, `sys_cpu`, `sys_mem` |
| 1001 | Elkeid Agent | Plugin Heartbeat | `name`, `pversion`, `rx_speed`, `tx_speed`, `rx_tps`, `tx_tps`, `cpu`, `rss`, `read_speed`, `write_speed`, `pid`, `started_at`, `du`, `fd_cnt` |
| 1010 | Elkeid Agent | Agent Error Log | `level`, `msg`, `source`, `plugin`, `psign`, `pver` |
| 2010 | Elkeid RASP | Command | `data_type`, `probe_version`, `name`, `commands`, `pid`, `state`, `runtime`, `probe_message` |
| 2011 | Elkeid RASP | Report | `data_type`, `pid`, `action`, `reason`, `cmdline`, `tracing_state`, `runtime`, `attach_start_time`, `attach_end_time`, `failed_time`, `missing_time`, `try_attach_count`, `attached_count` |
| 2459 | Elkeid RASP | Hook | `data_type`, `probe_version`, `runtime`, `runtime_version`, `message_type`, `class_id`, `method_id`, `pid`, `args`, `stack_trace`, `rasp_timestamp` |
| 4000 | Journal Watcher | SSH Login | `status`, `types`, `invalid`, `user`, `sip`, `sport`, `rawlog`, `extra`, `pid` |
| 4001 | Journal Watcher | Kerberos Auth | `authorized`, `principal`, `rawlog`, `pid` |
| 5050 | Collector Plugin | Process | `cmdline`, `cwd`, `checksum`, `exe_hash`, `exe`, `pid`, `comm`, `state`, `ppid`, `pgid`, `sid`, `cpu`, `start_at`, `nthreads`, `vsize`, `rss`, `umask`, `tcpid`, `ruid`, `euid`, `suid`, `fsuid`, `rusername`, `eusername`, `susername`, `fsusername`, `nspid`, `nspgid`, `nssid`, `dns`, `cns`, `ins`, `mns`, `nns`, `pns`, `tns`, `uns`, `utns` |
| 5051 | Collector Plugin | Port | `family`, `protocol`, `state`, `sport`, `dport`, `sip`, `dip`, `uid`, `inode`, `username`, `pid`, `exe`, `comm`, `cmdline` |
| 5052 | Collector Plugin | User | `username`, `password`, `uid`, `gid`, `groupname`, `info`, `home`, `shell`, `last_login_time`, `last_login_ip`, `weak_password` |
| 5053 | Collector Plugin | Cron | `path`, `username`, `schedule`, `command`, `checksum` |
| 5054 | Collector Plugin | Service | `name`, `type`, `command`, `restart`, `working_dir`, `checksum`, `bus_name` |
| 5055 | Collector Plugin | Software | `name`, `sversion`, `type`, `source`, `status`, `vendor`, `component_version`, `pid`, `pod_name`, `psm` |
| 5057 | Collector Plugin | Integrity | `software_name`, `digest`, `origin_digest`, `digest_algotithm`, `exe`, `modify_time`, `software_version` |
| 5058 | Collector Plugin | Sudoers | `run_as_users`, `run_as_groups`, `options`, `command`, `username` |
| 6001 | ScannerClamav Plugin | Dir Match | `types`, `class`, `name`, `exe`, `exe_size`, `exe_hash`, `md5_hash`, `create_at`, `modify_at` |
| 6002 | ScannerClamav Plugin | Proc Match | `types`, `class`, `name`, `exe`, `exe_size`, `exe_hash`, `md5_hash`, `create_at`, `modify_at`, `pid`, `ppid`, `pgid`, `tgid`, `argv`, `comm`, `sessionid`, `uid`, `pns` |
| 6003 | ScannerClamav Plugin | Scan Task | `types`, `class`, `name`, `exe`, `exe_size`, `exe_hash`, `md5_hash`, `create_at`, `modify_at`, `error`, `token`, `{custom}` |

## 固定生产者交叉核对

以下字段位于 `Payload.fields`，键和值都是字符串。外层类型、时间和主机元数据按主报告的通用字段解释。

| 类型 | 生产者和键 | 已验证约束 |
|---|---|---|
| 4000 | Journal Watcher SSH 登录，`status`、`types`、`invalid`、`user`、`sip`、`sport`、`extra`、`pid`、`rawlog` | `status` 为 `true`（Accepted）或 `false`（Failed）。`invalid` 为 `true`（invalid user）或 `false`。认证方法 `types` 由解析器按开放文本取得。 |
| 4001 | Journal Watcher Kerberos 授权，`authorized`、`principal`、`pid`、`rawlog` | 前两项分别是被授权用户名与 Kerberos principal。 `authorized` 为用户名，协议表示为字符串。 |
| 1010、1011 | Agent 与 Go 插件远程日志 | 接收日志编码器生成的开放对象；可解析的 `timestamp` 移至外层，其余 string 成员进入 map。实现另有 Go `int` 分支，但默认 JSON 解码把普通数字解为 float64，数值成员不会进入该分支。 |
| 5100 | Collector 任务结果、Agent 带任务 token 的错误日志 | `status` 为 `succeed` 或 `failed`，`token` 为对账标识，`msg` 为说明。Agent 错误日志仅用 `failed`。 |
| 6000 | Scanner 扫描完成记录 | `status` 为 `succeed` 或 `failed`，`msg` 为结果说明，带对账信息的发送分支增加 `token`。全盘扫描超时在线上归为 `failed`。 |
| 8000 | Baseline 检查结果 | `data` 存嵌套 JSON 字符串，`token` 存任务标识。见下表。 |
| 8010 | Baseline 任务状态 | `status` 为 `succeed` 或 `failed`，`msg` 为说明，`token` 仅在非空时写入。 |
| 2001、2003、5101 | AgentCenter 任务对账分支 | 固定接收代码可确认解析为开放字符串 map，并额外写入十进制字符串 `data_type`，未定位到固定版本中的对应生产者。 |

[Journal Watcher 生产者](https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/plugins/journal_watcher/src/main.rs#L150) · [SSH 语法](https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/plugins/journal_watcher/src/sshd.pest) · [Agent 日志](https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/agent/log/writer.go) · [Go 插件日志](https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/plugins/lib/go/log/writer.go) · [Collector 任务结果](https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/plugins/collector/engine/engine.go#L144) · [Agent 任务日志](https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/agent/log/custom.go) · [Scanner](https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/plugins/scanner/src/data_type.rs#L18) · [全盘状态映射](https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/plugins/scanner/src/model/functional/fulldiskscan.rs#L44) · [Baseline 生产者](https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/plugins/baseline/main.go)

### Baseline 8000 的嵌套 JSON

| 路径 | 类型 | 含义或枚举 |
|---|---|---|
| baseline_id | int | 基线编号 |
| baseline_version | string | 基线版本 |
| status | string | `success` 或 `error` |
| msg | string | 结果说明 |
| check_list | object[] 或 null | 逐条检查信息，nil 时编码 null |
| check_list[].check_id | int | 检查编号 |
| check_list[].security | string | 安全分类，业务配置决定 |
| check_list[].type、.title、.description、.solution | string | 类型、标题、描述、修复说明 |
| check_list[].type_cn、.title_cn、.description_cn、.solution_cn | string | 对应中文文本 |
| check_list[].result | int | `1` 通过、`2` 未通过、`-1` 执行错误、`-2` 规则配置错误、`-3` 文件读写异常 |
| check_list[].msg | string | 本条检查说明 |

[Baseline 结果定义](https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/plugins/baseline/src/check/analysis.go) · [结果代码](https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/plugins/baseline/src/check/rule_engine.go#L47)

Collector 公布的 8 个资产采集编号与固定源码中的生产者对应如下。顶层编号已复核，动态字段的可选性与枚举取决于相应采集器和平台。

| 类型 | 生产者 |
|---|---|
| 5050 | [process.go](https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/plugins/collector/process.go) |
| 5051 | [port.go](https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/plugins/collector/port.go) |
| 5052 | [user.go](https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/plugins/collector/user.go) |
| 5053 | [cron.go](https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/plugins/collector/cron.go) |
| 5054 | [service.go](https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/plugins/collector/service.go) |
| 5055 | [software.go](https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/plugins/collector/software.go) |
| 5057 | [integrity.go](https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/plugins/collector/integrity.go) |
| 5058 | [volume.go](https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/plugins/collector/volume.go)，公开矩阵的展示标签为 Sudoers，固定生产者请以源码为准。 |

固定导出中 `5058` 的实际字段为 `name`（设备）、`fstype`（文件系统类型）、`mount_point`（挂载点）、`total`、`used`、`free`（十进制字节数）、`usage`（0–1 的使用比例文本）、`package_seq`（采集批次序号文本）。仅在获取分区用量成功后产生记录。Sudoers 的展示字段保留在官方矩阵行，作为版本差异。 [固定 Volume 生产者](https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/plugins/collector/volume.go)

Scanner 的 `6001`、`6002` 和 `6003` 同时发送 `types`、`class`、`name`、`exe`、`static_file`、`exe_size`、`exe_hash`、`md5_hash`、`create_at`、`modify_at`。有匹配明细时加入 `hit_data`。 `6001` 和 `6002` 的任务关联分支加 `token`。 `6002` 另带 `pid`、`ppid`、`pgid`、`tgid`、`argv`、`comm`、`sessionid`、`uid`、`pns`，任务关联分支还写 `file_path`、`file_hash`。 `6003` 另带 `error` 和 `token`，一个分支追加自定义键。字段在 Payload 中均为字符串。 [Scanner 记录序列化](https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/plugins/scanner/src/data_type.rs)

展示字段的中文义项、逻辑类型及事件分类见同一固定版本的[字段名称与类型](https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/docs/ElkeidData/raw_data_desc.md#字段对应中文描述)和[数据类型分类](https://github.com/bytedance/Elkeid/blob/c906c512d20c0d56f2b779b8558ed69abae718c8/docs/ElkeidData/raw_data_desc.md#数据类型分类)。逻辑类型可能来自数据处理后的视图，上报的插件通用映射仍以 Protobuf 的 string 值为准。
