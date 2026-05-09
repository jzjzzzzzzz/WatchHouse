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
- 独立 Ubuntu ARM64 QEMU guest 中的真实 Linux、systemd 255 和 OpenSSH 集成；不是 privileged systemd container。
- 独立 `watchhouse` UID、journal-only supplemental group、零 effective capabilities、`NoNewPrivileges`、seccomp、`ProtectSystem=strict` 和 `ProtectHome=yes` 的运行时核验。
- bounded oneshot collector 与 non-overlapping timer 已实际安装；service 验收的 native journal 产生 112 条 pending event，独立 audit 数量与逻辑 bytes 一致。
- clean-worktree reboot 验收生成恰好五次 rejected-key 事件并把它们关联到同次新 finding；guest boot ID 改变前后 131 条 pending records 不丢失，随后从 native cursor 继续插入 15 条 matched records。
- 当前 network namespace 的 TCP listener 解析与 inode→PID/start-time/UID/systemd unit 关联；普通账号对 root fixture 明确返回 `unknown_permission`，lab-admin fixed reader 才能完成归属。
- mTLS TLS 1.3 event batch、certificate URI host identity、strict bounded JSON、exact receipts、SQLite Ack 和 PostgreSQL `(host,event)` 幂等存储。
- 实际 agent/control binaries 经 loopback mTLS 交付七条事件；错误证书/载荷 host 组合被拒且七条本地记录保留。另有 response-after-commit 损坏故障注入与无重复恢复。

测试证据：[SSH 只读链路](../evidence/test-runs/2026-10-05-ssh/README.md)。

持久化证据：[SQLite spool 与突杀恢复](../evidence/test-runs/2026-10-05-spool/README.md)。

真实 systemd 证据：[Ubuntu guest collector acceptance](../evidence/test-runs/2026-10-05-systemd/README.md)。

## 阶段状态

| 阶段 | 状态 |
| --- | --- |
| M0 初始契约 | 事件、规则、响应状态和场景目录已写；新增真实接口仍需对应契约 |
| M1 只读观测 | 部分完成：journal 快照与 forward capture、cursor/queue 持久化、真实 VM service/timer 验证；缺物理磁盘限制、unit/socket/container 采集和网络上报 |
| M2 控制面 | 部分完成：mTLS ingest、PostgreSQL event store、进程级 delivery 已实现；缺查询 API、规则持久状态、外部探针、RBAC 与正式部署 |
| M3 写动作 | 未开始 |
| M4 部署和数据库恢复 | 未开始 |
| M5 第一版发布 | 未完成 |
| M6 连续自用 | 未开始 |

## 下一切片

真实 Linux/systemd 集成环境已经运行：OpenSSH 实际日志、emitting UID/comm、native cursor、持久队列和 service sandbox 均已在独立 QEMU guest 验证。原始 journal、官方 cloud image、生成密钥和 VM 状态留在 Git 外；公开证据只保留摘要、digest 和边界断言。

下一切片把 systemd delivery/control units 做真实运行验收，增加服务端查询/RBAC 与持久规则处理；随后把 listener snapshot 作为独立 schema 上报，并采集容器 published-port 与外部可达性证据。VM 证据不冒充公网 VPS、长期运行或 production accuracy。
