# Runbook index

所有 runbook 都应说明前置条件、执行命令、预期输出、失败语义和结论边界。除非文档明确说明，主机检查均为只读操作。

## 采集与队列

- [离线 journal 重放](replay.md)
- [有界 journal 快照](snapshot.md)
- [增量 journal 采集](collect.md)
- [SQLite spool 操作](spool.md)

## 传输与控制面

- [mTLS 事件交付](delivery.md)
- [控制面运行](control.md)
- [HTTPS 外部探测](probe-https.md)

## 主机与网络审计

- [TCP listener 快照](listeners.md)
- [nftables 摘要](firewall.md)
- [有效 sshd 配置](audit-ssh.md)
- [systemd unit sandbox](audit-units.md)
- [磁盘容量](audit-disk.md)
- [证书生命周期](audit-cert.md)
- [Debian package inventory](packages.md)
- [Docker port bindings](docker-ports.md)

## 证据与恢复

- [事件证据包](evidence-bundle.md)
- [PostgreSQL backup/restore 验收](postgres-backup.md)

