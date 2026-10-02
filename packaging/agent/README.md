# PierOps M2 Linux Agent 开发包

这是开发阶段构建，尚未在真实 VPS 上运行验收。包内没有真实配置或凭据，解压不会自动安装服务或修改系统。

## 文件

- `pierops-agent`：对应 Linux 架构的静态构建。
- `BUILD.json`：源码提交、架构和构建信息。
- `agent.monitoring.example.json`：监控模式，无运维能力。
- `agent.readonly.example.json`：只读文件模式，需设置允许目录。
- `pierops-agent.service.example`：以普通用户运行的 systemd 示例，默认限制写入。
- `AGENT-POLICY.md`：能力、目录、票据、升级和边界说明。
- `LICENSE`：Agent 的 MIT 许可。

## 手动使用

1. 确认架构：x86_64 对应 amd64，aarch64 对应 arm64；先升级 Hub 到同一 M2 源码阶段。
2. 对照同批 `SHA256SUMS` 校验压缩包，解压到独立目录。
3. 复制示例到节点本机的私有配置文件；设置 Hub 地址、节点 token、Hub 中的节点 UUID。限制配置文件读取权限，不提交 Git。
4. 先选择监控模式；只读文件模式需要事先创建允许目录，并让运行 Agent 的用户拥有读取权限。不要将含令牌/私钥的目录开放为允许目录。
5. 在该普通用户下手动启动：

```sh
./pierops-agent --config /absolute/path/to/agent.json
```

## systemd

服务示例不是自动安装脚本。节点管理员需自行创建 `pierops` 用户、安装二进制、配置目录和服务，再启动服务。示例仅适用于监控或只读文件；`ProtectHome=true` 会隐藏用户主目录，`ProtectSystem=strict` 限制文件写入。如要开放终端或命令，必须配置非 root 的 `execution_user`，并按需要审查主目录与服务的访问限制。

M2 拒绝上游自动更新/远程版本替换。升级由管理员替换为审查过的 PierOps 构建并重启；重启前签发的票据不能重放。运行中的任意命令不受文件管理允许目录限制，而受执行用户的系统权限约束。
