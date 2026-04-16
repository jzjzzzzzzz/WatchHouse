# Durable journal ingestion

WholeJournalFile 模式用于完整 JSONL 文件：如果已有 checkpoint，先扫描并找到相同 cursor，跳过它及之前记录，再落盘之后记录。找不到 cursor 时返回 source gap，不添加任何新事件。文件必须保留 checkpoint，不能随意用 tail 结果覆盖完整文件。

VerifiedAfterCheckpoint 仅提供给已验证 native resume 的 collector；它不自行证明上游 cursor 存在。不能把任意文件以这种模式输入后宣称缺口检测通过。

Native collector 必须使用 RunVerifiedAfter 并传入 Poll 使用的原始 cursor；捕获期间其他 collector 更新 checkpoint 时，旧 batch 拒绝，不得以最新 checkpoint 给旧 batch 重新锚定。后续每条记录的 CAS 继续保护并发变化。

每条记录先验证 boot、cursor、微秒时间，再规范化；匹配事件和 cursor 原子落盘，明确 unmatched 记录只推进 cursor，malformed 记录停止。记录级事务意味着此前成功的记录保留，后续重试从 checkpoint 继续，不是整文件 all-or-nothing。

容量满立即停止，不跳到后面的 unmatched 记录，不丢弃当前记录；Ack 释放容量后重新输入完整文件即可继续。完整输入处理与待上报事件持久化有独立计数，Complete 仅指本次输入，不代表远端已接收。
