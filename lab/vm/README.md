# Ubuntu systemd integration lab

使用独立 QEMU TCG guest，而不是 privileged systemd container。tools 容器只运行 userspace emulator；VM 内拥有独立 Linux kernel、systemd 和 OpenSSH。VM 结果不等于公网 VPS 部署。

Canonical Ubuntu 24.04 ARM64 image 固定到 20260926 build 与 SHA-256；tools base 固定 OCI digest。APT package 版本在实际 build 后记录，不声称仅固定 base digest 就锁定后续仓库内容。下载的 SHA 与固定 manifest 比较，未通过不得启动。普通 checksum 不是 publisher signature 或供应链 attestation。

所有 image、overlay、cloud-init seed、生成私钥和运行状态在 lab/local，Git 忽略。SSH 仅发布到 127.0.0.1 的动态端口，已知 host key 由 bootstrap 时生成并写入 seed，不关闭 host key verification。

提供 prepare/start/status/probe/stop/restart lifecycle。restart 只接受已核验 label 且实际停止的 container，保留 overlay 与之前 runtime 状态；probe 超时不自动重启。

virtio-net PCI option ROM 明确禁用：guest 从本地磁盘启动，不需要 PXE/EFI network boot ROM。最小 tools image 不安装 ipxe-qemu，初次启动暴露了该缺失依赖，修复后需重新验证启动。

## 运行

```sh
python3 scripts/vm_image.py
docker build -f lab/vm/tools.Dockerfile -t watchhouse-vm-tools:24.04 lab/vm
python3 scripts/lab_vm.py prepare
python3 scripts/lab_vm.py start
python3 scripts/lab_vm.py probe
python3 scripts/systemd-integration.py --reboot
```

TCG 是软件仿真，guest boot/readiness 与生产 VPS 延迟不能混为一谈。一次 240 秒 reboot readiness 预算在实际运行中超时，container 仍在运行，没有自动重启。改为显式可配置的 30 到 1200 秒 lab readiness 预算，默认 600 秒；应用内 journalctl 十秒 timeout 不变。

QEMU tools container 的内存上限为 3 GiB，guest 本身仍为 1 GiB。实际 reboot 时 TCG/QEMU 开销曾超过 1.8 GiB；2 GiB container 上限会造成间歇性 SSH banner 和文件传输停顿但不设置 OOM 标记。SSH 建连预算因此为 20 秒，probe 的总预算为 45 秒；这些是软件仿真实验设施预算，不放宽 collector 的应用期限，也不表示 guest 获得 3 GiB 内存。

integration failure 记录独立写入 lab/local 中，不覆盖旧成功结果，也不把旧报告当新运行的证据。发布报告必须检查 runner digest、build commit、实际时间和 reboot 项；不只检查 passed 字段。

官方来源：[Ubuntu image directory](https://cloud-images.ubuntu.com/noble/20260926/)、[QEMU virt platform](https://www.qemu.org/docs/master/system/arm/virt.html)、[cloud-init SSH](https://docs.cloud-init.io/en/latest/reference/modules.html#ssh)。
