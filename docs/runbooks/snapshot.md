# Linux journal 只读快照

```sh
go build -o bin/watchhouse ./cmd/watchhouse
./bin/watchhouse snapshot --host vps-staging --limit 200
```

snapshot 仅支持 Linux，固定调用 /usr/bin/journalctl，按 trusted _COMM 过滤 sshd 和 sshd-session；不会调用 shell、安装软件、申请 sudo、写配置或修改服务。

读取最新 limit 条匹配记录，默认 200、最多 1000；输出仍经过来源 UID 检查、规范化和 SSH 关联规则。stdout/stderr 协议与 replay 一致。

子进程有十秒超时；stdout 容量为 limit × 64 KiB 左右，stderr 上限 16 KiB。journalctl 非零退出、输出超限、取消或任何 stderr 诊断都会失败。原因是 journalctl 在权限不足时可能只输出提示却返回零，不能把部分可见日志当完整视图。

调用者需要原有 journal 读取权限，例如系统提供的 systemd-journal/adm 组权限；程序不自行提权。不要为了这个切片开放远程 root shell。

这是有界历史快照，不是常驻 agent：没有 cursor 持久化、补发、心跳或 journal 缺口检测；超过 limit 的记录不会被读取。complete=true 只表示所捕获记录成功处理，不证明完整主机日志、持续检测覆盖或主机安全。即使没有 stderr，也可能因 journal 命名空间或系统 ACL 而看不到全部记录。

当前尚未在真实 systemd 主机上完成集成验收。Mac 会明确拒绝 native snapshot；Linux 容器无 journal 也不能充当该验收。

字段与工具语义参考官方 [systemd journal fields](https://github.com/systemd/systemd/blob/main/man/systemd.journal-fields.xml)、[journalctl](https://github.com/systemd/systemd/blob/main/man/journalctl.xml) 和 [OpenSSH authentication logging](https://github.com/openssh/openssh-portable/blob/master/auth.c)。
