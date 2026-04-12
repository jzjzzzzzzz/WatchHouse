# Agent spool contract v1

Append 在一个 SQLite transaction 中写入规范化 event、payload 计数和 source cursor。只有所有写入提交成功才返回 Inserted。测试在 checkpoint 写入阶段注入 SQL trigger 失败，核验 event 和计数都回滚。

Checkpoint 以 host 和 journald.ssh stream 为 scope，使用 ExpectedCursor 做 compare-and-swap。并发来源进度变化时拒绝，不覆盖更新者。cursor 是 opaque token，不用字符串大小猜测时间顺序。

待发送队列按 event_id 去重，相同身份但内容变化拒绝；ReceivedAt 不参与内容摘要，因为重放接收时间可以不同。当前保留 pending 身份去重，不提供无限时间的历史去重。

已明确处理的 unmatched record 可通过 nil event 事务性推进 cursor。malformed record 不能假装 unmatched；是否隔离到 quarantine 留给后续明确设计。

队列满时采取 backpressure：不插入、不推进该记录的 cursor，持久化 blocked_attempts。该计数不是 dropped events；没有主动丢弃策略。断网过久导致源 journal 轮转仍可能丢数据，需要后续 cursor 缺口告警，不能提前承诺不会丢日志。

逻辑 payload 上限默认 100 MiB、记录上限默认 100,000。容量不是物理磁盘总上限，未完成原设计的 100 MiB 总磁盘验收。后续物理限制、checkpoint 和 vacuum 必须单独验证。
