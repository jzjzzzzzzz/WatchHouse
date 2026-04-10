# 开发进度

实际日期：2026-10-05。

## 当前可运行部分

- 可信字段与 root emitting UID 过滤的 SSH journal 解析。
- 64 KiB 输入限制、重复顶层字段拒绝、UTF-8 验证和稳定事件标识。
- 认证失败后成功的有界关联规则，按 host、boot、用户和来源 IP 分组。
- 证据引用、状态容量限制、去重、乱序和身份冲突错误。
- replay CLI，JSON Lines 输出、处理 summary 和部分结果标记。
- Linux 有界 journal snapshot，固定 argv、不用 shell、不申请提权、限制输出并设置超时。
- 单元、race、fuzz、静态检查、Linux 双架构编译和隔离 Linux 容器 smoke。

测试证据：[SSH 只读链路](../evidence/test-runs/2026-10-05-ssh/README.md)。

## 阶段状态

| 阶段 | 状态 |
| --- | --- |
| M0 初始契约 | 事件、规则、响应状态和场景目录已写；新增真实接口仍需对应契约 |
| M1 只读观测 | 部分完成：journal 快照；缺常驻 agent、cursor 持久化、queue、unit/socket/container 采集及真实 VM 验证 |
| M2 控制面 | 未开始；离线规则实现不代表控制端完成 |
| M3 写动作 | 未开始 |
| M4 部署和数据库恢复 | 未开始 |
| M5 第一版发布 | 未完成 |
| M6 连续自用 | 未开始 |

## 下一切片

先准备真实 Linux systemd 集成环境，验证 OpenSSH 实际日志的 emitting UID、comm 和 JSON 结构，再实现 cursor 持久化与 SQLite 有界队列。

不把 snapshot 变成轮询 tail 后假称“不丢日志”：必须先规定 cursor 与事件落盘的顺序、崩溃重放、重复身份、缺口和容量满时的行为。之后再做 unit/socket 观测和控制面。
