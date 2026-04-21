# Initial collector service boundary

watchhouse-collect.service 是 bounded oneshot，timer 在上一轮结束后十秒再次运行，不在一个 unit 未完成时并发启动。它不是已经具备 transport 的完整 agent daemon。

进程使用独立 watchhouse system account，没有 login shell、sudo 权限或 Linux capabilities。systemd-journal 组授予 journal 读取能力，该权限本身能看到系统日志，不等于字段级隔离。应用只保留已定义的规范化字段。

NoNewPrivileges 阻止通过 setuid 获得新权限；ProtectSystem=strict 与 ProtectHome 限制写入和个人文件读取；只有 /var/lib/watchhouse 与 private tmp 可写。AF_UNIX 是当前不联网 collector 的限制；后续 transport 增加网络能力必须明确修改并测试，不悄悄放宽。

host label 来自 root-owned /etc/watchhouse/agent.env，systemd 的 argv 变量展开不是 shell。变量仍需经过 Go host 校验。缺配置或不合法 label 时 unit 失败，不默认为未认证身份。

逻辑 queue 容量、磁盘压力、source gap 与 journalctl 诊断失败会通过 exit status/journald 体现。timer 只做重试调度，不会自动删除坏 queue 或重置 cursor；控制端尚未实现，不能称为已闭环告警。

部署文件现阶段需要 Ubuntu 255 实测验证；静态配置测试不证明 kernel sandbox 生效。参考 [systemd 255 execution settings](https://github.com/systemd/systemd/blob/v255/man/systemd.exec.xml)。
