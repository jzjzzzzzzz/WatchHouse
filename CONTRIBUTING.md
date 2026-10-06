# Contributing to Watchhouse

感谢参与 Watchhouse。项目以安全边界清晰、行为可验证和文档与实现一致为首要原则。

## 开发流程

1. 阅读 [`docs/requirements.md`](docs/requirements.md)、[`docs/architecture.md`](docs/architecture.md) 和相关 contract。
2. 将改动限制为一个可解释、可测试的 vertical slice。
3. 先明确输入、输出、资源上限、权限要求和失败行为，再修改实现。
4. 为正常路径、错误输入、权限不足和边界条件添加测试。
5. 更新受影响的 contract、runbook 和 [`docs/progress.md`](docs/progress.md)。
6. 提交前检查 diff，确认没有凭证、原始生产遥测、备份或构建产物。

## 本地验证

```sh
make test
make vet
```

按改动范围执行额外验证：

```sh
make linux                      # Linux amd64/arm64 编译
make smoke                      # 受限容器 smoke test
make crash                      # SQLite spool SIGKILL 恢复
make docker-ports-integration   # Docker binding 集成测试
```

不得把 fixture、交叉编译或容器测试描述为真实宿主机验收。需要 Linux、systemd、OpenSSH、Docker 或 PostgreSQL 的声明，必须记录实际运行环境和未覆盖边界。

## 代码约定

- Go 代码必须通过 `gofmt`、`go vet` 和 race tests。
- 外部输入必须有明确的大小、数量或时间限制，并采用严格解析。
- 系统命令使用固定 executable 和 argv；不要拼接 shell 命令。
- 权限或观测不足应显式返回，不得静默推断安全结论。
- 持久化和网络协议变更必须保持崩溃恢复、幂等性和身份绑定语义。
- 不为尚未实现的功能添加占位声明或成功状态。

## Pull request 要求

PR 描述应包含变更范围、安全和数据迁移影响、测试环境、已知限制，以及相关 contract、runbook 或 evidence 链接。

不要提交真实密钥、证书私钥、生产日志、数据库备份、VM 镜像、toolchain、`bin/` 或 coverage 输出。

