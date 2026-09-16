# PUBLIC_RELEASE_AUDIT.md — 公开发布前检查报告

> 更新时间：2026-09-16（第二阶段：三仓库建立 + 首次远端 CI 预演）
> 状态：**三个仓库均已创建（Private）并完成推送与预演；尚未转为 Public，未创建任何 Release。**

## 0. 结论摘要

| 项 | 结果 |
| --- | --- |
| 仓库创建 | ✅ `xinian5216/komari-stable`、`xinian5216/komari-web-stable`、`xinian5216/komari-agent-stable`（均为 Private，默认分支 `stable`） |
| Secret / 历史 / 隐私扫描 | ✅ 三个仓库全部通过（详见 §4） |
| 依赖引用审计（关键路径） | ✅ 已消除全部 `komari-monitor/*` **运行期/构建期**关键依赖（详见 §5） |
| 净室构建验证 | ✅ 三仓库均可从全新克隆构建（Server 嵌入固定前端 + 冒烟通过） |
| Docker 构建（amd64+arm64） | ✅ **实际构建成功**（非推送预演） |
| CI：agent-index / build-frontend / 三个平台构建 / secret scan | ✅ 全部通过 |
| CI：`go test`（Linux） | ❌ **失败**——3 个 `pkg/jsruntime` 测试在 ubuntu 上 "permission denied"（Windows 本地通过；上游 CI 从无测试，属**继承自上游的潜在缺陷**） |
| CI：`govulncheck` | ❌ 失败（**符合预期**：2 个已知可达漏洞） |
| **是否可安全转 Public** | ⛔ **暂不可**。需先决定/处理 §7 的两个阻塞项 |

## 1. 三个仓库与基线

| 仓库 | 可见性 | 默认分支 | 分支 | 固定 tag / commit |
| --- | --- | --- | --- | --- |
| `xinian5216/komari-stable` | Private | `stable` | `stable` @ `0751063`、`upstream-baseline` @ `0ca87aa` | —（尚未打 tag） |
| `xinian5216/komari-web-stable` | Private | `stable` | `stable` @ `c9d4749`、`upstream-baseline` @ `dec6495` | **`v1.5.0-stable.0`** → `c9d4749` |
| `xinian5216/komari-agent-stable` | Private | `stable` | `stable` @ `84647ed`、`upstream-baseline` @ `9e532e04` | **`v1.5.10-stable.0`** → `84647ed` |

- 上游基线：Server `1.5.0-fix1` `0ca87aa`；Web 上游 `1.5.0` `dec6495`；Agent `1.5.10` `9e532e04`。
- **未创建任何 GitHub Release**（按指示）。
- Web 的固定 tag 在创建时**已包含安装来源改造**（否则 Server 构建出的面板仍会指向上游 Agent 安装地址——这与"不得存在关键上游依赖"冲突；详见 §5）。

## 2. 工作流清理记录（本轮）

**Server（本仓库）**
- 删除：`build.yml`、`release.yml`、`release-docker.yml`、`docker-publish.yml`、`snapshot.yml`（均会以**上游默认分支**作为前端构建输入，且与 stable-* 重复）、`rebuild-release.yml`（引用了上游跨仓库 action `komari-monitor/komari/.github/actions/...`）
- 保留：`stable-ci.yml`、`stable-release.yml`、`docker-preview.yml`、`secret-scan.yml`、`cleanup-packages.yml`
- 复合 action `build-frontend` 的默认来源改为 `xinian5216/komari-web-stable @ v1.5.0-stable.0`（不再可能静默回退到上游分支）
- 全部为普通提交删除，**git 历史完整保留**

**Web 镜像**
- 删除：`follow-komari-release.yaml`（定时跟随上游 release 并向 komari-monitor 派发事件）、`generate-release-notes.yml`（依赖 `OPENAI_API_KEY`）、`development.yaml`（SSH 部署到上游作者生产主机）
- 保留：`build.yaml`、`i18n-sync.yml`、`preview-theme.yaml`

