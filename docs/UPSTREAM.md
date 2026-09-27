# 来源与版本

| 组件 | 来源 | 本次快照 |
| --- | --- | --- |
| Hub | 当前目录同级 `komari/`；https://github.com/komari-monitor/komari | `9812acf` |
| Agent | 当前目录同级 `komari-agent/`；https://github.com/komari-monitor/komari-agent | `828afaf` |
| Web UI | https://github.com/komari-monitor/komari-web | `0321789bc1989e53df729dfc98bed2a2800c39c6`；构建时获取，不纳入公开项目仓库 |

Hub 和 Agent 中的 `LICENSE`/`NOTICE` 文件随源码保留。Web UI 仓库快照中未见独立 LICENSE 文件，因此 PierOps 公开 Git 仓库不包含其源码；本地/Docker 构建按固定 commit 获取。公开分发包含前端的二进制或镜像前仍须确认授权。上游构建将前端 `dist` 打为 `hub/web/public/defaultTheme/dist.tar.zst`；本项目的 `scripts/build-frontend.sh` 复现这个步骤。

本项目不修改同级的 `komari/`、`komari-agent/` 原仓库。未来同步上游时先记录 commit，再按组件逐项合并并验证协议兼容性。
