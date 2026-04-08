# GitHub landscape

## 调研范围与口径

调研日期：2026-10-05。实际检索了 GitHub 仓库、官方 README 和相关官方文档，并通过 GitHub REST API 读取 stars、主要语言、pushed_at 和 archived。

表内 stars 是当日 API 快照；最近 push 只是维护活动信号，不等于最新正式发布，也不能证明维护质量。技术栈描述结合项目文档，不把 GitHub 的主要语言当作完整技术栈。

“剩余问题”指 Watchhouse 关心、但该项目不以完整交付为主要目标的问题，不表示现有项目无法通过扩展实现。以下差异判断是本项目的设计判断，而不是已证实的市场空白。

## 开源项目比较

| Repository | Stars / 最近 push | 解决的问题与技术栈 | 已覆盖的能力 | 与 Watchhouse 的重叠及剩余问题 |
| --- | --- | --- | --- | --- |
| [wazuh/wazuh](https://github.com/wazuh/wazuh) | 17,117 / 10-05 | SIEM/XDR；C/C++、Python、agent、indexer | 日志、FIM、配置检查、漏洞、容器监测、主动响应 | 高重叠。不能重写“采集日志再封 IP”；重点改为服务上下文、受限变更与修复后的业务验证 |
| [fleetdm/fleet](https://github.com/fleetdm/fleet) | 6,949 / 10-05 | 设备管理；Go、osquery、Web/API、GitOps | 资产盘点、策略、查询、脚本、策略自动修复 | 高重叠。声明式策略和脚本自动修复已存在；本项目聚焦实时事件调查、受约束响应及恢复证据 |
| [cockpit-project/cockpit](https://github.com/cockpit-project/cockpit) | 15,181 / 10-05 | 服务器管理 UI；Python、JavaScript/TypeScript、系统接口 | systemd、journal、网络、存储、SSH、多主机管理 | 高重叠。不要做通用运维面板；只呈现 finding 的证据、动作和验证状态 |
| [falcosecurity/falco](https://github.com/falcosecurity/falco) | 9,453 / 10-05 | 运行时检测；C++、内核事件、eBPF、规则 | 进程行为、容器上下文、实时告警 | 中重叠。复用未来的运行时事件，不重写内核探针；业务恢复不是其主要目标 |
| [crowdsecurity/crowdsec](https://github.com/crowdsecurity/crowdsec) | 15,076 / 10-05 | 日志与 HTTP 行为检测、防护；Go、规则、remediation components | SSH/Web 检测、IP 决策、WAF、阻断集成 | 中重叠。恶意 IP 检测和封禁已成熟；不将封 IP 当核心，关注证据和服务约束 |
| [aquasecurity/trivy](https://github.com/aquasecurity/trivy) | 38,247 / 10-02 | 漏洞、配置、secret、SBOM 扫描；Go | 容器/文件系统/仓库检查、CI 集成 | 中重叠。复用扫描结果；自己关联运行资产、暴露面和修复验证，不维护 CVE 数据库 |
| [osquery/osquery](https://github.com/osquery/osquery) | 23,613 / 10-02 | SQL 式系统观测；C++、SQLite 接口 | 进程、socket、文件、系统状态与查询 | 中重叠。可作后续只读适配器；不重建通用系统查询语言 |
| [Velocidex/velociraptor](https://github.com/Velocidex/velociraptor) | 4,297 / 10-05 | 终端调查与取证采集；Go、VQL、client/server | host artifact、远程采集、调查、triage | 中重叠。复用深入取证，第一版不做全功能 DFIR；主要交付服务修复与恢复验证 |
| [StackStorm/st2](https://github.com/StackStorm/st2) | 6,541 / 10-05 | 事件驱动运维自动化；Python、规则、workflow、集成包 | 事件响应、runbook、部署、审批询问、ChatOps | 高重叠。告警触发工作流与审批已存在；不做通用 SOAR，仅实现少量类型化动作 |
| [ansible/ansible](https://github.com/ansible/ansible) | 70,862 / 10-05 | 配置和部署自动化；Python、YAML、SSH | Linux 配置、包、用户、服务、部署、幂等 | 中重叠。复用安装和配置；持续事件关联与响应协议属于自己的核心 |
| [dev-sec/ansible-collection-hardening](https://github.com/dev-sec/ansible-collection-hardening) | 5,493 / 10-05 | Linux、SSH、Nginx、MySQL 加固；Ansible、Jinja | 安全基线、系统与服务加固 | 中重叠。学习并选择性集成基线；不能把现成 role 部署结果包装为自己的核心工程 |
| [prometheus/prometheus](https://github.com/prometheus/prometheus) | 66,379 / 10-05 | 指标采集和时序数据库；Go、PromQL | 可用性、资源监控、告警、系统自身可观测性 | 低到中重叠。复用指标能力；指标告警不能替代事件证据和恢复事务 |
| [restic/restic](https://github.com/restic/restic) | 36,430 / 10-01 | 加密、去重、快照备份与恢复；Go | 备份、恢复、仓库检查、远端存储 | 中重叠。直接复用存储引擎；自己完成数据库一致性导出、隔离恢复和业务验证 |
| [getsops/sops](https://github.com/getsops/sops) | 23,303 / 10-05 | 加密配置和 secrets 管理；Go、age/KMS 等 | Git 中加密存储、密钥集成 | 低重叠。复用加密；自己设计解密位置、运行时权限、轮换和泄漏测试 |

上述仓库在 API 快照中均未归档。第三方依赖的许可证和固定版本需要在选定集成时逐项确认。

## 不能忽略的相近设计

[Hanalyx/kensa](https://github.com/Hanalyx/kensa) 的 README 已描述 Linux 合规修复的 Capture → Apply → Validate → Commit/Rollback、持久化 journal、签名证据和特权 systemd helper。当日 API 为 1 star、最近 push 为 2026-09-28；它处于 pre-1.0。README 标明 BSL-1.1，并计划在 2029-01-01 转 Apache-2.0，当前按 source-available 处理，不计入上面的 14 个开源项目。

这直接否定了“带证据的事务式修复是新发明”的说法。Watchhouse 的价值必须落在连续运行的安全事件到服务恢复链路、网络侧验证和数据库恢复演练，而不是复制通用合规修复引擎。对该项目的能力描述来自其公开文档，没有运行验证它的全部承诺。

## 已有能力的官方核查

- Wazuh 已支持主动响应，包括有时限的 stateful response：[Incident response](https://documentation.wazuh.com/current/getting-started/use-cases/incident-response.html)。
- Fleet 已支持策略失败触发脚本：[Automatically run scripts](https://fleetdm.com/guides/policy-automation-run-script)。不同功能涉及 Free/Premium 边界，不能把所有功能都视为免费开源能力。
- StackStorm 已支持工作流中等待审批和限制答复身份：[Inquiries](https://docs.stackstorm.com/inquiries.html)。
- Falco 的官方安全模型明确它负责检测与通知，而不是直接阻断或隔离：[Security policy](https://github.com/falcosecurity/falco/security)。
- Docker 的端口发布涉及 NAT 和 FORWARD 路径，可能绕过 UFW 的常规过滤：[Packet filtering and firewalls](https://docs.docker.com/engine/network/packet-filtering-firewalls/)。因此“防火墙显示 deny”不是公网不可达的充分证据。

## 结论

日志采集、主机面板、漏洞扫描、加固 role、工作流和备份引擎都已有成熟方案。简单把它们装在一起，没有足够独立价值。

值得自己做的是一个窄而完整的工程系统：将服务预期状态、实时事件、当前系统上下文、审批权限、受限执行和恢复结果连接起来，并公开正常与失败场景的测试证据。复用成熟工具不降低含金量；没有自己的核心数据模型、关联逻辑、执行边界和失败处理才会降低含金量。
