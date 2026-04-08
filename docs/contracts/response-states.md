# 响应状态契约 v1

本表是后续写动作的验收契约，当前尚未实现执行器。

| 当前状态 | 输入和条件 | 下一状态 |
| --- | --- | --- |
| detected | 固定允许动作、完整证据和目标版本 | planned |
| planned | operator 批准当前 plan digest | approved |
| planned | 身份无权限或计划内容变化 | rejected |
| approved | 有效期已过 | expired |
| approved | 本地前置状态摘要变化 | stale |
| approved | 本地 allowlist 校验、资源锁、前态和意图持久化成功 | executing |
| executing | 测试和 reload 成功 | verifying |
| executing | 发生写入或 reload 失败 | rolling_back |
| verifying | 内部及有效的外部确认都通过 | verified |
| verifying | 检查失败或超过本地确认期限 | rolling_back |
| rolling_back | 前态恢复并验证成功 | rolled_back |
| rolling_back | 恢复或验证失败 | rollback_failed |
| executing / verifying | 崩溃后不能确定资源状态 | result_unknown |

没有成功获取前态或持久化意图时不得修改资源，记录 execution_failed。result_unknown 不能直接重跑，必须重读状态后继续验证、恢复或要求人工调查。终态重放同 command_id 只能返回已有结果。

rolled_back 仅表示恢复操作前态，finding 不因此自动关闭。拒绝越权操作不消费合法计划的审批权。持久化失败、锁丢失和外部晚到确认分别需要独立测试。
