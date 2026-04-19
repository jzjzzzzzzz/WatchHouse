# 开发进度

实际日期：2026-10-05。

## 当前可运行部分

- 可信字段与 root emitting UID 过滤的 SSH journal 解析。
- 64 KiB 输入限制、重复顶层字段拒绝、UTF-8 验证和稳定事件标识。
- 认证失败后成功的有界关联规则，按 host、boot、用户和来源 IP 分组。
- 证据引用、状态容量限制、去重、乱序和身份冲突错误。
- replay CLI，JSON Lines 输出、处理 summary 和部分结果标记。
- Linux 有界 journal snapshot，固定 argv、不用 shell、不申请提权、限制输出并设置超时。
- 单元、race、fuzz、静态检查、Linux 双架构编译和隔离 Linux 容器 smoke。
- 私有 state 路径、SQLite WAL schema/application ID 和启动计数检查。
- event/cursor 原子落盘、checkpoint CAS、精确 receipt Ack 和完整文件 resume。
- 逻辑 byte/record backpressure，旧 pending 事件不回退 cursor。
- native forward Poll 核验起始 cursor、限制前 N 条，不选 tail；captured batch 与原始 anchor 绑定。
- spool 只读完整性审计、SQL 故障注入、并发 CAS 和实际 SIGKILL 重启恢复。

测试证据：[SSH 只读链路](../evidence/test-runs/2026-10-05-ssh/README.md)。

持久化证据：[SQLite spool 与突杀恢复](../evidence/test-runs/2026-10-05-spool/README.md)。

## 阶段状态

| 阶段 | 状态 |
| --- | --- |
| M0 初始契约 | 事件、规则、响应状态和场景目录已写；新增真实接口仍需对应契约 |
| M1 只读观测 | 部分完成：journal 快照与 forward capture、cursor/queue 持久化；缺常驻 agent、物理磁盘限制、unit/socket/container 采集及真实 VM 验证 |
| M2 控制面 | 未开始；离线规则实现不代表控制端完成 |
| M3 写动作 | 未开始 |
| M4 部署和数据库恢复 | 未开始 |
| M5 第一版发布 | 未完成 |
| M6 连续自用 | 未开始 |

## 下一切片

准备真实 Linux systemd 集成环境，验证 OpenSSH 实际日志的 emitting UID、comm、JSON 结构和 native cursor resume。Mac 上没有现成 limactl/multipass/QEMU；计划使用独立 Docker tools 容器中的 QEMU TCG 启动真正的 Ubuntu VM，不启动 privileged container、不接触现有 Docker workloads。

VM 只提供 Linux/systemd 集成证据，不冒充公网 VPS 部署。官方 cloud image、校验摘要、生成密钥和 VM 运行状态放在 Git 外。VM 验证后再做 unit/socket 观测、网络上报与控制面。
