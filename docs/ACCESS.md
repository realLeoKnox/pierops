# M1 用户、节点与动作授权

## 账户角色与兼容

已有账户在数据库迁移时获得 `owner` 角色，安装时创建的首个账户也为 owner。owner 可以管理平台和所有节点。受限账户只能由 owner 创建，初始授权必须明确列出节点与动作；空授权不会获得节点操作能力。

| 角色 | 可授予的动作 |
| --- | --- |
| owner | 平台管理与全部节点操作；不通过受限账户 API 修改 owner |
| operator | 下表全部节点动作 |
| viewer | `node.read`、`file.read`、`command.read`、预留的 `docker.read` |
| disabled | 拒绝所有运维操作，删除已有登录会话 |

| 动作 | 范围 |
| --- | --- |
| node.read | 查看指定节点的管理信息，隐藏 Agent token 和私人备注 |
| terminal.open | 打开或恢复该节点终端 |
| file.read | 文件列表、根目录、元信息、搜索、下载及短期预览 |
| file.write | 上传、创建目录、删除、移动、复制、权限和所有者修改 |
| command.exec | 在指定节点运行命令 |
| command.read | 读取指定节点的单项命令结果 |
| docker.read | 授权字段已预留；Docker 适配器尚未实现 |

节点和动作都使用精确匹配，不支持 `*`。查看完整任务列表、任务的跨节点聚合结果、Agent token、平台设置、主题、插件、备份与恢复等操作限 owner。

上游单一全局 API Key 继续作为 owner 凭据；本阶段没有提供受限 API Key。可信的进程内调用使用显式 internal 主体。插件具有 Hub 进程权限，只有 owner 可以安装和管理插件。

## 统一入口

- `hub/internal/access` 在每次操作时重新读取会话有效期、账户角色及节点授权；不依赖 WebSocket 握手时的缓存权限。
- REST 的管理路由默认要求 owner，运维路由在 handler 中检查具体动作。直接 JSON-RPC、批量 JSON-RPC 与 WebSocket 使用同一授权服务。
- 多节点命令先检查全部目标；任何一个目标无权访问时不创建任务、不发送命令。
- 终端的每次输入会重新检查权限；空闲终端每五秒检查一次，权限撤销、会话到期或 API Key 更换后关闭连接。
- 上传续传绑定用户及节点；旧版本未记录上传所有者的续传状态不能继续使用，需重新上传。
- 文件预览令牌绑定原始身份与文件路径，下载时重新校验授权，权限撤销后令牌失效。

公开监控接口仍遵循原来的公开/私有站点配置；节点授权限制运维管理能力。受限用户经公开接口不能取得 Agent token、私人备注或未公开节点的管理员视图。已经开始的单次文件流不会在中途撤销，下一次分片或新请求会重新检查；后续 M2 将增加操作票据与 Agent 本地限制。

## 管理 API

使用已有的登录 cookie 或 owner API Key 认证。REST 响应沿用 `{status,data}` 格式，权限拒绝返回 403。直接 RPC 通过 `/api/rpc2`，权限拒绝错误码为 `-32041`。创建用户与修改策略沿用已有的敏感操作 2FA 要求。

| REST | RPC 方法 | 参数 |
| --- | --- | --- |
| GET /api/admin/access/self | admin:accessGetSelf | 当前账户，无需参数 |
| GET /api/admin/access/users | admin:accessListUsers | 返回 UUID、用户名和角色，不包含凭据 |
| POST /api/admin/access/users | admin:accessCreateUser | username、password、role、grants |
| GET /api/admin/access/users/:uuid | admin:accessGetPolicy | uuid |
| PUT /api/admin/access/users/:uuid | admin:accessSetPolicy | uuid、role、grants；替换完整策略 |
| POST /api/admin/access/audit | admin:accessGetAudit | limit（默认100，上限200）、before（游标ID） |

创建受限账户示例参数（密码为示例占位，实际请求使用自行设置的密码）：

```json
{
  "username": "ops-viewer",
  "password": "replace-with-your-password",
  "role": "viewer",
  "grants": [
    {"client_uuid": "替换为节点UUID", "action": "node.read"},
    {"client_uuid": "替换为节点UUID", "action": "file.read"}
  ]
}
```

新增受限账户密码采用 bcrypt，长度为 12–72 字节；已有上游账户密码格式仍可登录。本阶段策略管理以 API 为主，现有 Komari 页面尚未按受限角色重新组织所有菜单。

## 审计语义与验证范围

`operation_audits` 记录主体、动作、节点、授权或拒绝、原因与时间。策略变更与新策略快照在同一数据库事务中保存。授权尝试记录失败会阻止派发操作。审计中不保存密码、token、命令正文、文件路径或原始请求；`allowed` 表示授权通过，不表示 Agent 已执行成功。

本地使用临时 SQLite 数据库及测试服务验证跨节点拒绝、读写角色分离、会话过期、API Key 轮换、策略替换原子性、批量命令预检查、位置参数解码、REST/RPC/WS 入口及预览令牌撤销。真实 VPS 与 Agent 的运行验收由用户暂缓；本阶段不据此宣称生产验证完成。
