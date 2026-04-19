# Ubuntu systemd integration lab

使用独立 QEMU TCG guest，而不是 privileged systemd container。tools 容器只运行 userspace emulator；VM 内拥有独立 Linux kernel、systemd 和 OpenSSH。VM 结果不等于公网 VPS 部署。

Canonical Ubuntu 24.04 ARM64 image 固定到 20260926 build 与 SHA-256；tools base 固定 OCI digest。APT package 版本在实际 build 后记录，不声称仅固定 base digest 就锁定后续仓库内容。下载的 SHA 与固定 manifest 比较，未通过不得启动。普通 checksum 不是 publisher signature 或供应链 attestation。

所有 image、overlay、cloud-init seed、生成私钥和运行状态在 lab/local，Git 忽略。SSH 仅发布到 127.0.0.1 的动态端口，已知 host key 由 bootstrap 时生成并写入 seed，不关闭 host key verification。

当前切片只提供 image 校验和 tools build 定义，VM lifecycle 尚未完成。

官方来源：[Ubuntu image directory](https://cloud-images.ubuntu.com/noble/20260926/)、[QEMU virt platform](https://www.qemu.org/docs/master/system/arm/virt.html)、[cloud-init SSH](https://docs.cloud-init.io/en/latest/reference/modules.html#ssh)。
