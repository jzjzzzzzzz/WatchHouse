# Watchhouse

面向小规模 Linux 和 VPS 部署的安全运维闭环。

Watchhouse 围绕一个具体问题展开：服务器出现异常或服务故障时，能否说明发生了什么，在明确权限边界内完成修复，并用独立检查证明服务和安全约束已经恢复。

它不是服务器管理面板，也不是新的 SIEM。项目核心是带证据的事件调查、受限修复协议、故障恢复和可复现验证。

## 当前状态

2026-10-06：开发初期。已实现 SSH journal 规范化、有界认证检测、replay/snapshot CLI、核验 cursor 的 native forward collector，以及 SQLite 持久化队列、精确 receipt Ack、容量 backpressure 与只读审计。附单元、race、fuzz、Linux 容器测试、子进程 SIGKILL 恢复测试，以及独立 Ubuntu QEMU guest 中的真实 systemd/OpenSSH 验收。当前有 timer 驱动的 bounded collector、严格 mTLS batch transport、PostgreSQL event/finding store、证书隔离的 human query 和 append-only authorization audit；真实 systemd control/delivery 验收已通过，loopback 进程验收已证明 finding 的持久化 evidence chain、查询及 allow/deny audit。另有 nftables、sysctl、关键文件、有效 sshd 配置、systemd sandbox、磁盘容量、证书生命周期和 Debian package inventory 的只读审计，以及确定性 incident evidence bundle。本地 PostgreSQL 验收已实际完成 custom-format dump 和同实例隔离数据库 restore/count reconciliation。仍无公网/VPS 部署、second-node 定时探针、审计外部导出或服务器写动作，恢复证据也不等于 off-host disaster recovery。MVP 文档中的性能与恢复指标仍为目标。

## 已选定的方向

**PROJECT: Watchhouse — Linux 安全事件与服务恢复闭环**

第一版管理一台真实 Ubuntu VPS：宿主 Nginx 提供 TLS termination，代理一个 Docker 中运行的自用服务，服务使用 PostgreSQL。另一台节点承载控制端和外部探针。恢复演练使用隔离的临时 VM。

完整链路：

```text
真实事件 → 归一化 → 服务上下文与证据 → finding
       → 受限修复计划 → 审批 → 主机重新检查
       → 执行 → 内部与外部验证 → 成功或回滚 → 事件报告
```

MVP 只做三类检测、一个生产变更动作和一条数据库恢复演练路径。宁可证明少量动作在异常条件下仍然正确，也不堆几十个浅层功能。

## 文档入口

| 文档 | 内容 |
| --- | --- |
| [GitHub landscape](docs/landscape.md) | 14 个开源项目比较、最接近的竞争者、复用边界 |
| [选题决定](docs/decision.md) | 四个候选、最终选择、实际用途与差异化 |
| [MVP 与验收](docs/mvp.md) | 第一版范围、具体检测与响应、可执行验收目标 |
| [架构](docs/architecture.md) | 组件、数据流、证据与动作协议、异常处理 |
| [威胁模型](docs/threat-model.md) | 资产、信任边界、攻击面、控制措施和限制 |
| [阶段计划](docs/milestones.md) | 每阶段的 deliverable、test、demo、completion criterion |
| [工程证据与 Demo](docs/evidence.md) | 面试官可以直接验证什么、四分钟演示 |
| [仓库结构](docs/repository.md) | 后续源码、测试、部署、文档和证据的位置 |
| [开发进度](docs/progress.md) | 已实现部分、测试证据和未完成边界 |

## 下一步

下一步把现有 Docker binding/listener correlation 与 nftables 及真正 second-node probe 组合成部署验收。当前规则处理采用每 host 有界重扫，尚未实现增量 watermark、迟到事件策略、物理磁盘硬上限或生产持续采集验收。

先解决“我们看到的是什么，哪些地方看不到”，再赋予系统修改服务器的能力。

## 运行当前切片

```sh
go test -race ./...
go build -o bin/watchhouse ./cmd/watchhouse
./bin/watchhouse replay --host lab-1 --input tests/fixtures/ssh-sequence.journal.jsonl
```

需要 Go 1.27.1。当前示例使用合成数据，输出 7 条事件和 1 条有证据引用的调查 finding，详见[重放说明](docs/runbooks/replay.md)。程序没有服务器写动作。

