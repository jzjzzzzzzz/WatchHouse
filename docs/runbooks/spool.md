# 本地持久化队列

```sh
go build -o bin/watchhouse ./cmd/watchhouse
work=$(mktemp -d)
./bin/watchhouse spool init --state "$work/state"
./bin/watchhouse spool ingest --state "$work/state" --host lab-1 --input tests/fixtures/ssh-sequence.journal.jsonl
./bin/watchhouse spool status --state "$work/state"
./bin/watchhouse spool peek --state "$work/state" --limit 2
./bin/watchhouse spool check --state "$work/state"
```

输入为合成示例，队列应有 7 条记录。重复 ingest 相同完整文件会找到持久化 cursor 并跳过此前 8 条输入，队列不会变成 14 条。

默认逻辑 payload 100 MiB、100,000 个 pending event；--max-bytes 与 --max-records 可缩小以测试 backpressure。容量满时 ingest 返回 1、complete=false，未落盘记录的 source cursor 不推进。status 的 blocked_attempts 反映阻塞尝试，不代表丢弃。

peek 不消费 event，不表示上报成功；CLI 不暴露手动 Ack。`deliver` 只在验证 event receipts 后删除 event，`report-listeners`/`deliver-listeners` 只在验证 exact snapshot receipt 后删除 listener outbox row。

status/peek 要求队列已经存在，不自动创建；init/ingest 可创建私有 state 子目录，但其父目录必须已存在。不会自动修复现有不安全权限。stdout 输出 JSON，错误写 stderr。实际数据库/WAL 文件可能超过逻辑 payload 上限，不能宣称磁盘硬上限已完成。

文件 resume 要求完整文件保留已保存的 cursor；tail 缺失 cursor 会明确报 source gap。不要用可信度不足的 fixture 冒充 live journal 身份或无缺口采集。

`make crash` 启动自己创建的 ingest 子进程，确认两条记录已经事务性提交后 SIGKILL，仅杀该子进程；重启验证两条记录仍在，完整文件 resume 后队列为七条且无重复。这验证进程突杀与 WAL 恢复，不是磁盘损坏或真实掉电测试。测试只使用临时目录和合成数据。

check 在一致的只读 transaction 中执行 SQLite quick_check、逐条 event 与 listener snapshot 身份/内容摘要/host 索引校验、两个 outbox 的 payload 计数比较和 checkpoint scope 校验。失败不自动修复或删除数据。若启动时已发现 schema 或计数损坏，Open 会先拒绝；保留文件供后续人工调查，不绕过检查。

内容摘要不包含 ReceivedAt，以允许来源事件重放后接收时间变化；它不是签名或抗 root 篡改机制。
