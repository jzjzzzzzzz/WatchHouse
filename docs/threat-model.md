# 威胁模型与安全边界

## 保护的资产

受管服务可用性、Nginx 配置和已批准版本、主机和人类身份、审批记录、事件证据、应用数据、备份、数据库与解密凭证。

系统最敏感的能力是修改服务器。检测失误不能直接转换为不受约束的 root 操作。

## 攻击者与信任假设

覆盖公网访问者、可控制日志字段的人、获得低权限 workload 访问的人、盗用 viewer 身份的人、伪造网络请求的人，以及失陷控制端导致的滥用。

部署管理员、初始 approved bundle 和主机本地允许配置属于信任根。已经完全控制宿主 root 的攻击者不在完整防御保证内：他能伪造 telemetry、替换 agent 或修改 helper。远端证据和外部探针只能增加佐证，不能消除该限制。

## 边界与测试

| 风险 | 设计控制 | 必须验证的负例 |
| --- | --- | --- |
| 日志注入或解析混淆 | 日志作为数据处理；长度限制、结构化解析、输出转义 | shell 字符、换行、异常编码和超大字段不导致执行或假事件 |
| 主机身份冒用 | mTLS 身份绑定 host_id，服务端拒绝 body 覆盖 | 主机 A 上报 B 的身份被拒绝 |
| 越权响应 | viewer/operator/admin 分离，agent 独立权限域 | viewer 审批、人类冒充 agent 均拒绝 |
| 审批被替换或重放 | digest 绑定、有效期、持久化唯一 command_id | 参数变化、过期、跨资源复用与重复投递 |
| 审批后状态变化 | 执行前重新计算状态摘要 | 新配置覆盖旧 plan 的预期状态时拒绝 |
| helper 成为 root shell | 类型化动作、固定路径、固定 bundle、无任意 exec | 越界路径、符号链接、路径穿越和未知动作拒绝 |
| 控制端失陷 | 主机本地 allowlist 不能被远端扩大；出站管理 | 控制端要求操作未登记资源时拒绝 |
| Docker 权限扩大 | 普通 agent 无 Docker socket；有限元数据输出 | 网络 coordinator 无法获得任意 Docker API 能力 |
| 管理员诊断被当作 agent 权限 | Docker inventory 仅作为短生命周期本地诊断；service unit 不挂 socket、不进 docker group | unprivileged service 无法运行 Docker inventory，socket 缺失时 fail closed |
| 证据归档路径穿越或混入 | 只接收 bounded top-level regular JSON；确定性 manifest；verify 不解压落盘 | symlink、重复名、`../`、未声明文件、digest/length mismatch 均拒绝 |
| 修复导致停机 | 前态捕获、语法检查、健康验证、超时回滚 | 配置合法但 upstream 错误、reload 失败、外部确认丢失 |
| 控制面或 agent 崩溃 | 持久化意图、资源锁、状态重读 | 每个变更步骤断电/杀进程后恢复 |
| 观测停止 | heartbeat、cursor 缺口、权限错误和队列丢弃指标 | agent 离线、journal 轮转、磁盘满可见 |
| 备份不可恢复 | 一致性导出、隔离恢复、业务校验 | 损坏导出物、缺少凭证、schema 不符不能判定成功 |
| 秘密泄漏 | 最小采集、脱敏、受限文件和数据保留 | 日志、错误响应、导出报告中不出现 fixture secret |

## 不能过度承诺的地方

- 获得 operator 身份或控制端的攻击者仍可能在允许范围内影响服务；动作受限不代表没有风险。
- 主机 helper 的代码和参数验证本身必须经过审查，部署 root-owned 文件不等于实现安全。
- 外部探针的检查只是从特定网络位置验证，不代表所有用户和路径都正常。
- PostgreSQL 中的审计记录不是不可篡改账本。第一版可导出摘要清单到独立存储，但不声称控制端管理员无法改写历史。
- evidence bundle 内部 SHA-256 只检测 archive 内部不一致；攻击者可同时替换内容与 manifest。没有独立签名、可信时间戳和外部保管就不能声称来源真实性。
- Docker CLI 代码只执行 list/inspect 不会降低 daemon socket 本身的 root-equivalent 权限；失陷的 diagnostic container 仍可能直接调用 socket，因此它不进入常驻服务边界。
- sysctl、file、sshd、systemd、package 和 socket 观测均来自被测主机；已控制 root 的攻击者能伪造这些本地证据。
- 密文入 Git 不解决运行时泄漏，SOPS 不能替代操作权限与密钥轮换。
- 关联异常认证不等于账户失陷；没有事件不等于没有攻击。
- 回滚恢复前态不保证前态安全。修复失败且前态仍异常时，finding 保持打开。
- 恢复 VM 必须隔离出站副作用，否则恢复真实配置可能发送生产通知或写回生产系统。

## 变更原则

新增写动作必须同时提交：威胁分析、参数约束、前置条件、状态恢复、验证方式、回滚失败处理、正常测试和负例测试。只增加一个成功路径脚本不能进入允许动作集合。
