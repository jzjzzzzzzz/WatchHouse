# 阶段交付计划

阶段按完成标准推进，不按“看起来做了很多”推进。M0 到 M5 构成 MVP；后续扩展不能阻塞第一版闭环。

## M0 冻结验收契约

- Deliverable：事件字段、三类规则定义、唯一写动作协议、状态转换表、测试场景清单、固定实验拓扑。
- Test：逐项检查每条状态转换、每个权限和每种异常是否有通过/拒绝判据。
- Demo：手工走一遍配置故障案例，展示证据、plan、审批、前态、验证和回滚分别是什么。
- Completion criterion：MVP 文档中不存在“系统自动处理”等无明确输入输出的占位描述；每项验收都有场景编号和预期结果。

## M1 真实 Linux 只读观测

- Deliverable：原生 systemd agent，journal cursor、unit 状态、socket/process 和注册容器发布元数据采集；本地队列；采集质量状态。
- Test：进程快速退出、PID 复用、缺权限、日志轮转、agent 重启、磁盘满与容器 NAT 发布。
- Demo：启动未声明监听，展示端口、owner/unit 或 unknown；重启 agent 后继续读 journal。
- Completion criterion：不依赖控制端也能导出可解释的真实事件；所有不可可靠归属的情况不伪造归属；无写服务器能力。

## M2 控制面与检测

- Deliverable：Go API、mTLS 主机身份、PostgreSQL、事件去重、三类检测、服务上下文、外部探针、CLI 查询。
- Test：伪造身份、畸形事件、重复乱序、断网补发、正常 SSH 重试、阈值边界和探针分层失败。
- Demo：实际产生 SSH 失败与成功序列，查看原始 journal 证据；展示 DNS/TCP/TLS/HTTP 不同失败层。
- Completion criterion：三类 finding 可追溯，满足 MVP 检测与队列指标；正常 fixtures 与异常 fixtures 分别有结果报告。

## M3 受限响应与恢复状态

- Deliverable：plan、身份角色、审批、主机本地重验、helper、Nginx approved bundle、持久化动作 journal、验证和回滚。
- Test：越权、过期、重放、状态漂移、路径穿越、符号链接、每一步崩溃、外部确认丢失、回滚失败。
- Demo：viewer 被拒；operator 批准合法恢复；外部验证通过后提交；验证失败场景触发回滚。
- Completion criterion：不提供任意 shell；每个动作终态都有证据；重启和重投递不会盲目重复修改；所有写操作负例通过。

## M4 部署与数据库恢复

- Deliverable：Ansible 部署、secret 交付、TLS、网络策略、一致性导出、restic 集成、隔离恢复 runner 和恢复报告。
- Test：空白 VM 两次部署、外部端口验证、密钥文件权限、secret 泄漏、备份损坏、数据库导入和应用业务检查。
- Demo：恢复脱敏 staging 数据到新 VM，读取业务数据并显示实测 RPO/RTO。
- Completion criterion：第三方能按文档部署；恢复不会连接生产依赖；backup exit code 不是唯一成功判据。

## M5 第一版发布与完整演示

- Deliverable：版本化发布物、CI、安全文档、测试报告、四分钟 Demo、至少一份故障 postmortem。
- Test：固定拓扑全链路回归；CI 单元测试、parser fuzz、静态检查、依赖/secret 扫描；真实 VM 集成结果附版本清单。
- Demo：按 evidence 文档进行现场异常、调查、审批、恢复和验证。
- Completion criterion：mvp.md 验收全部满足；已知限制公开；README 没有未完成能力伪装成已交付。

## M6 连续自用运行

- Deliverable：至少 30 天运行记录、资源和存储消耗、检测延迟、误报处理、证书更新、恢复演练和故障记录。
- Test：定期停止 agent、断开控制端、恢复过期、测试数据保留与告警链路。
- Demo：展示真实事件历史和一个真实故障从发现到恢复的证据，而非只重放合成日志。
- Completion criterion：能解释一次真实失败及改进；记录观测盲区和误报；至少一次隔离恢复成功。

## 后续扩展顺序

1. 多主机与身份生命周期：逐主机授权、证书轮换/吊销、升级和兼容测试。避免升级工具本身成为通用远程执行器。
2. 运行时事件：通过 Falco 或 osquery 适配器增加进程和容器证据，不自行维护 eBPF driver。
3. 漏洞管理闭环：集成 Trivy，将结果关联具体 image digest、运行实例和网络暴露，做人工 triage 与验证过的发布；不声称静态依赖存在就必然可利用。
4. 更多恢复场景：备份凭证分权、PITR、持久卷、完整服务重建，分别增加一致性与副作用测试。
5. 更多受限动作：仅当有真实需求才增加特定 unit 隔离、有限规则修复等；每种动作独立定义边界与失败语义。
6. 可读 UI：在 CLI/API 与证据稳定后做事件调查页，不做通用管理 dashboard。
7. 加强证据保全：独立存储锚点、签名导出、长期保留与取证适配；先说明威胁模型再选择机制。
