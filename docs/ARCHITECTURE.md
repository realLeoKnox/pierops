# PierOps 最小架构

## 核心原则

Agent 主动通过 HTTPS/WSS 连接 Hub；Hub 不向节点发起入站连接。Hub 负责人的身份、节点选择和权限决策；Agent 负责本地能力开关与操作执行。两侧均不得把“已连接”当作“已授权”。

```text
浏览器 ── HTTPS ── Hub（API、策略、审计、会话转发）
                       │
                       └── WSS / HTTPS ⇄ Agent（指标、PTY、文件、未来 Docker 适配器）
```

## 现有链路

| 业务 | Hub 入口 | Agent 入口 |
| --- | --- | --- |
| 状态 | `hub/web/api/client/report_v2.go` | `agent/server/websocket.go` 与 `agent/monitoring/` |
| 终端 | `hub/web/api/terminal/` | `agent/terminal/` |
| 文件 | `hub/web/filemanager/` 和 `hub/web/rpc/jsonrpc/admin.file.go` | `agent/server/files.go` |
| 协议 | `hub/protocol/v2/jsonrpc.go` | `agent/protocol/v2/jsonrpc.go` |

两个 `protocol/v2` 目录目前按上游方式各自维护；修改协议时必须同时更新并做双端兼容验证。第一版不急于抽共享模块，以免为简单扩展引入部署依赖。

## 第一条新增业务链路：Docker 只读

1. Agent 检测本机 Docker 是否可用，上报 `docker.read` 能力。
2. Hub 的管理员界面请求某节点容器列表；统一授权层校验 `docker.read` 和节点范围。
3. Hub 经现有 v2 事件连接下发结构化请求，含唯一 ID、过期时间和查询参数。
4. Agent 再校验本地 Docker 开关与请求类型，通过本机 Unix socket 调 Docker Engine API；返回限定字段与大小的结果。
5. Hub 记录操作者、节点、请求、结果和耗时；前端展示状态。日志采用限行数/限字节流式读取。

初期只读 UI 不是 Docker socket 的操作系统级权限隔离。Docker daemon 访问权限很高，启停等写操作须在权限和审计架构就绪后再开发。

## 需要建立的模块边界

- `hub/internal/access`：用户对节点及动作的授权，所有 REST/JSON-RPC/WebSocket 敏感入口复用。
- `hub/internal/operations`：有过期时间与去重 ID 的 Agent 请求派发和结果关联。
- `hub/internal/audit`：操作尝试、拒绝、开始、结束和错误的结构化记录。
- `agent/internal/capabilities`：本地启用能力、版本和运行时可用性。
- `agent/internal/docker`：只读 Engine API 适配器，不暴露任意 URL/方法代理。

这些目录是下一阶段的目标边界，**本次没有宣称它们已实现**。先保持已有监控功能运行，再以 Docker 只读链路验证新边界。

