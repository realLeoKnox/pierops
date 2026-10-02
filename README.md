# 栈桥 · PierOps

Agent 主动连接 Hub 的轻量可控运维平台。当前已建立独立构建与 Docker 运行基线，正在推进 M1：Hub 的账户角色、节点动作授权与权限审计。

## 目录

```text
pierops/
├── hub/                  Hub/API/状态存储/终端与文件转发
├── agent/                节点采集与操作执行
├── frontend/             构建时获取的固定版本 Web UI（本地目录，不入公开 Git）
├── docs/                 架构、来源与实施记录
├── scripts/              本地构建及运行入口
├── Makefile              统一构建/测试
└── go.work               双 Go 模块工作区
```

Hub 与 Agent 暂时保留上游 Go 模块名和主要接口，避免架构基线阶段引入大规模改名。来源见 [来源记录](docs/UPSTREAM.md)。`hub/` 和 `agent/` 是原仓库的独立快照；Web UI 在构建时按固定 commit 获取，不纳入公开 Git 历史。

每个开发步骤的改动、验证和未完成事项记录在 [开发日志](docs/DEVLOG.md)；项目级记录规则见 [AGENTS.md](AGENTS.md)。

后续实施顺序、阶段产物和验收条件见 [开发路线](docs/ROADMAP.md)。

## 本地构建

需要 Go 1.25+、Node.js 22+、npm、zstd。首次运行 npm 会安装前端依赖。

```bash
make all
./scripts/dev-hub.sh
```

`make test` 运行协议、文件、终端和任务的本地测试。Agent 上游的外部 ping 测试需要真实外网和原始 ICMP socket，不包含在此命令中。

## Docker 运行 Hub

需要 Docker Engine 与 Compose。第一次构建会获取固定版本的上游 Web UI；Hub 镜像由多阶段构建生成，不依赖本机已安装的 Go 或 Node.js。

```bash
docker compose build hub
docker compose up -d hub
docker compose ps
```

默认将容器端口映射到主机 `127.0.0.1:25774`，数据保存在 Compose 命名卷 `pierops-data`。浏览器打开 `http://127.0.0.1:25774/install` 完成首次安装。需要给局域网或反向代理访问时，设置 `PIEROPS_BIND_HOST`、`PIEROPS_PORT` 后重新创建容器；公网访问应在反向代理处启用 HTTPS。Hub 容器不挂载 Docker socket。

隔离测试可使用独立 Compose 项目和端口，例如：

```bash
PIEROPS_PORT=25785 docker compose -p pierops-test up -d --build hub
PIEROPS_PORT=25785 docker compose -p pierops-test down
```

`down` 不带 `-v`，测试数据卷会保留，便于检查重启持久化。正式使用前先运行 `make check-public` 检查可达 Git 历史和当前跟踪文件中的常见凭据模式；该检查不能替代人工审阅。

本地脚本启动时，Hub 默认只监听 `127.0.0.1:25774`，数据位于 `hub/data/`；Compose 使用上文所述的数据卷。浏览器完成初始化后，在 Hub 内创建节点并获取 Agent token。在另一终端运行：

```bash
AGENT_TOKEN='从 Hub 创建的节点 token' ./scripts/dev-agent.sh
```

Agent 的上游自动更新默认关闭，避免把本项目二进制替换为原版 Agent。本地脚本还默认禁用 Web 控制能力；需要验证终端和文件流程时，显式设置 `AGENT_DISABLE_WEB_SSH=0`。需要让其他机器连接时，按部署环境设置 `PIEROPS_LISTEN` 和 `AGENT_ENDPOINT`，并使用 HTTPS/WSS。**Hub 授权核心已加入；Agent 本地约束、完整会话审计与真实部署验收仍在后续阶段。**

## 当前状态与下一步

- 已集成上游 Hub、Agent，并提供按固定版本获取 Web UI 的构建入口及 Hub Docker 运行入口。
- 已加入 M1 授权核心：账户角色、具体节点/动作授权、拒绝与策略变更审计。入口及边界见 [授权说明](docs/ACCESS.md)。
- owner 登录后打开 `/api/admin/access/ui`，可创建受限账户、分配节点动作和查看授权审计。
- 下一阶段：Agent 本地能力与目录约束、操作票据、Docker 只读适配器、完整会话审计。
- 开发时先阅读 [架构与边界](docs/ARCHITECTURE.md) 和 [阶段计划](docs/ROADMAP.md)。
