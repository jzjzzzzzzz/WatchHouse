# Private agent state

Agent 状态目录要求当前进程用户拥有，权限为 0700；数据库文件为普通文件、0600，不接受最终路径 symlink。权限错误时拒绝，不自动 chmod 现有目录或文件。

调用者明确选择状态目录；父目录必须已经存在，父级 symlink 解析为实际路径，以兼容 Mac 的 /var 临时目录路径。私有目录隔离数据库 WAL/SHM sidecar；同用户和 root 恶意改路径仍属于信任边界之外，不能声称完全免疫 filesystem TOCTOU。

agent 不自动建多层系统目录，不使用全局 os.Chmod 改权限，不读取任意状态文件名。部署后由 Ansible/systemd 创建指定私有位置。
