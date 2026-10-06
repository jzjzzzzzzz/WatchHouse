# Watchhouse documentation

本目录记录 Watchhouse 的设计依据、稳定契约、运维步骤、实施状态和验证证据。文档描述的是当前仓库状态；规划中的能力会明确标记，不应被理解为已经交付。

## 建议阅读顺序

1. [MVP 与验收标准](mvp.md)
2. [系统架构](architecture.md)
3. [威胁模型](threat-model.md)
4. [开发进度](progress.md)
5. [工程证据](evidence.md)

## 文档分类

| 类别 | 内容 |
| --- | --- |
| [`contracts/`](contracts/) | 数据模型、接口语义、安全约束和失败行为 |
| [`runbooks/`](runbooks/) | 可执行操作步骤、预期结果、限制和故障排查 |
| [`adr/`](adr/) | 已接受的架构决策及其理由 |
| [MVP](mvp.md) | 第一版的范围、场景和验收目标 |
| [Milestones](milestones.md) | 分阶段交付物、测试和完成条件 |
| [Repository](repository.md) | 源码、测试、部署和证据目录约定 |
| [Release checklist](release-checklist.md) | 发布前必须满足的工程门槛 |
| [Requirements](requirements.md) | 仓库开发与提交规则 |

## 证据规则

- 合成 fixture、交叉编译和容器测试只证明其明确覆盖的行为。
- 真实宿主集成声明必须链接到可复现的环境说明、命令、摘要和边界。
- 原始生产遥测、私钥、备份和 VM 状态不得提交到 Git。
- 测试运行记录位于 [`../evidence/test-runs/`](../evidence/test-runs/)；实现状态以[开发进度](progress.md)为准。

