# 持久化 Linux 增量采集

```sh
./bin/watchhouse collect --host vps-staging --state /path/to/private-state --limit 200
./bin/watchhouse spool status --state /path/to/private-state
```

读取私有 SQLite 中的 cursor，调用 verified native Poll，然后逐条原子落盘。首次从 SSH stream 头部读取；再次调用从保存 cursor 后继续，不只是抓最近 200 条。当前是单次采集，不是常驻 daemon，也没有网络上报。

cursor 缺失/不匹配或权限诊断会失败，不自动重置；容量满也停止，不推进未存储记录。不要删除数据库后称为“无缝恢复”；删除状态会丢失未上报的 queue 和来源进度。

Mac 不支持 native journal；collect 可能先创建本地私有状态，然后明确报平台不支持，但不会修改服务器配置。真实 Linux systemd 集成尚未验收；当前 native reader 仅有 runner unit tests。