**Agent 镜像**
- 删除：`generate-release-notes.yml`（依赖 `OPENAI_API_KEY`）
- 保留：`build.yml`、`release.yml`、`release-docker.yml`、`snapshot.yml`（镜像名使用 `${{ github.repository }}`，自动指向本 fork）

## 3. CI 首次预演结果（远端实测）

| 检查项 | 结果 | 证据 |
| --- | --- | --- |
| Agent index check | ✅ PASS | run 35065641452 · job `agent-index` |
| 前端固定 ref 构建（私有仓库 + 只读 Deploy Key） | ✅ PASS | job `build-frontend`；clone `xinian5216/komari-web-stable@v1.5.0-stable.0` |
| Linux amd64 构建 | ✅ PASS | job `build (linux, amd64)` |
| Linux arm64 构建 | ✅ PASS | job `build (linux, arm64)` |
| Windows amd64 构建 | ✅ PASS | job `build (windows, amd64)` |
| Secret scan（全历史 + 工作树） | ✅ PASS | run 35065641424（`Secret Scan`） |
| **`go vet`** | ✅ PASS（`test` job 内，vet 步骤通过） | 同上 |
| **`go test ./...`（Linux）** | ❌ **FAIL** | `pkg/jsruntime`：`TestNodeCoreModulesAndECMAScriptBuiltins`、`TestStorageDirIsConfinedAdditionalRoot`、`TestChildProcessPermissionAndExecution` —— `open /tmp/.../data/sync.txt: permission denied` |
| **`govulncheck`** | ❌ **FAIL（预期）** | 2 个可达漏洞：`golang.org/x/text@v0.33.0`（GO-2026-5970）、`golang.org/x/net@v0.41.0`（GO-2026-5026） |
| **Docker amd64 + arm64 构建** | ✅ **PASS（实际构建成功）** | run 35065362477 `Docker Preview Build (no push)`：`build-docker-binaries` ×2 + `docker-preview`（buildx 双平台，**不推送**） |

> `vuln` 失败是刻意设计的**诚实信号**（发现可达漏洞即失败），不是流水线缺陷；
> 修复计划见 §7。`test` 失败是**真实缺陷**（详见 §7）。

## 4. 三仓库公开发布前检查

| 检查项 | komari-stable | komari-web-stable | komari-agent-stable |
| --- | --- | --- | --- |
| Secret（工作树） | ✅ 通过（gitleaks dir；`scan_secrets.py` 仅余上游测试夹具） | ✅ 通过（gitleaks 无泄漏；文本文件扫描 0 命中，二进制资产误报已排除） | ✅ 通过（gitleaks 无泄漏；余项为公共 DNS IP 等上游内容） |
| Git 历史 | ✅ 703 commits / 2 处误报 | ✅ 全历史无泄漏 | ✅ 204 commits 无泄漏 |
| 个人信息 | ✅ 无本地路径/用户名；提交身份 `xinian5216@users.noreply.github.com` | ✅ 同左（新增提交 `c9d4749` 身份合规） | ✅ 同左（新增提交 `84647ed` 身份合规） |
| Commit author/email | ✅ 全部为本 fork noreply 身份（历史中上游作者元数据保持原样） | ✅ | ✅ |
| License / Copyright | ✅ `LICENSE`(MIT)、`NOTICE` 原样保留 | ⚠️ **上游 komari-web 本身不含 LICENSE 文件**（GitHub API `license=null`）；本镜像保持原状，未添加/未移除 | ✅ `LICENSE` 原样保留（含原作者版权行） |
| 上游来源 | ✅ `UPSTREAM.md`（基线 tag/commit、fork 日期、改动清单） | ✅ `UPSTREAM.md` | ✅ `UPSTREAM.md` |
| 安装链接 | ✅ `install-komari.sh` 全部指向 `xinian5216/komari-stable`（4 处 URL 已参数化） | ✅ 安装命令指向 `xinian5216/komari-agent-stable` | ✅ `install.sh` / `install.ps1` 全部指向本仓库 |
| 更新链接 | ✅ 构建资产来自本仓库 Release | ✅ 面板升级检查 API 指向 `xinian5216/komari-stable` | ✅ Agent 自更新 slug = `xinian5216/komari-agent-stable` |

