# 工程证据与四分钟 Demo

## 面试官可以直接验证什么

下表是最终验收目标；每行是否已经完成必须由链接的机器结果和当前
`docs/progress.md` 判断，不能因为出现在表中就宣称完成。

| 可验证能力 | 仓库与现场证据 |
| --- | --- |
| 管理 systemd 服务并从 journald 定位失败 | 原生 unit、启动失败集成测试、exit status、journal cursor 和故障报告 |
| 将网络资源归属到实际进程和服务 | socket inode/PID/start_time/cgroup 关联测试；进程退出和 unknown 负例 |
| 正确解释 Docker 端口暴露 | published port、NAT/FORWARD 检查和外部 TCP 结果；不错误要求宿主一定有监听 PID |
| 配置 TLS termination 和反向代理 | Nginx 配置、证书验证、SNI/hostname 检查、upstream 502 场景 |
| 分层排查 DNS、TCP、TLS 和 HTTP | 各层独立失败 fixtures 与探针报告，而不是一个 health bool |
| 调查异常认证活动 | 真实 SSH journal、规则版本、窗口和证据；NAT/合法重试误报测试 |
| 设计最小权限执行边界 | 非 root 网络进程、helper 接口、固定资源、路径/动作负例 |
| 落实认证和授权 | 主机身份绑定、人类角色、viewer 拒绝、跨主机请求拒绝 |
| 安全处理审批与环境变化 | 审批 digest、期限、执行前重验；旧 plan 被拒绝的测试 |
| 处理重复消息和崩溃 | journal、状态转换、步骤级故障注入、重复 command_id 结果 |
| 验证修复并处理回滚失败 | 外部健康确认、超时回滚、rollback_failed 和仍打开的 finding |
| 进行可控数据库恢复测试 | 一致性导出、隔离恢复、schema/业务检查、实测 RPO/RTO |
| 实现 secrets 管理而非硬编码 | 加密配置、运行时权限、泄漏 fixture 测试、轮换说明 |
| 可重复部署与 CI/CD | 空白 VM 部署记录、二次运行结果、固定依赖、CI 与发布摘要 |
| 观测监控系统自身 | agent offline、队列满、解析失败、延迟和资源指标 |
| 开展真实事件响应 | 时间线、保全证据、处置原因、恢复验证与 postmortem |

## 证据发布规则

每份测试结果附 commit、发行版、内核、Docker 版本、环境、数据规模和执行时间。区分单元测试、fixture replay、真实 VM 场景和生产事件。

公开日志必须脱敏，不包含真实 IP、用户名、域名、密钥、业务数据或备份内容。脱敏不能改变规则依赖的字段关系；原始敏感证据只保留在私有受控位置。

报告保留失败和未覆盖项。不只放绿色截图；至少有一个失败修复被阻止、一个修复后回滚和一个真实故障 postmortem。

## 当前可复核证据索引

| 运行 | 已证明 | 未证明 |
| --- | --- | --- |
| [SSH](../evidence/test-runs/2026-10-05-ssh/README.md) | OpenSSH/journal parsing、UID/comm filtering、ordered detection | 长期生产采集、攻击者归因 |
| [SQLite crash](../evidence/test-runs/2026-10-05-spool/README.md) | transaction、backpressure、SIGKILL 后 committed queue 恢复 | 物理磁盘损坏、跨主机恢复 |
| [Ubuntu systemd](../evidence/test-runs/2026-10-05-systemd/README.md) | 原生 systemd/OpenSSH、sandbox、reboot/cursor continuation | 公网 VPS、长期运行 |
| [Control/PostgreSQL](../evidence/test-runs/2026-10-05-control/README.md) | mTLS identity、exact receipts、findings/query audit、同实例 logical restore | production DB TLS、off-host/PITR disaster recovery |
| [Nginx network](../evidence/test-runs/2026-10-05-network/README.md) | digest-pinned non-root edge、TLS、200→504→200 | public ingress、multi-vantage probing |
| [Docker binding](../evidence/test-runs/2026-10-06-docker/README.md) | Linux collector 对真实 loopback published binding 的准确识别 | native VPS firewall traversal、Internet reachability |

## 四分钟主 Demo

演示使用真实 VPS 上的 staging 服务，不修改生产数据。事先准备相同部署拓扑、固定已批准配置版本和外部探针。场景注入器只存在于 staging 测试，不作为生产响应接口。

### 0:00 到 0:35 展示基线

展示两节点拓扑、agent 心跳、正常 HTTPS 请求、Nginx unit 和注册服务。用 CLI 显示预期监听与实际暴露相符。

### 0:35 到 1:20 制造真实配置故障

在 staging 将受管 Nginx upstream 改为错误端口并 reload。配置语法合法，但实际请求返回 502。

系统同时出现配置偏移、HTTP 失败以及 upstream 连接失败日志。展示不是“服务红了”，而是“进程活着、TLS 正常、upstream 不可达”。

### 1:20 到 2:00 查看证据和权限边界

打开 finding，展示 journal 引用、当前配置摘要、unit、端口和分层探针。viewer 尝试审批被拒绝；operator 审阅固定目标、前置状态、期限和验证检查。

### 2:00 到 3:10 恢复并验证

operator 批准。agent 重验前置状态，helper 恢复 approved bundle，`nginx -t` 和 reload 后等待外部 HTTPS/业务检查。

展示 verified 终态、真实响应、动作前后摘要和统一 command_id。重复送达相同 command_id 不再次 reload。

### 3:10 到 4:00 展示失败与恢复证据

展示已执行的过期审批、审批后状态变化、验证失败回滚、agent 重启测试报告，并明确这些是先前运行的测试，不冒充现场动作。

最后展示最近一次隔离 PostgreSQL 恢复报告、数据规模和实测时间。恢复演练不必压缩为四分钟内完成，但必须有可重跑命令和完整证据。

## 单独的深度 Demo

另录制一个 10 到 15 分钟演示，现场执行确认丢失、helper 崩溃、回滚失败、数据库恢复和外部端口漂移验证。主 Demo 证明链路，深度 Demo 证明异常处理。
