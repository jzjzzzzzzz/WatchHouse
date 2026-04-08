# 验收场景目录

| 编号 | 场景 | 成功或拒绝判据 | 层次 |
| --- | --- | --- | --- |
| EVT-01 | OpenSSH IPv4/IPv6 失败和成功行 | 标准字段正确，cursor 身份稳定 | 单元 |
| EVT-02 | 假 SYSLOG_IDENTIFIER 和 MESSAGE | 非 trusted comm 不产生认证事件 | 单元 |
| EVT-03 | malformed/重复字段/坏时间/IP/port/超大行 | 显式错误，无伪事件 | 单元与 fuzz |
| EVT-04 | journal 重放 | 同记录 event_id 相同 | 单元 |
| DET-01 | 同 host、boot、用户、IP 窗口内失败后成功 | finding 包含失败和成功 event_id | 单元 |
| DET-02 | NAT 不同用户、过期失败、不同 boot | 不合并出高优先级 finding | 单元 |
| DET-03 | 重复、乱序与容量超限 | 去重或显式拒绝，不无界增长 | 单元 |
| HOST-01 | PID 退出/复用和缺权限 | unknown/partial，不伪造归属 | 真实 Linux |
| HOST-02 | Docker published port 无宿主监听 PID | 发布元数据和探针结果分开 | 真实 Linux |
| QUEUE-01 | 断网补发、重启和队列满 | 达到 MVP 约定，丢弃范围可见 | 集成 |
| AUTH-01 | 主机冒用、viewer 审批 | 拒绝且有审计记录 | 集成 |
| ACT-01 | 过期、重放与前置状态变化 | 不发生未批准变更 | 集成 |
| ACT-02 | 写入步骤崩溃与确认丢失 | 重读后恢复，超时回滚 | 故障注入 |
| ACT-03 | 回滚失败与晚到外部确认 | rollback_failed 不误报 verified | 故障注入 |
| DEPLOY-01 | 空白 VM 和再次部署 | 可重现，权限与暴露验证 | 真实 Linux |
| RESTORE-01 | 好/坏数据库导出物 | 隔离业务检查决定成功 | 恢复 VM |

场景目录用于追踪，不等于所有场景已经通过。阶段状态与具体运行结果记录在 evidence 中。