## 5. `komari-monitor/*` 残留引用审计（分类）

**结论：不存在会让核心安装/升级流程回到上游的关键路径。** 残留引用仅限三类：

1. **Go module path**（`github.com/komari-monitor/komari[-agent]`）：语言内标识符，非网络依赖（改它需全仓库改 import，属禁止类改动）。
2. **Credits / 上游信息**（按要求保留）：`README.md`、`UPSTREAM.md`、`about.tsx`（README 链接与版权）、`NavBar.tsx`（组织主页链接）、`komari-theme.json` 的 `url` 字段、`LICENSE`/`NOTICE` 版权行。
3. **文档中的对照引用**：`.agent/*`（协议冻结对照 `komari-protocol`、文档源 `komari-document`、上游作为"择优移植来源"）。

已消除的关键耦合（本轮修复）：
- `rebuild-release.yml` 的跨仓库 action 引用（已删除该 workflow）
- 6 个上游 workflow 的"默认前端 = 上游默认分支"回退路径（已删除，且复合 action 默认值改为本 fork）
- Web 的 Agent 安装命令 / Docker 镜像 / 升级检查（已改为集中配置 `src/lib/repoSources.ts`）
- Agent 的安装脚本与自更新 slug

## 6. 净室验证（全新克隆，不依赖本机文件/未提交产物）

| 验证项 | 结果 |
| --- | --- |
| 全新克隆三仓库 | ✅ server@`43a2fc9` / web@`c9d4749` / agent@`84647ed` |
| Agent：`go build` | ✅ `AGENT_BUILD_OK` |
| Web：`npm install` + `npm run build` | ✅ `WEB_BUILD_OK`（808 包，dist 生成） |
| 安装来源核对 | ✅ `FORK_OWNER=xinian5216` / `AGENT_REPO=komari-agent-stable`；`src/` 内不再出现上游 agent 地址 |
| Server：用固定前端 tag 打包 + 构建 | ✅ `SERVER_BUILD_OK`（35 MB 二进制） |
| Server 冒烟 | ✅ 启动正常（首启安装模式），`/api/install/status` 200 |

> 说明：因仓库当前为 Private，**真正的"匿名/无凭据克隆"只能在转为 Public 之后执行**；
> 本轮以"净室（全新目录 + 全新克隆 + 无本机缓存产物）"作为等价验证。匿名复检列入 §8 待办。

## 7. 阻塞项（转为 Public 前需要决定）

### 7.1 `pkg/jsruntime` 在 Linux 上的测试失败（真实缺陷）

- 现象：ubuntu-latest 上 3 个测试以 `permission denied` 失败（`TestNodeCoreModulesAndECMAScriptBuiltins`、
  `TestStorageDirIsConfinedAdditionalRoot`、`TestChildProcessPermissionAndExecution`）；**Windows 本地全绿**。
- 影响面：`pkg/jsruntime`（插件 JS 运行时的文件系统沙箱）。失败发生在**沙箱写入**路径，
  可能影响插件在 Linux 上的文件读写——**需要优先定位**（这是上游从未被 CI 覆盖的区域）。
- 已排除：`mkdir` 默认 mode（`fsMode` 对 options 对象有正确的 fallback `0o777`）。
- 待查方向：`pkg/jsruntime/fs`（`os.OpenRoot`/`withinAnyRoot`/符号链接解析）在 Linux 与 Windows 的语义差异。
- 处理原则：**必须先定位根因，再以 failing test 为先导做最小修复**；不跳过测试、不放宽沙箱校验。

### 7.2 `govulncheck` 的 2 个可达漏洞

