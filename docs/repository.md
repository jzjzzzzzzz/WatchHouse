# 仓库结构与实现约束

## 当前目录

当前只创建 README 和设计文档。下面是实现阶段的目标结构，不是已经存在的源码。

```text
watchhouse/
  README.md
  cmd/
    agent/
    helper/
    server/
    watchhouse/
  internal/
    telemetry/
    inventory/
    correlation/
    detection/
    plans/
    authorization/
    executor/
    verification/
    recovery/
    storage/
  api/
    openapi.yaml
    schemas/
  policies/
    services/
    detections/
  deploy/
    ansible/
    systemd/
    nginx/
    compose/
    secrets.example/
  tests/
    unit/
    integration/
    scenarios/
    fixtures/
    fault-injection/
  lab/
    topology/
    staging/
    restore/
  scripts/
  docs/
    landscape.md
    decision.md
    mvp.md
    architecture.md
    threat-model.md
    milestones.md
    evidence.md
    repository.md
    contracts/
    adr/
    runbooks/
    postmortems/
  evidence/
    test-runs/
    restore-drills/
    demo/
  .github/
    workflows/
  SECURITY.md
  CONTRIBUTING.md
  LICENSE
```

Go 的同包单元测试可以直接放在相关源码旁；tests 下集中存跨组件测试、场景和数据，不为目录整齐打破语言习惯。

## 每个核心目录的责任

- telemetry 只负责读取、归一化和采集质量，不决定执行动作。
- detection 生成 finding，不直接调用 helper。
- plans 固定动作内容、证据、前置条件与验证契约。
- authorization 处理身份与审批权限，不能由 agent 自行批准。
- executor 维护资源锁和动作 journal，不接收任意 shell。
- verification 区分内部状态和外部网络视角。
- recovery 负责隔离恢复和结果判断，不假设备份 exit code 就代表可恢复。
- deploy 保存可重现配置，不保存实际私钥和解密秘密。
- evidence 保存脱敏结果，不提交生产日志和真实备份。

## 开发顺序

下一份文档是 docs/contracts 下的事件契约、响应状态表和场景目录。通过 M0 后才开始 agent 的只读采集。没有可靠归属和可观察失败状态，不进入写动作开发。

所有扩展都应回答：真实需求是什么？现有工具是否已提供？自己新增的核心是什么？失败后会怎样？如何证明结果？

第一版保持一个仓库和少量部署进程。不先建插件市场，不引入消息总线，不优先做 dashboard。
