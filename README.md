# Watchhouse

面向小规模 Linux 和 VPS 部署的安全运维闭环。

Watchhouse 围绕一个具体问题展开：服务器出现异常或服务故障时，能否说明发生了什么，在明确权限边界内完成修复，并用独立检查证明服务和安全约束已经恢复。

它不是服务器管理面板，也不是新的 SIEM。项目核心是带证据的事件调查、受限修复协议、故障恢复和可复现验证。

## 当前状态

2026-10-05：开发初期。已实现 SSH journal 规范化、有界认证检测、replay/snapshot CLI、核验 cursor 的 native forward collector，以及 SQLite 持久化队列、精确 receipt Ack、容量 backpressure 与只读审计。附单元、race、fuzz、Linux 容器测试、子进程 SIGKILL 恢复测试，以及独立 Ubuntu QEMU guest 中的真实 systemd/OpenSSH 验收。当前有 timer 驱动的 bounded collector、严格 mTLS batch transport、PostgreSQL event receiver 和证书隔离的 human query；真实 systemd control/delivery 验收已通过。尚无公网/VPS 部署、外部探针、查询审计或服务器写动作。MVP 文档中的性能与恢复指标仍为目标。

Git 日期按用户指定的 2026-04-08 至 2026-10-05 区间回溯编排；实际开发从 2026-10-05 开始。提交 trailer 保留实际执行时间。日期覆盖不是半年真实开发或运行证明，详见[开发与提交要求](docs/requirements.md)。

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

下一步用隔离的 Ubuntu systemd VM 验证真实 OpenSSH 和 journal resume，再加入主机身份、网络上报和服务/socket 观测。cursor 持久化与本地队列已完成初始切片，但尚无物理磁盘硬上限和生产持续采集验收。

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

`watchhouse deliver` 使用证书绑定的 host identity、TLS 1.3 和 exact receipts 上报一个有界批次；`watchhouse-control` 在 PostgreSQL 完整提交后才返回 receipt。详见[delivery runbook](docs/runbooks/delivery.md)、[control runbook](docs/runbooks/control.md)和[control evidence](evidence/test-runs/2026-10-05-control/README.md)。

`watchhouse spool ingest/status/peek/check` 可操作私有本地队列；`watchhouse collect` 用已核验的 journal cursor 做单次增量落盘。详见[队列说明](docs/runbooks/spool.md)和[增量采集](docs/runbooks/collect.md)。`make crash` 验证已提交事件在子进程突杀后仍可恢复。独立 Ubuntu guest 的 service sandbox 验收结果在 [systemd evidence](evidence/test-runs/2026-10-05-systemd/README.md)；它不是公网 VPS 或长期运行证明。

## 成功标准

从空白 VM 可重复部署；产生真实日志；能解释一次异常；拒绝越权和过期修复；正确修复可被外部验证；失败修复能回滚；备份能恢复到隔离实例；连续自用运行并留下故障报告。

认真完成这些工作，**YES，这足以成为证明 Cybersecurity + Operations 能力的主项目**。证明力来自代码、测试、部署记录和失败处理，不来自名称、界面或技术栈数量。
