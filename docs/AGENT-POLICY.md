# Agent 本地策略（M2）

Hub 的用户授权与 Agent 的本地策略必须同时允许一项操作。Agent 默认只开启监控，不开启文件、终端或命令；原 `disable_web_ssh` 仍是远程运维的总关闭开关，设为 `false` 不再自动开启能力。

## 本机配置

| JSON 字段 | 环境变量 | 默认值 / 说明 |
| --- | --- | --- |
| `enable_file_read` | `AGENT_ENABLE_FILE_READ` | `false`，目录列表、信息、搜索和下载 |
| `enable_file_write` | `AGENT_ENABLE_FILE_WRITE` | `false`，创建、修改、复制、移动、删除和上传 |
| `file_roots` | `AGENT_FILE_ROOTS` | 空；以分号分隔的绝对、已存在目录；禁止整个文件系统根目录 |
| `enable_terminal` | `AGENT_ENABLE_TERMINAL` | `false` |
| `enable_exec` | `AGENT_ENABLE_EXEC` | `false` |
| `execution_user` | `AGENT_EXECUTION_USER` | 空；开启终端/命令必须指定已有非 root 用户 |
| `node_uuid` | `AGENT_NODE_UUID` | 空；用于后续操作票据绑定 Hub 节点 UUID |

配置加载沿用现有顺序：命令行默认值/参数 → 环境变量 → JSON 配置文件。能力设置只在启动时由本机加载，变更后重启 Agent；Hub 没有远程修改本地策略的入口。Agent 和 Hub 构建统一要求 Go 1.25 或更新兼容版本。

只读文件示例（替换端点和凭据；此文件应放在节点本机并限制读取权限，不提交 Git）：

```json
{
  "endpoint": "https://hub.example.invalid",
  "token": "REPLACE_ON_NODE",
  "disable_auto_update": true,
  "enable_file_read": true,
  "file_roots": "/srv/pierops-workspace",
  "enable_file_write": false,
  "enable_terminal": false,
  "enable_exec": false
}
```

允许目录需事先创建，且不应包含 Agent 配置、令牌、私钥或其他不应开放的文件。`/` 的列表和搜索映射到已配置的允许目录；其他文件请求必须使用允许目录下的绝对路径。

## 文件边界

所有远程文件访问（包括递归搜索/复制、原始 HTTP 流、上传分片和提交）使用启动时固定的 `os.Root` 目录句柄。路径匹配后，实际读写由句柄执行；不使用“检查符号链接后再按全局路径打开”的方式。允许内部相对符号链接；绝对符号链接和逃出根目录的链接不能读取或写入。实现依据：[Go 官方路径边界 API 说明](https://go.dev/blog/osroot)。

- 禁止删除、移动或替换允许目录本身；禁止跨两个允许目录直接移动，可分别复制和删除。
- 写入和下载针对普通文件，拒绝设备/FIFO；列表仍能展示目录和符号链接元数据。
- 上传 ID 只能包含字母、数字、横线和下划线，长度 1–128；不得通过分片文件名注入路径。
- 权限修改限制为 `0000–0777`；远程 `chown` 不开放。
- 复制保留普通权限，暂不保留源时间戳，以避免路径形式时间设置的符号链接竞争。

`os.Root` 是路径边界，不是容器或整个进程的沙箱。它不隔离硬链接、管理员配置的挂载点和网络。目录内容由节点管理员负责选择；文件读写仍以 Agent 进程身份执行。

## 终端与命令身份

Unix 节点必须指定非 root 执行用户。Agent 若以 root 启动，会在启动子进程前切换 UID、GID 和附加组；若以普通用户启动，只能选择自身。子进程工作目录是该用户主目录，环境仅包含基础 PATH、HOME、USER、终端和语言设置，不继承 Agent 令牌与连接配置。

终端和任意命令按该用户本身的系统权限执行，**不受文件管理允许目录限制**。若执行用户拥有 sudo、Docker socket、特权组或敏感文件权限，这些权限仍然存在。M2 不提供命令容器沙箱。Windows 暂不开启受控终端/命令，监控和文件目录边界可继续使用。

## 当前边界

本条开发步骤已实现能力、目录与执行身份。短期票据和 Hub 对旧 Agent 的拒绝将在下一条开发步骤接入；真实 VPS 验收按用户要求暂缓。