Linux 有 journal 读取权限时可运行 `./bin/watchhouse snapshot --host vps-staging --limit 200`，详见[快照说明](docs/runbooks/snapshot.md)。快照不等于持续监控。

`make test`、`make vet`、`make linux` 分别执行测试、静态检查和双架构 Linux 编译；`make smoke` 使用 Docker scratch 容器验证非 root、只读、无网络运行。容器 smoke 不构成真实 systemd/VPS 验收。

`watchhouse listeners` 在当前 Linux network namespace 内关联 TCP listener inode、稳定进程身份和 systemd unit，并明确报告权限盲区；详见[监听快照说明](docs/runbooks/listeners.md)。它不把监听、端口发布和外部可达性混为一谈。

`watchhouse report-listeners` 先把 snapshot 写入 SQLite v2 outbox，再通过 host-bound mTLS 上报；PostgreSQL 幂等提交并返回 exact ID 后才删除本地行。`deliver-listeners` 可独立重试最旧记录。

`watchhouse probe-https` 从执行节点记录 DNS、pinned TCP address、TLS identity/cipher、HTTP status 和 bounded body digest。真实 Nginx lab 已验证 bridge-only backend、TLS termination、504 outage 和恢复；见[probe runbook](docs/runbooks/probe-https.md)及[network evidence](evidence/test-runs/2026-10-05-network/README.md)。

`watchhouse report-probe` 先把结果写入 SQLite v3 outbox，再使用独立 `spiffe://watchhouse/probe/...` 证书提交；PostgreSQL 幂等持久化并返回 exact receipt 后才删除本地行。另有 hardened collection/retry systemd timers。

`watchhouse deliver` 使用证书绑定的 host identity、TLS 1.3 和 exact receipts 上报一个有界批次；`watchhouse-control` 在 PostgreSQL 完整提交后才返回 receipt。详见[delivery runbook](docs/runbooks/delivery.md)、[control runbook](docs/runbooks/control.md)和[control evidence](evidence/test-runs/2026-10-05-control/README.md)。

`watchhouse query-events` 和 `watchhouse query-findings` 使用独立 human certificate 与本地 role map；finding 返回 deterministic ID、规则参数和 ordered evidence event IDs。处理与查询边界见[finding contract](docs/contracts/finding-processing.md)。

`watchhouse spool ingest/status/peek/check` 可操作私有本地队列；`watchhouse collect` 用已核验的 journal cursor 做单次增量落盘。详见[队列说明](docs/runbooks/spool.md)和[增量采集](docs/runbooks/collect.md)。`make crash` 验证已提交事件在子进程突杀后仍可恢复。独立 Ubuntu guest 的 service sandbox 验收结果在 [systemd evidence](evidence/test-runs/2026-10-05-systemd/README.md)；它不是公网 VPS 或长期运行证明。

`watchhouse firewall/posture/audit-files/audit-ssh/audit-units/audit-disk/audit-cert/packages` 提供边界明确的只读主机证据；它们不会自动修改系统，也不会把单层观测冒充外部可达性或完整合规结论。对应限制和故障排查见 `docs/runbooks/` 与 `docs/contracts/`。

`watchhouse evidence-bundle create/verify` 可确定性打包已筛选 JSON 并核验内部 digest；它不替代外部签名和加密。PostgreSQL restore 验收及其未覆盖的 off-host/PITR 边界见[恢复 runbook](docs/runbooks/postgres-backup.md)。

`watchhouse docker-ports` 使用固定 Docker CLI 操作列出 running-container host bindings；`watchhouse exposure` 将它与当前 namespace 的 TCP listeners 并列关联而不推断可达性。真实 Docker Desktop Linux VM 验收见[Docker evidence](evidence/test-runs/2026-10-06-docker/README.md)。Docker socket 等价高权限，因此这些命令是短生命周期管理员诊断，不进入 unprivileged agent service。

## 成功标准

从空白 VM 可重复部署；产生真实日志；能解释一次异常；拒绝越权和过期修复；正确修复可被外部验证；失败修复能回滚；备份能恢复到隔离实例；连续自用运行并留下故障报告。

认真完成这些工作，**YES，这足以成为证明 Cybersecurity + Operations 能力的主项目**。证明力来自代码、测试、部署记录和失败处理，不来自名称、界面或技术栈数量。
