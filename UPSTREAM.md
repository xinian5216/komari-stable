# UPSTREAM.md — 上游来源与 fork 关系

> 本文件是**溯源记录**：任何时候都能据此判断"哪些是原项目代码，哪些是 Komari Stable 的改动"。

## 1. 原项目

| 项 | 值 |
| --- | --- |
| Original Project | **Komari** — A simple server monitor tool（轻量自托管服务器监控） |
| Original Repository | https://github.com/komari-monitor/komari |
| Original Author / Org | `komari-monitor`（作者个人站点：https://ss.akz.moe） |
| Original License | **MIT**（`LICENSE` 原样保留） |
| Original Notices | `NOTICE`、`README_zh-cn.md`、`.github/ISSUE_TEMPLATE/*` 原样保留 |
| 文档 | https://www.komari.wiki/ · 源：https://github.com/komari-monitor/komari-document |
| 上游归档时间 | **2026-09-15**（仓库转为 read-only，作者不再接受修复） |

## 2. Fork 基线

| 项 | 值 |
| --- | --- |
| Fork 日期 | 2026-09-16 |
| 最后使用的 upstream tag | `1.5.0-fix1` |
| 最后使用的 upstream commit | `0ca87aafd184ed75f9030ede0902772142af5eec`（= 归档时 `main` HEAD） |
| 基线分支 | `upstream-baseline`（只读镜像，**永不修改**） |
| 维护分支 | `stable` |
| Fork 维护者 | `xinian5216`（GitHub） |

## 3. 本 fork 的改动清单（相对上游基线）

> 原则：**只做 Bug 修复 / 安全修复 / 兼容性 / 构建 CI / 文档**；不重构、不加功能。
> 每次发布在此追加一节（与 `CHANGELOG.md` 对应）。

### 1.5.0-stable.0（基线版本，未发布）

仅文档与 CI，**无业务代码改动**：

- 新增：`BASELINE_AUDIT.md`、`MAINTENANCE_POLICY.md`、`SECURITY.md`、`CHANGELOG.md`、`TECH_DEBT.md`、
  `UPSTREAM.md`、`PUBLIC_RELEASE_AUDIT.md`、`.agent/*`（AI 索引）、`.gitleaks.toml`、
  `scripts/check_agent_index.py`
- 改写：`README.md`（标注 community-maintained fork，保留原作者与 MIT 信息）
- 修改：`.gitignore`（加固：凭据/日志/本地产物/AI 缓存等，全部为"文件形状"规则）
- 修改：`install-komari.sh`（**参数化**下载来源：`REPO_OWNER`/`REPO_NAME`/`RELEASE_BASE`/`GITHUB_API_BASE`）
- 新增 CI：`.github/workflows/{stable-ci,stable-release,secret-scan}.yml`
- 未改动：`cmd/ database/ internal/ pkg/ protocol/ utils/ web/`（全部业务代码与上游一致）

## 4. 组件仓库（本 fork 维护）

| 组件 | 仓库 | 说明 |
| --- | --- | --- |
| 服务端 | `xinian5216/komari-stable`（本仓库） | 基于上游 `1.5.0-fix1`；只做修复与兼容性维护 |
| 前端默认主题 | `xinian5216/komari-web-stable` | 源自上游 `komari-web`；本 fork 固定 tag 引用 |
| Agent | `xinian5216/komari-agent-stable` | 源自上游 `komari-agent`；本 fork 的 Agent 安装/更新通道；**v2 协议冻结、向后兼容** |

## 5. 代码溯源规则（重要）

- **Go module path 保持 `github.com/komari-monitor/komari` 不变**：改 module path 会导致
  全仓库 import 变更（数百文件）并破坏与上游的对照关系，属于禁止类改动。
- 因此 `cmd/ database/ internal/ pkg/ protocol/ utils/ web/` 中的 import 仍指向上游 module path，
  这是**有意为之**，不代表代码来自上游当前主线（上游已停止更新）。
- 判断某文件是否为 fork 修改：与 `upstream-baseline` 分支 diff 即可：

```bash
git diff upstream-baseline --stat          # 全部差异
git diff upstream-baseline -- <path>       # 单文件差异
```