- `golang.org/x/text@v0.33.0` → 需 `v0.39.0`；`golang.org/x/net@v0.41.0` → 需 `v0.55.0`。
- 计划：作为 `1.5.0-stable.1` 的内容，一次一个模块族、单独提交、单独发布（尚未开始）。

## 8. 转为 Public 前的剩余待办

1. 决定 §7.1 的处理方式（定位 + 修复，或明确接受 CI 红并文档化）。
2. 决定 §7.2 的依赖升级时机（建议先修 7.1，再一起发 `1.5.0-stable.1`）。
3. 转 Public 后执行**匿名克隆复检**：`gitleaks` 全历史 + `scan_secrets.py` + 三仓库净室构建。
4. 转 Public 后：把前端来源从 SSH Deploy Key 切回 HTTPS，并**移除 Deploy Key 与 `FRONTEND_DEPLOY_KEY` secret**。
5. 首次 Release（`1.5.0-stable.0`）由人工确认后再创建（本轮明确未创建）。
6. **转 Public 之前，前端生成的 Agent 安装命令（raw.githubusercontent.com/xinian5216/komari-agent-stable/...）
   对终端用户不可访问**（Private 仓库的 raw URL 需要凭据）——因此"安装命令可用"这一项必须在转 Public 后复验。

---

## 附录：首次公开发布记录（1.5.0-stable.0）

公开发布已完成，以下是可复现的发布事实（按时间顺序）。

### 公开与安全开关

| 项目 | 状态 |
| --- | --- |
| `komari-web-stable` | Public（先公开，Server CI 依赖它） |
| `komari-agent-stable` | Public |
| `komari-stable` | Public（最后公开） |
| Secret Scanning / Push Protection | 三仓库均 enabled |
| 默认分支 | 三仓库均为 `stable` |
| 跨仓库私有凭据 | 已移除（前端改为匿名 HTTPS 拉取；Deploy Key 与 `FRONTEND_DEPLOY_KEY` 已删除） |

### 冻结的版本

| 仓库 | tag | 指向 commit | 说明 |
| --- | --- | --- | --- |
| komari-stable | `v1.5.0-stable.0` | `43953d6`（上游基线 `0ca87aa` = 1.5.0-fix1） | Release 已发布，附 `komari-linux-amd64`、`komari-linux-arm64`、`komari-windows-amd64.exe`、`SHA256SUMS` |
| komari-web-stable | `v1.5.0-stable.0` | `c9d4749` | 前端镜像（CI 固定引用） |
| komari-agent-stable | `v1.5.10-stable.0` | `84647ed` | Agent fork 首个 tag；Release 附各平台二进制 + 逐文件 `.sha256` + `SHA256SUMS` |

说明：tag 之后的 `stable` 分支继续前进（迁移脚本、E2E 演练、文档），版本 tag 不移动、Release 产物与 tag 一致。

### 发布验证

- 正式二进制版本串：`1.5.0-stable.0`（无 `-candidate`；二进制中出现的 `candidate` 字样是 Go 标准库 TLS 错误文案）。
- Docker：`ghcr.io/xinian5216/komari-stable:v1.5.0-stable.0` 与浮动标签 `:stable`，多架构 `linux/amd64,linux/arm64`。
- 匿名端到端演练（无 PAT、无本机源码、未登录 GitHub）：从 README 一键命令装 Server → 完成向导 → 面板 RPC 建节点 → 面板命令装 Agent → 节点在线（v2 协议）→ 版本与升级检查均指向本仓库 → 面板 Web 正常加载。
- 迁移演练：官方 1.4.3（`bf6b45e`）实例 → 本 fork 原地升级（数据保留、自动备份、校验和验证、启动失败回滚）；官方 Agent → 本 fork 接管（参数与 token 保留）。

### 已知未做项（有意为之）

- 19 个仅在依赖模块中、本代码不可达的漏洞未处理。
- 未开启 GitHub 平台级 “Release immutability”（该仓库设置为 UI 项，API 未暴露；本项目通过“不移动 tag + 发布产物附校验和”保证不可变语义）。
