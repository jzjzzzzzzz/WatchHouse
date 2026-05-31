# Agent spool contract v2

Append 在一个 SQLite transaction 中写入规范化 event、payload 计数和 source cursor。只有所有写入提交成功才返回 Inserted。测试在 checkpoint 写入阶段注入 SQL trigger 失败，核验 event 和计数都回滚。

Checkpoint 以 host 和 journald.ssh stream 为 scope，使用 ExpectedCursor 做 compare-and-swap。并发来源进度变化时拒绝，不覆盖更新者。cursor 是 opaque token，不用字符串大小猜测时间顺序。

待发送队列按 event_id 去重，相同身份但内容变化拒绝；ReceivedAt 不参与内容摘要，因为重放接收时间可以不同。当前保留 pending 身份去重，不提供无限时间的历史去重。

重放已经 pending 的旧事件不能回退当前 source cursor；返回 Duplicate，不推进 checkpoint。相同当前 cursor 的重试同样不修改状态。

已明确处理的 unmatched record 可通过 nil event 事务性推进 cursor。malformed record 不能假装 unmatched；是否隔离到 quarantine 留给后续明确设计。

队列满时采取 backpressure：不插入、不推进该记录的 cursor，持久化 blocked_attempts。该计数不是 dropped events；没有主动丢弃策略。断网过久导致源 journal 轮转仍可能丢数据，需要后续 cursor 缺口告警，不能提前承诺不会丢日志。

逻辑 payload 上限默认 100 MiB、记录上限默认 100,000。容量不是物理磁盘总上限，未完成原设计的 100 MiB 总磁盘验收。后续物理限制、checkpoint 和 vacuum 必须单独验证。

Peek 按 insertion sequence 返回最多 500 条、最多 8 MiB payload 的批次，不删除记录；预算连第一条都容纳不了时显式报错。读取时验证 event 身份和内容摘要，拒绝损坏数据。

Ack 接收精确 sequence + event_id receipt，在一个事务里删除这些记录并更新计数；不采用累计序号删除。已不存在的 receipt 幂等忽略，现存 sequence 与 identity 冲突则整个 batch 回滚。AUTOINCREMENT 防止旧 receipt 删除复用序号的新事件。

只有 transport 完成远端认证并获得提交成功的 receipt 后才可调用 Ack。store 本身不能验证远端身份。删除后的重复事件仍需由控制端持久化 event_id 去重；本地 cursor 防止正常 journal resume 重读，不提供无限本地 receipt 历史。

v2 迁移保留原有 event、checkpoint 和计数，新增独立 listener snapshot outbox 与计数。snapshot 在网络调用前提交，按 snapshot ID 去重并校验 host、boot、network namespace、observed time 和完整 payload digest。listener Ack 同样要求精确 local sequence + snapshot ID；错误或损坏 receipt 不删除记录。启动计数核验和只读 Audit 同时覆盖两个 outbox。两个 stream 各自使用配置的逻辑容量，因此配置值不是二者合计的物理磁盘硬上限。
