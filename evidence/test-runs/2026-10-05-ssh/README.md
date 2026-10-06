# SSH 只读链路测试记录

实际执行日期：2026-10-05。被测源码 commit：8b6b9bba8acf0294ee6871684e5b7d29a5c105d5。

## 环境与输入

- 开发主机：Darwin arm64。
- Toolchain：Go 1.27.1 darwin/arm64。
- Docker：Linux aarch64，Server 29.6.1。
- 输入：tests/fixtures/ssh-sequence.journal.jsonl，完全合成，使用文档地址段和测试用户名。
- manifest.json 保留实际执行时间、commit、环境和输入摘要。

## 已执行的验证

| 检查 | 结果 |
| --- | --- |
| go test -count=1 -race -coverprofile=coverage.out ./... | 五个 package 全部通过，未报告 race；整体 statement coverage 88.0% |
| go vet ./... 与格式检查 | 通过 |
| Linux amd64/arm64 静态交叉编译 | 两种架构通过 |
| Docker scratch Linux smoke | 非 root、只读 rootfs、无网络、cap-drop ALL、no-new-privileges 条件下通过 |
| 缺 journalctl 的 Linux snapshot | 明确失败，没有把容器误报为可读取 systemd journal |
| Mac native snapshot | 明确拒绝非 Linux 平台 |
| FuzzParseJournal，目标 10 秒 | 通过；此次 warm corpus 308 seeds，527,604 次执行，实际 11.370 秒，未发现失败输入 |

22 个顶层 Go Test 函数包含多组表驱动子测试，另外有一个 fuzz target。覆盖率不是可靠性的充分证明；真实 OS 接口分支和未实现的持续采集不由这些测试覆盖。

## 重放结果

summary.json：8 条输入、7 条匹配、1 条 unmatched、1 个 finding，complete=true。

output.jsonl 保存 7 个事件和 1 个 finding。finding 引用的 6 个 event_id 都能在同一输出找到。原始 MESSAGE、公钥 fingerprint 和命令字符串不进入规范化输出。

重现命令：

```sh
make test
make vet
make linux
make smoke
go test ./internal/telemetry -run '^$' -fuzz FuzzParseJournal -fuzztime 10s
go build -o bin/watchhouse ./cmd/watchhouse
./bin/watchhouse replay --host lab-1 --input tests/fixtures/ssh-sequence.journal.jsonl
```

重新运行的 received_at 和实际执行时间会变化，event_id 和 finding_id 在同配置下应保持稳定。fuzz 执行数量受机器和 corpus 影响，不要求重复得到相同计数。

## 尚未证明

没有真实 systemd 主机/VPS 集成记录，没有持久化队列、journal 缺口检测、mTLS、控制端、socket/unit 归属、写动作或数据库恢复。GitHub Actions workflow 已加入仓库，但尚未 push，不存在远端 CI 运行结果。
