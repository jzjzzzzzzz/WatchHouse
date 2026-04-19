# Durable spool test record

实际执行日期：2026-10-05（America/New_York）；manifest 的 UTC 时间可能为 2026-10-06。回溯 Git 日期不用于说明真实开发或运行时间。

被测源码：8f71484，具体完整 commit 与环境见 manifest.json。素材为合成 SSH fixture，不包含生产 telemetry 或凭证。

## 已验证

- 完整本地测试、race、vet 和格式检查通过；8 个 Go package，5 个 Python 日期编排测试。
- SQLite event/checkpoint transaction 回滚、并发 CAS、pending duplicate 不倒退 cursor、容量阻塞与精确 Ack 测试通过。
- foreign database 拒绝且文件摘要不变；缺失 schema 与错误计数拒绝；只读 audit 校验正常及破坏状态。
- Linux amd64/arm64 静态编译通过；Linux aarch64 scratch 环境以 UID 65534、read-only rootfs、无网络、无 capabilities 执行 spool 测试通过。只有受限 /tmp 可写。
- 实际 SIGKILL ingest 子进程后，两条已提交记录保留；完整 fixture resume 只新增五条，最终为七条。
- JournalPosition 独立 fuzz 目标执行 816,595 次、实际 11.353 秒，通过，无失败输入。

## 保存的结果

ingest.json：首次插入七条、一个 unmatched，处理完整。

resume.json：找到持久化 cursor，跳过八条原始记录，不重复插入。

status.json：pending_records=7，blocked_attempts=0。

audit.json：七条 payload、一个 source checkpoint，通过一致性检查。

crash.json：进程突杀和重启恢复结果，明确不是掉电或磁盘损坏模拟。

## 边界

Native journal reader 仍只有 runner 测试，未在真实 systemd VM/VPS 上验证。尚无 transport、mTLS、控制端、常驻 agent、100 MiB 物理磁盘硬限制、写动作或业务数据库恢复。CLI queue 上限是逻辑 payload，不是整个 SQLite 文件加 WAL/SHM 大小。

复现：make test、make vet、make linux、make smoke、make crash，以及 docs/runbooks/spool.md 的完整文件 ingest/resume/check 命令。GitHub workflow 尚未 push，因此没有远端 CI 执行结果。
