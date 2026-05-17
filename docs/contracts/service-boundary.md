# Initial collector service boundary

watchhouse-collect.service 是 bounded oneshot，timer 在上一轮结束后十秒再次运行，不在一个 unit 未完成时并发启动。它不是已经具备 transport 的完整 agent daemon。

进程使用独立 watchhouse system account，没有 login shell、sudo 权限或 Linux capabilities。systemd-journal 组授予 journal 读取能力，该权限本身能看到系统日志，不等于字段级隔离。应用只保留已定义的规范化字段。

NoNewPrivileges 阻止通过 setuid 获得新权限；ProtectSystem=strict 与 ProtectHome 限制写入和个人文件读取；只有 /var/lib/watchhouse 与 private tmp 可写。AF_UNIX 仍是 journal collector 的限制；网络能力没有加给该进程。独立 delivery oneshot 才允许 AF_UNIX/AF_INET/AF_INET6，且不属于 systemd-journal 组。它只能读写同一私有 spool，并通过 `LoadCredential` 取得 agent CA/certificate/key。

host label 来自 root-owned /etc/watchhouse/agent.env，systemd 的 argv 变量展开不是 shell。变量仍需经过 Go host 校验。缺配置或不合法 label 时 unit 失败，不默认为未认证身份。

逻辑 queue 容量、磁盘压力、source gap 与 journalctl 诊断失败会通过 exit status/journald 体现。collector timer 只做采集重试，不会自动删除坏 queue 或重置 cursor；delivery timer 只在上一轮结束后再发送一个有界批次。控制端 ingest/query 已实现，但持久规则处理、告警与响应闭环尚未完成。

2026-10-05 local 的 Ubuntu systemd 255 guest 验收实际观察到：service UID 999、无 sudo group、零 effective capabilities、NoNewPrivileges、seccomp mode 2、ProtectSystem=strict、ProtectHome=yes，并完成一次 native collection 与 queue audit。公开摘要见 [systemd evidence](../../evidence/test-runs/2026-10-05-systemd/README.md)。world-readable `/home` fixture 在普通 service UID 下可读、在 mirrored sandbox 中不可读，避免把普通 DAC 权限误当作 ProtectHome 证据。

该验收只证明固定的 read-only unit 在独立本地 VM 上生效；不证明公网 VPS、长期运行、网络 transport 或后续 privileged executor。参考 [systemd 255 execution settings](https://github.com/systemd/systemd/blob/v255/man/systemd.exec.xml)。

2026-10-05 22:23 EDT 的后续 guest 验收实际运行了独立 delivery unit 和 control unit。delivery 在 NoNewPrivileges/ProtectSystem/ProtectHome 下把隔离 queue 从七条降到零，PostgreSQL 恰有七条；control 由 DynamicUser 运行、使用五个 systemd credentials，并用 viewer certificate 返回三条查询结果。证据见 [control systemd result](../../evidence/test-runs/2026-10-05-control/README.md)。该数据库链路是未发布 Docker bridge 且未启用 DB TLS，不能外推为生产网络验收。
