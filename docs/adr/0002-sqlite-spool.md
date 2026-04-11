# Transactional SQLite spool

选择 modernc.org/sqlite v1.60.1，保留 CGO_ENABLED=0 和 Linux 双架构构建。依赖通过 go.mod/go.sum 固定；不是自行重写数据库。

一个进程使用一个 SQLite connection；WAL、synchronous=FULL、foreign_keys=ON、busy_timeout=5000。文件放在私有 state 目录。只支持当前 schema，发现更高版本拒绝，不能悄悄降级或丢弃数据。

events 以 AUTOINCREMENT sequence 排序，event_id 在待发送队列内唯一；checkpoints 与事件在同一事务写入；queue_state 记录 payload 字节和记录数。容量指逻辑 payload，不代表数据库/WAL 物理文件总大小，后续需单独测量并限制物理磁盘使用。

目录位于本地文件系统。WAL 不适合作为跨主机共享网络队列。[SQLite WAL](https://www.sqlite.org/wal.html)、[Transactions](https://www.sqlite.org/lang_transaction.html)、[driver documentation](https://pkg.go.dev/modernc.org/sqlite)。
