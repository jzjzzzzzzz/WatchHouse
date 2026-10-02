# 系统架构

## 组件与部署

```text
管理员 CLI ── 独立身份 / TLS ── 控制端 API
                                  │
                                  ├─ 规则与服务上下文关联
                                  ├─ finding / plan / 审批状态
                                  ├─ PostgreSQL
                                  └─ 外部 DNS → TCP → TLS → HTTP 探针
                                           │ 公网入口
                                           ▼
受管 VPS                             Nginx → Docker 服务 → 应用数据库
  journald / systemd / socket / 配置          │
               │                            │
         只读 collector                      │
               │                            │
         本地持久化队列 ── mTLS 出站 ── 控制端
               │          拉取受限 command / 回传 result
          agent coordinator
               │ Unix socket / 本地身份检查
          privileged helper
               └─ 注册 Nginx 配置目录 / approved bundle / reload

备份导出物 → restic 远端仓库 → 隔离恢复 runner → DB 与业务验证报告
```

## 技术选择

核心 agent、helper、控制 API 和 CLI 统一用 Go。Python 仅用于后续集成测试和场景验证，Bash 仅用于小型诊断与演练脚本。不为覆盖语言 checklist 引入多语言微服务。

控制面使用 PostgreSQL 存事件索引、服务清单、finding、plan、审批与执行结果；agent 本地使用 SQLite 做队列和动作 journal。第一版不引入消息总线或搜索集群。

Ansible 负责安装、用户、权限、unit、Nginx 与 workload 配置；restic 负责备份存储；SOPS/age 负责 Git 中加密配置。提供 Prometheus 格式指标，不强制先部署 Grafana。

原生 systemd agent 是有意选择：必须面对真实宿主的 journal、D-Bus、权限和进程生命周期，不能只在 Docker 容器中假装管理 Linux。

## 观测权限分离

网络 coordinator 不以 root 运行。日志访问授予专用账号并记录其可读范围。需要特权的 socket/process 和容器元数据采集走独立、固定只读接口；没有权限时返回 partial/unknown。

Docker socket 按 root 等效权限对待，不能“只读挂载”后认为安全。普通网络进程不持有 Docker socket；特权采集路径只输出过滤后的注册容器元数据，不提供任意 Docker API 代理。

特权 helper 单独运行，固定目录、固定资源和固定动作。Unix socket 限制调用账号并验证 peer credentials。读写职责在接口和测试中分开；不把原始日志字段拼成 shell 命令。

## 事件与上下文

事件保存：event_id、host_id、boot_id、source、source_cursor/sequence、observed_at、received_at、schema_version、payload 和采集质量标记。

主机身份绑定证书。进程身份采用 host_id + boot_id + PID + start_time，防止 PID 复用误关联。journal cursor 用于断点续传；轮转或缺口生成观测故障记录。

宿主监听 socket 优先使用内核接口和 inode/PID 关联，再通过 cgroup 映射 unit。容器场景同时检查网络命名空间和端口发布元数据。Docker NAT 发布不一定对应一个宿主监听进程，因此不能强行做 socket → PID 归属。

第一版输出资源视图和事件时间线，不做通用 graph database。时间相关性不代表同一个攻击者或因果链；跨主机时钟漂移需要记录和显示。

## 数据流

1. collector 读取来源并写入本地队列，保留 cursor 与完整性状态。
2. coordinator 批量上报，控制端校验身份、schema、长度和速率。
3. 控制端以 host_id + event_id 去重并事务性写入。
4. 有界时间窗口规则结合服务清单生成 finding，保存规则版本和证据引用。
5. 外部探针补充 DNS/TCP/TLS/HTTP 结果；失败层次独立记录。
6. operator 对固定 plan 内容审批；审批记录绑定 plan digest 与期限。
7. agent 拉取 command，本地再次校验身份范围、资源状态、期限和 allowlist。
8. helper 记录前态与意图，再实施固定动作；结果回传。
9. 通过内部和外部检查后提交；失败或超时回滚；最后生成可导出的事件报告。

## 已实现的诊断平面

常驻 agent 只持有 journal、`/proc` 和私有 SQLite state 所需权限。nftables
ruleset、Docker daemon metadata 与其他管理员级观测保持为短生命周期本地
诊断，不因“只读代码路径”并入 service 权限。每个 collector 使用固定命令或
固定内核路径、有界输出、deadline 和 typed result；权限不足是显式 error 或
partial quality，而不是自动 sudo。

网络诊断保留四份独立事实：当前 namespace TCP listener、Docker host
binding、nftables lossy summary、指定 vantage 的 DNS/TCP/TLS/HTTP probe。
correlation 只记录兼容 endpoint，不产生“公网开放”结论。Docker kernel NAT
没有 userspace listener 是允许状态，反向代理和 tunnel 也可能使 loopback
listener 对外可达。

PostgreSQL logical restore runner、host audit JSON 和 integration result 可以进入
deterministic evidence bundle。bundle manifest 提供内部长度/digest 对账，外部
签名、加密、保留和可信时间戳仍属于部署系统。该导出路径不改变控制面数据，
也不把本地主机生成的证据提升为 remote attestation。

## 计划与动作契约

plan 固定包含：plan_id、host_id、resource_id、action_type、typed parameters、expected state digest、target bundle digest、policy version、evidence references、expiry、verification contract 和 rollback reference。

审批绑定完整 plan，而不是“允许修复这台主机”。修改参数后审批失效。执行前检查前置状态，避免审批之后环境变化造成错误修复。

command_id 是幂等标识，不要求网络 exactly-once。消息允许重复传输，效果靠持久化 journal、资源锁和状态重读确保不会盲目重复应用。

第一版采用显式允许动作，不创建通用 workflow DSL。不接收 command string、任意路径、任意 unit 名称、任意 Docker 操作或上传脚本。

## 状态转换与崩溃恢复

正常路径：detected → planned → approved → executing → verifying → verified。

终止和异常状态：rejected、expired、stale、execution_failed、rolling_back、rolled_back、rollback_failed、result_unknown。

动作前必须持久化前态、目标和意图。agent 重启遇到 executing 状态时，先读取当前配置、Nginx 状态和 journal，再决定继续验证或回滚，不能直接重跑。

外部验证的确认携带 command_id、检查结果和有效期。管理节点失联时，主机不能永久等待；超过本地验证期限自动尝试回滚。第一版明确接受“控制面故障可能使正确修复回滚”的保守取舍。

外部确认和状态不是全系统原子事务。文档和测试必须覆盖确认丢失、确认晚到、本地先回滚、回传重复等竞争条件。

## 网络与秘密

workload 的数据库不发布公网端口。SSH 使用 key，限制管理来源；Nginx 暴露业务入口。Docker 防火墙 backend 锁定并记录，主机 INPUT 与容器 FORWARD/NAT 规则分别验证，不混用未经测试的规则路径。

agent 只主动连接管理节点，不新增主机公网管理端口。控制 API 的 mTLS 验证和身份授权保留在应用端，不能只相信代理传来的任意身份 header。

秘密在部署时解密到受限位置，以 systemd credentials 或等效受限文件交给进程，不写入 Git、命令行、事件 payload 或报告。备份写入、恢复读取和仓库管理权限尽量分开；age 私钥不与密文一起提交。
