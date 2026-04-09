# SSH journal 离线重放

## 编译与运行

需要 Go 1.27.1 或兼容的新版本。没有第三方 Go 依赖。

```sh
go test -race ./...
go build -o bin/watchhouse ./cmd/watchhouse
./bin/watchhouse replay --host lab-1 --input tests/fixtures/ssh-sequence.journal.jsonl
```

示例数据是合成 fixture：8 条记录，7 条匹配、1 条不可信来源忽略、1 条调查 finding。stdout 输出 event/finding JSON Lines；stderr 输出 summary。finding 包含 5 条失败和 1 条成功的 event_id，可在同一输出中解析得到。

读取 stdin：

```sh
cat tests/fixtures/ssh-sequence.journal.jsonl | ./bin/watchhouse replay --host lab-1
```

阈值、窗口和内存事件容量可用 --threshold、--window、--max-events 调整。事件需按 observed_at 非递减排列，重复记录在保留状态内不会重复产生 finding，但规范化 event 输出仍保留输入记录。

退出码 0 表示处理完整，不代表主机安全。1 表示输入、配置、检测或输出错误，2 表示 CLI 用法错误。已输出部分结果不会被撤回；遇到错误时 summary.complete 为 false，不能当作完整报告。

空文件处理完整但没有数据；没有 agent 心跳意义。unknown 格式、不可信来源和非认证记录统称 unmatched，这个计数不能解释成日志无安全问题。

## 实际日志与来源

Linux 可用 journalctl 的 JSON 输出作为输入。fixture replay 的 host 标签不经过认证，也不提供主机身份证明。不要提交生产日志；分享前脱敏，并确认不会改变关联所需的字段关系。

初始解析器要求 trusted _UID=0 和 _COMM 为 sshd/sshd-session，可能遗漏使用不同日志转发路径的部署。使用来源适配器前先检查真实 journal 的字段，不用 SYSLOG_IDENTIFIER 假装 trusted 来源。
