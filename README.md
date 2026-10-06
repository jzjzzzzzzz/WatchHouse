# Watchhouse

[![CI](https://github.com/jzjzzzzzzz/WatchHouse/actions/workflows/test.yml/badge.svg)](https://github.com/jzjzzzzzzz/WatchHouse/actions/workflows/test.yml)
[![Go](https://img.shields.io/badge/Go-1.27.1-00ADD8?logo=go&logoColor=white)](go.mod)
[![Platform](https://img.shields.io/badge/platform-Linux-1f6feb?logo=linux&logoColor=white)](#运行边界)
[![Status](https://img.shields.io/badge/status-active_development-d97706)](#项目状态)

Watchhouse 是面向小规模 Linux 与 VPS 环境的安全事件调查和服务恢复系统。它将主机证据采集、事件检测、可靠传输、人工授权查询和外部服务验证组织为一条可审计链路。

项目优先保证**证据完整性、权限最小化、故障可恢复性和结论边界**，而不是提供通用服务器管理面板或完整 SIEM。

## 项目状态

> **项目状态：活跃开发中。** 当前版本已具备可测试的只读观测和控制面能力，但尚未达到生产发布标准，也不执行服务器写操作。详细进度见[开发状态](docs/progress.md)。

## 设计目标

Watchhouse 试图可靠回答三个问题：

1. **发生了什么？** 将有界的系统证据规范化为可追溯事件和 finding。
2. **证据是否可靠送达？** 通过本地持久化队列、mTLS、幂等提交和精确 receipt 处理中断与重试。
3. **服务是否真正恢复？** 分离主机内部状态、容器端口绑定和外部 HTTPS 探测，避免把单一观测误当作端到端可用性。

```text
Linux host                          Control plane                 Probe node
┌──────────────────────┐           ┌──────────────────────┐      ┌──────────────┐
│ journal / listeners  │  mTLS    │ ingest + PostgreSQL  │      │ DNS/TCP/TLS │
│ posture / file audit ├──────────►│ detection + findings │◄─────┤ HTTP probe   │
│ SQLite durable spool │ receipts │ authorized queries   │ mTLS │ durable spool│
└──────────────────────┘           └──────────────────────┘      └──────────────┘
```

完整的组件、信任边界和失败语义见[架构文档](docs/architecture.md)与[威胁模型](docs/threat-model.md)。

## 已实现能力

| 领域 | 当前能力 | 关键边界 |
| --- | --- | --- |
| 事件采集 | SSH journal 解析、snapshot、经 cursor 核验的增量采集 | Linux journal 权限决定可见范围 |
| 本地可靠性 | SQLite WAL outbox、容量 backpressure、CAS checkpoint、精确 Ack | 尚无物理磁盘硬上限 |
| 控制面 | TLS 1.3 mTLS ingest、PostgreSQL 幂等存储、持久化 finding | 规则处理仍采用每 host 有界重扫 |
| 查询授权 | 独立 human certificate、角色映射、append-only 授权审计 | 尚无外部不可变审计导出 |
| 主机证据 | listener、nftables、sysctl、文件、sshd、systemd、磁盘、证书、dpkg 审计 | 均为只读检查，不等同于完整合规扫描 |
| 网络证据 | Docker port binding 关联、HTTPS DNS/TCP/TLS/HTTP 探测 | 本地或 loopback 探测不代表公网可达性 |
| 事件取证 | 确定性 evidence bundle 及内部 digest 校验 | 未提供外部签名或加密 |
| 恢复验证 | PostgreSQL custom-format dump 与隔离数据库恢复核对 | 尚未证明 off-host、跨实例或 PITR 恢复 |

## 快速开始

### 环境要求

- Go 1.27.1
- Linux（使用 journal、network namespace 和主机审计能力时）
- PostgreSQL（运行控制面或集成测试时）
- Docker（仅用于可选 smoke/integration 验证）

### 构建与测试

```sh
git clone https://github.com/jzjzzzzzzz/WatchHouse.git
cd WatchHouse

make test
make vet
make build
```

构建产物写入 `bin/`，不会进入版本控制。可使用 `make linux` 生成 Linux amd64/arm64 二进制；`make smoke` 和 `make crash` 分别验证受限容器运行与队列突杀恢复。

### 重放示例

以下命令使用仓库内的合成 journal fixture，不需要主机权限：

```sh
./bin/watchhouse replay \
  --host lab-1 \
  --input tests/fixtures/ssh-sequence.journal.jsonl
```

预期输出为 7 条规范化事件和 1 条带 evidence references 的 SSH finding。输出契约和故障语义见[重放 runbook](docs/runbooks/replay.md)。

### Linux 只读采集

```sh
./bin/watchhouse snapshot --host vps-staging --limit 200
./bin/watchhouse listeners
./bin/watchhouse firewall
./bin/watchhouse audit-ssh
```

这些命令不会修改系统。持续采集、交付、控制面、探针和证据包操作请从[运维手册索引](docs/runbooks/README.md)进入。

## 命令概览

| 类别 | 命令 |
| --- | --- |
| 事件 | `replay`, `snapshot`, `collect`, `spool` |
| 交付 | `deliver`, `report-listeners`, `deliver-listeners`, `report-probe`, `deliver-probes` |
| 查询 | `query-events`, `query-findings` |
| 主机检查 | `self`, `listeners`, `firewall`, `posture`, `audit-files`, `audit-ssh`, `audit-units`, `audit-disk`, `audit-cert`, `packages` |
| 网络检查 | `docker-ports`, `exposure`, `listener-check`, `probe-https` |
| 取证 | `evidence-bundle create`, `evidence-bundle verify` |

运行 `./bin/watchhouse --help` 查看参数。控制面由 `watchhouse-control` 提供；部署契约见[控制面 runbook](docs/runbooks/control.md)。

## 运行边界

- 当前 agent 仅执行只读采集和审计；MVP 中的审批、修复与回滚协议尚未实现。
- fixture、交叉编译和容器 smoke test 不作为真实宿主机集成证明。
- 仓库保存经过筛选的测试证据，不保存凭证、原始生产遥测、备份、toolchain 或构建产物。
- 已完成的 Ubuntu QEMU/systemd、Docker Desktop 和 PostgreSQL 验收范围记录在 [`evidence/test-runs`](evidence/test-runs/)；这些结果不代表公网 VPS 或长期生产运行。
- 性能、RTO 与 RPO 数值目前是验收目标，而不是已完成的服务承诺。

## 文档

| 文档 | 说明 |
| --- | --- |
| [Documentation index](docs/README.md) | 文档导航与阅读路径 |
| [MVP scope](docs/mvp.md) | 第一版范围和验收标准 |
| [Architecture](docs/architecture.md) | 组件、数据流和失败处理 |
| [Threat model](docs/threat-model.md) | 资产、信任边界、攻击面与控制措施 |
| [Contracts](docs/contracts/) | 事件、传输、查询和主机证据契约 |
| [Runbooks](docs/runbooks/) | 构建、运行、故障排查和验证步骤 |
| [Milestones](docs/milestones.md) | 分阶段 deliverable 与完成条件 |
| [Progress](docs/progress.md) | 已实现能力、验证证据和未完成项 |
| [Evidence](docs/evidence.md) | 可复现工程证据及演示路径 |

## 参与贡献

提交变更前请阅读 [CONTRIBUTING.md](CONTRIBUTING.md)。项目要求小而可验证的 vertical slice；代码变更应包含测试，文档必须准确说明尚未实现的边界。

安全问题请按 [SECURITY.md](SECURITY.md) 中的流程报告，不要在公开 issue 中提交凭证、生产日志或未脱敏遥测。
