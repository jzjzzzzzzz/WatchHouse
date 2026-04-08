# 只读事件契约 v1

事件采用 JSON Lines，每行一个对象。schema_version 固定为 1；首次切片仅实现 ssh.authentication，不假装支持所有 telemetry。

必填字段：event_id、host_id、boot_id、source、source_cursor、observed_at、received_at、kind、authentication。source 固定 journald。认证载荷包含 outcome、method、user、source_ip 和 source_port。

event_id 为 host_id、boot_id 和 journal cursor 的长度前缀编码的 SHA-256。同一 journal 记录重放身份稳定。时间为 UTC RFC3339Nano，journal 微秒时间必须有效。

认证解析只接受 journal 的 trusted _COMM 为 sshd 或 sshd-session。MESSAGE 中的程序名或 SYSLOG_IDENTIFIER 不作为来源信任证明。fixture 文件可以伪造 trusted 字段，因此 fixture replay 不构成主机认证证据。

初始支持 OpenSSH 的 Failed password/publickey 与 Accepted password/publickey 行，支持 invalid user、IPv4 和 IPv6。未知格式返回 unmatched 并计数；输入畸形、重复 JSON 字段、字段类型错误、缺失 cursor/boot/timestamp、非法 IP/port 返回错误，不能默认为无事件。

每行最大 64 KiB。不上传原始 MESSAGE，只保留 source cursor 和规范化字段；该切片不提供原始日志的长期保全机制。source cursor 仅用于追溯，不证明远端原始记录永远存在。

未来上传时，host_id 由证书身份决定；当前离线 CLI 的 host 参数是调用者提供的标记，不能当作 mTLS 身份。
