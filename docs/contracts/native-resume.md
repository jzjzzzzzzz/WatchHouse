# Native journal resume

增量 Poll 使用 journalctl --cursor=保存值 --no-tail，并验证返回的第一条记录 cursor 与保存值完全相同；该记录只验证、不重新入队，然后读取其后的前 N 条记录。首次没有 cursor 时从匹配 stream 的历史头部开始，不悄悄跳到最近记录。

不能组合 --after-cursor 与 --lines=N 后假设得到最早 N 条新记录；选择 tail 可能跳过积压。当前 bounded reader 在读到 N 条后取消子进程，下一轮从最后成功落盘的 cursor 继续。

返回首条不匹配、保存 cursor 无记录、工具报错、stderr 诊断或畸形 metadata 时不返回可落盘数据，cursor 不自动重置。journal 轮转导致的 gap 必须调查；尚无自动 quarantine 或安全重置操作。

Poll 对应单元 runner 测试，真实 journalctl 行为仍需 systemd VM 集成验证。十秒超时、64 KiB 单条限制、最多 1000 条、16 KiB diagnostics 与无 shell/无提权边界保留。
