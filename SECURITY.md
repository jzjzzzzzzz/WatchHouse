# Security policy

## Supported versions

Watchhouse 目前处于发布前开发阶段，尚无受支持的稳定版本。安全修复仅应用于默认分支的最新代码。

## Reporting a vulnerability

请使用 GitHub 仓库的 **Security → Report a vulnerability** 私密报告入口提交安全问题。不要创建公开 issue，也不要在报告中包含不必要的生产数据。

报告应尽可能包含：

- 受影响的 commit 和组件；
- 前置权限与信任边界；
- 最小复现步骤或测试；
- 对机密性、完整性、可用性或审计证据的影响；
- 已知缓解措施。

维护者确认问题后会在私密线程中协调复现、修复和披露。由于项目尚未发布，目前不承诺固定响应时限。

## Scope

重点关注 mTLS 身份绑定、授权或审计绕过，receipt/cursor/spool 数据丢失，非预期提权或越界文件访问，资源边界绕过，以及 evidence bundle 验证绕过。

