# BASELINE_AUDIT.md — Komari Stable 基线审计报告

> 本文件是 **Komari Stable**（社区维护稳定分支）第一阶段（项目审计）的产出。
> 本阶段**未修改任何业务代码**；所有结论均来自对上游源码、CI 配置、官方文档、GitHub API 的
> 读取，以及一次完整的本地构建 / 测试 / 运行验证。

| 项目 | 值 |
| --- | --- |
| 审计对象 | `komari-monitor/komari`（上游官方仓库） |
| 审计基线 commit | `0ca87aafd184ed75f9030ede0902772142af5eec`（= `main` HEAD） |
| 对应 tag | `1.5.0-fix1`（= 上游最后一次发布，2026-09-14 13:26 UTC） |
| 上一发布 tag | `1.5.0` = `f18ad72c173b8821711cba0f2fb238c2486ec9af`（2026-09-14 08:35 UTC） |
| 审计日期 | 2026-09-16 |
| 审计主机 | Windows 11 / amd64（本地构建验证；CI 目标平台见 §5） |
| 仓库状态 | 上游仓库 **已于 2026-09-15 被作者归档（read-only）** |

---

## 0. 结论摘要（TL;DR）

1. **上游已停止维护**：`komari-monitor/komari` 于 2026-09-15 被归档为只读；最后一次提交与
   最后一次发布均为 `1.5.0-fix1`。生态中的 `komari-web` / `komari-agent` / `komari-protocol`
   / `komari-document` 等仓库当时**未**归档，但主仓库不会再接受修复。→ 社区稳定分支是当前
   唯一能继续获得 Bug 修复与安全补丁的途径。
2. **基线可独立构建、可测试、可运行**：本地在 Windows 上完整走通
   `前端构建 → 主题打包(zstd) → 后端 CGO 编译 → 全量测试 → 启动冒烟` 全链路，
   全部通过（详见 §4、§5）。结论：**Fork 可以在脱离原作者的情况下完成构建与验证**。
3. **CI 存在明确缺口**：上游 CI **从不运行 `go test`**、没有 Go 静态检查、没有依赖/CVE 扫描；
   且 `setup-go` 版本写死 `1.23`（与 `go.mod` 的 `go 1.25.0` 不一致，靠 Go 工具链自动下载兜底）。
   → 新分支的 CI 必须补齐：测试、lint、漏洞扫描。
4. **存在 2 个"代码可达"的已知漏洞**（govulncheck）：`golang.org/x/text`（GO-2026-5970）与
   `golang.org/x/net`（GO-2026-5026）；另有 9 个导入包级与 22 个模块级漏洞当前不可达。
   建议按"最小依赖升级"原则处理（见 §6、§9），升级前需评估兼容性并单独发布。
5. **数据库与 Agent 协议必须冻结**：主库（SQLite）由 GORM `AutoMigrate` + 一次性迁移组成，
   监控数据另库（`metrics.db`，支持 sqlite/mysql/postgres）；Agent 协议自 1.5.0 起 **仅支持 v2**
   （JSON-RPC 2.0，WebSocket + POST 回退）。任何 schema / 线协议改动都属最高风险区。
6. 已知未修复问题 6 个（上游 open issues，§8），其中"历史流量统计异常（TB/PB 级）"与
   "默认主题历史统计报 `unknown metric key`"最值得优先修复。

---

## 1. 上游与生态现状

| 仓库 | 归档 | 默认分支 | 最近推送 | 说明 |
| --- | --- | --- | --- | --- |
| `komari-monitor/komari` | **是（2026-09-15）** | `main` | 2026-09-14 | 服务端 + 默认主题容器 + 插件宿主（本审计对象） |
| `komari-monitor/komari-web` | 否 | `radix` | 2026-09-14 | 前端默认主题（React 19 + Vite + TS），独立仓库、独立 tag（`1.5.0` = `dec649518a769882308ab80794c633bf6bfc265b`） |
| `komari-monitor/komari-agent` | 否 | `main` | 2026-09-15 | Agent（Go），独立版本线（最新 `1.5.10`） |
| `komari-monitor/komari-protocol` | 否 | `main` | 2026-08-04 | v1/v2 线协议冻结 + 冻结测试（"frozen v1/v2, guarded by freeze tests"） |
| `komari-monitor/komari-document` | 否 | `main` | 2026-09-14 | 文档源（发布到 komari.wiki） |
| `plugin-sdk` / `plugin-dev` / `create-komari-plugin` / `plugin-market` / `theme-market` | 否 | — | 2026-08~09 | 插件与主题生态（面板运行期会访问这两个 market） |

其它事实：

- 上游 `main` 累计 861 次提交，首个提交 2025-04-12；标签节奏为"小步快发"（1.2.x → 1.5.0 共 ~3 个月）。
- `1.5.0` → `1.5.0-fix1` 的差异：**移除内置流量定时通知**（`utils/notifier/traffic_report.go` 等，
  共 612 行删除），并把 `models.TrafficReportNotification` 从 `AutoMigrate` 建表列表中移除
  （**不删表、不删列**，仅不再创建新表 → 向后兼容）。
- 仓库内自带安装脚本 `install-komari.sh`（46 KB，systemd + 二进制安装），其中同时引用上游
  `komari-monitor/komari` 与第三方 Lite 分支 `nuomiiiii/komari`。**Fork 必须决定该脚本的去向**（§9 建议）。

## 2. 仓库结构与规模

```
cmd/          Cobra 入口（server / chpasswd / disable2FA / permitPasswordLogin）
database/     GORM 数据层（models / dbcore / accounts / clients / tasks / notification …）
internal/     服务端内部模块（server 生命周期、config、migrations、metricstore、plugin、scheduler…）
pkg/          可复用库（metric 指标存储引擎 30k 行、jsruntime(goja)、rpc、timeutil）
protocol/v2/  Agent 线协议定义（JSON-RPC 2.0 方法常量与结构）
utils/        日志、通知渠道、GeoIP、续费/流量工具
web/          Gin 路由与 HTTP 层（api / rpc(jsonrpc) / terminal / filemanager / upload / install / recovery / security）
web/public/   //go:embed 前端主题（defaultTheme/dist.tar.zst + komari-theme.json，**不随仓库提交**）
```

- Go 代码量：**72,673 行**（376 个 `.go` 文件），其中 `pkg/` 约 30.4k、`web/` 约 19.8k、`internal/` 约 14.4k。
- 测试：**89 个 `_test.go`，487 个 `Test*` 函数**（`pkg/metric` 最重）。
- 前端独立仓库：React 19 / Vite / TypeScript / Radix UI / Tailwind 4；`npm run lint`（ESLint）可用。
- 无 `Makefile`、无 `go.work`、无 `.golangci.yml`；仓库内仅一个安装脚本。

## 3. 技术栈与官方构建链路

1. **后端**：Go（`go.mod` 要求 `go 1.25.0`），Gin + GORM；**必须 CGO**（`mattn/go-sqlite3`）。
2. **前端**：独立仓库 `komari-web` 构建出 `dist/`，用 `tar + zstd -19` 打包成
   `web/public/defaultTheme/dist.tar.zst`，连同 `komari-theme.json` 一起被 `//go:embed` 嵌入二进制
   （`web/public/public.go:18-22`）。
   ⚠️ 由此得到一条**硬性构建顺序**：**没有前端产物就无法编译后端**（embed 缺文件直接编译失败）。
3. **交叉编译**：CI 用 **zig 0.14.1** 作为 C 交叉编译器（`CC=zig cc -target <triple>`），
   产物矩阵：`linux/{amd64,arm64,386,riscv64,loong64}`、`windows/{amd64,arm64,386}`。
4. **Docker**：`Dockerfile` 基于 `alpine:3.21`，把预编译二进制 `komari-${TARGETOS}-${TARGETARCH}`
   拷入镜像；`EXPOSE 25774`；`CMD ["/app/komari","server"]`。镜像由 CI 用 buildx 构建并推送到
   `ghcr.io/<owner>/<repo>`。
5. **版本注入**：`utils/version.go` 默认 `0.0.1/unknown`，构建时经
   `-ldflags "-X .../utils.CurrentVersion=<tag> -X .../utils.VersionHash=<sha>"` 注入；
   运行期暴露于日志、`/api/version`。

## 4. 编译结果（本地实测，2026-09-16）

| 项目 | 结果 |
| --- | --- |
| 工具链 | Go **1.25.14**（满足 `go 1.25.0`）、zig **0.14.1**、zstd **1.5.7**、Node **v22.23.2** / npm 10.9.8 |
| 前端 | `komari-web @ tag 1.5.0` → `npm install`（808 包）+ `npm run build` **成功**；`dist/` 546 个文件 |
| 主题打包 | `dist.tar.zst`（9.9 MB tar → **2,115,333 B**，`zstd -19`），校验含 `index.html`；`komari-theme.json` 已复制 |
| 后端编译 | `CGO_ENABLED=1 CC="zig cc -target x86_64-windows-gnu" go build -trimpath -ldflags="-s -w -X ...CurrentVersion=1.5.0-fix1 -X ...VersionHash=0ca87aa…"` **成功**，耗时 **1m38s** |
| 产物 | `komari-windows-amd64.exe`，**35,257,344 B**；sha256 `df09b3b5…4d65ab7` |
| `go vet ./...` | **无输出（通过）** |
| 运行冒烟 | 启动成功：初始化 `./data/komari.db`（SQLite/WAL）+ `./data/{metrics.db,plugin,plugin-data,theme}`；首启进入安装向导（`/` → 307 `/install`）；`POST /api/install/complete` 建管理员成功；随后 `/api/version` 返回 `{"version":"1.5.0-fix1","hash":"0ca87aa…"}`；`GET /` 返回嵌入的仪表盘 HTML；`/api/public` 正常；`POST /api/login` 成功发放 `session_token` |
| `Docker` | **未在本地验证**（本机无 Docker）；镜像构建由 CI 承担，第一阶段后需在 CI 上验证 amd64/arm64 镜像 |

> 结论：**源码 → 编译 → 测试 → 打包 → 运行** 全链路可脱离原作者完成。
> （本地环境细节与证据文件清单见维护者私有笔记，不纳入本文件。）

## 5. 测试与静态分析结果（本地实测）

- `go test ./... -count=1`：**全部通过，exit 0**。示例耗时：`pkg/metric` 25.3s、
  `pkg/jsruntime` 3.4s、`web/public` 3.4s、`internal/plugin` 3.8s、`web/security` 3.0s、
  `utils/geoip` 9.5s；其余均在 3s 内。
- 外部数据库集成测试（PostgreSQL / MySQL / MariaDB）在未设置
  `METRIC_POSTGRES_DSN` / `METRIC_MYSQL_DSN` / `METRIC_MARIADB_DSN` 时**自动 skip**——
  即当前测试套件默认只覆盖 SQLite 路径，**非 SQLite 指标库路径在 CI 中无覆盖**（改进项）。
- 前端 `npm run lint`：**0 error / 29 warning**（仅 react-hooks 依赖警告一类）。
- 静态分析：无 `golangci-lint` 配置、CI 不跑 `go vet`；本次为审计新增 `go vet`（通过）与
  `govulncheck`（见 §6）。
- TODO/FIXME 标记：全仓库仅 3 处（非阻塞）。

## 6. 第三方依赖与漏洞审计

### 6.1 govulncheck（本地实测）

| 级别 | 数量 | 说明 |
| --- | --- | --- |
| **代码可达（需要修）** | **2** | `GO-2026-5970` `golang.org/x/text@v0.33.0` → 修复于 `v0.39.0`（无效输入死循环；经 goja/gorm/sqlite/net 路径可达）；`GO-2026-5026` `golang.org/x/net@v0.41.0` → 修复于 `v0.55.0`（idna Punycode 校验绕过） |
| 导入包级（当前不可达） | 9 | 主要为 `golang.org/x/net`、`golang.org/x/sys` 多个 idna/http2 问题 |
| 模块级（当前不可达） | 22 | `golang.org/x/crypto@v0.39.0`（多条，最高需 `v0.56.0`）、`golang.org/x/net`（最高需 `v0.56.0`）、`golang.org/x/sys`（需 `v0.44.0`）、`github.com/klauspost/compress@v1.17.11`（需 `v1.18.7`）；其中 `GO-2026-5932` 上游标注 **Fixed in: N/A**（尚无修复版本，需跟踪） |

> 注意：`go.mod` 中对 `x/crypto`、`x/net` 的版本有**人工上提注释**（历史上为修 CVE 而锁版本），
> 说明维护者对这两条线有既定策略；升级时须保持注释与实际版本一致。

### 6.2 依赖新鲜度

- `go list -m -u all`：**49 个模块可升级**，与安全相关的关键项：
  `golang.org/x/crypto v0.39.0→v0.57.0`、`golang.org/x/net v0.41.0→v0.59.0`、
  `golang.org/x/text v0.33.0→v0.36+`（`x/text` 更新线）、`gin v1.10.0→v1.12.0`、
  `go-sqlite3 v1.14.27→v1.14.52`、`goja`（伪版本）、`klauspost/compress v1.17.11→v1.20.0`、
  `pgx v5.10.0→v5.11.0`、`cobra v1.9.1→v1.10.2`、`testify v1.11.1→v1.12.1` 等。
- 已弃用：`github.com/golang/protobuf v1.5.0`（间接依赖，随上游迁移消失）。
- 前端依赖：`npm install` 引入 808 个包；`package.json` 未声明 `engines`，CI 固定 Node 23，
  本机 Node 22 构建同样通过（建议 Fork 明确 Node 版本区间）。

## 7. CI/CD 现状盘点（`.github/workflows`）

| Workflow | 触发 | 作用 | Fork 处置建议 |
| --- | --- | --- | --- |
| `build.yml` | PR → main | 前端 + 8 平台二进制矩阵构建（**无测试**） | 保留并**补测试** |
| `release.yml` | release published | 构建二进制并上传 release 资产 + 推 Docker 镜像（tag = release tag） | 保留（改为 stable 流程） |
| `release-docker.yml` | 手动（输入 version） | 手动构建并推 Docker 镜像 | 保留 |
| `docker-publish.yml` | 手动（main 推送已注释） | main 全平台 Docker | 可并入 release 流程 |
| `snapshot.yml` | 手动 / push main | 发布 `Snapshot-yymmddhhMM`（UTC+8）预发布 | 可保留（或关闭，稳定分支不需要快照） |
| `rebuild-release.yml` | 手动（release_tag/server_ref/frontend_ref） | 重打指定 release | 保留（用于可复现重发） |
| `generate-release-notes.yml` | release published | 用 AI（`OPENAI_API_KEY`）生成发版说明 | **关闭**，改用人工 `CHANGELOG.md` |
| `auto-merge-dev-to-main.yml` | PR（dev→main） | 自动合并 | **移除**（Fork 无 dev/main 双轨） |
| `development.yml` | push dev | 构建并 **SSH 部署到作者生产服务器** | **移除**（含 `PRODUCTION_SSH_KEY` 依赖） |
| `cleanup-packages.yml` | 手动 | 清理 ghcr 旧包 | 可选保留 |

**统一缺口（新 CI 必须补齐）：**

1. **没有任何 `go test`** —— 上游 487 个测试函数从未在 CI 中执行；Fork 应新增 `test` job（含 `-race` 可选）。
2. 无 `go vet` / `golangci-lint` / `staticcheck`。
3. 无 `govulncheck` / Dependabot / 依赖审计。
4. `setup-go` 固定 `"1.23"`，与 `go.mod` 的 `go 1.25.0` 不一致（依赖 GOTOOLCHAIN 自动下载，
   构建结果不受版本控制影响但不可复现）；Fork 应显式固定 `go-version: 1.25.x`。
5. 前端 `ref` 不可复现：`build-frontend` 默认克隆 `komari-web` **默认分支**（现为 `radix`，会漂移）；
   只有 `X.Y.Z-fixN` 形式的 tag 才会把 `frontend_ref` 解析为 `X.Y.Z`。→ 稳定分支必须**显式固定前端 ref**。
6. 无产物校验（checksums / SBOM / 构建来源证明）。
7. 无并发/权限最小化审计；`release.yml` 与 `snapshot.yml` 权限为 `contents: write, packages: write`。

## 8. 已知问题清单

**上游 open issues（6 个，归档后不会有人修）**

| # | 类型 | 摘要 | 备注 |
| --- | --- | --- | --- |
| #673 | bug | v1.5.0 私有站点：从服务列表终端入口访问返回 404；后台页面退出再访问 404（cookie 失效） | 影响 1.5.0 日常使用 |
| #670 | bug | 默认主题详情页切到 10 分钟/1 小时/1 天，报 `RPC Error -32602: unknown metric key: memory.total` | 主题/指标键映射问题 |
| #667 | bug | 历史流量统计异常（后台"最近 24 小时流量"与日报出现 TB/PB 量级；实时累计正常） | **数据正确性**，优先级高 |
| #666 | bug | 部分月付机器提前续费后仍显示"已过期" | 续费/到期计算 |
| #660 | feature | 负载警报增加"连接数"警报 | 非稳定分支目标 |
| #655 | bug | Windows 下 Agent 安装命令不兼容含空格用户名 | 影响新节点接入 |

**安全公告（GitHub Security Advisories）**

| GHSA | 严重度 | 主题 | 影响范围 | 修复版本 |
| --- | --- | --- | --- | --- |
| GHSA-hxjg-93wc-h8p8 | High (CVSS 8.8) | 管理接口 CSRF（`session_token` 未设 SameSite/Secure；管理端无 CSRF token） | ≤ 1.2.0 | 1.2.2 |
| GHSA-q355-h244-969h | High | 跨站 WebSocket 劫持（终端 WS 未校 Origin） | < 1.0.4 | 1.0.4-fix1 |
| GHSA-jhmr-57cj-q6g9 | High | 2FA 校验逻辑错误（任意 6 位码可通过） | < 1.0.4 | 1.0.4-fix1 |

当前基线（1.5.0-fix1）代码核对：登录 Cookie 已设 `HttpOnly + SameSite=Lax`（`web/api/public/login.go:26-33`）；
CORS Origin 校验与 WebSocket Origin 校验默认开启（`internal/config/settings.go`）；
**SSRF 防护默认关闭**（`ssrf_protection_enabled=false`，涉及主题/插件市场与远程导入）。
→ 上述历史公告均已修复，但 **CSRF 类问题只依赖 SameSite=Lax 兜底，无 CSRF token**，属于长期技术债（记录，不在稳定分支主动重构）。

## 9. 风险地图

### 9.1 数据库风险（最高优先级）

- **主库**：SQLite 单文件 `./data/komari.db`（WAL 模式，`flags.DatabaseFile` 可改路径，
  `-d/--database` 参数）。**仅支持 SQLite**（`cmd/flags/config.go`：`SupportedDatabaseTypes()` 只返回 `sqlite`）。
- **监控库**：独立配置 `metric_db_driver` / `metric_db_dsn`，默认 `./data/metrics.db`（SQLite），
  可选 MySQL / PostgreSQL（`pkg/metric` + `internal/metricstore`），并带**独立迁移与重构逻辑**
  （`pkg/metric/migrations.go`、`restructure.go`、`digest_reclaim.go`）与管理端迁移向导（`web/migration`）。
- **升级保护**：`database/dbcore/dbcore.go` 的 `backupOnVersionUpgrade()` —— 进程启动时若检测到
  配置中的 `system_version` 与当前版本不同，会先把整个 `./data` 打包成
  `./data/backup/upgrade-<UTC时间戳>.zip`（先 WAL checkpoint），再写入新版本标记。
  这是**升级与回滚的主要安全网**，修改 DB 相关代码时必须保证其行为不变。
- **Schema 演进方式**：GORM `AutoMigrate`（只加不删）+ `internal/migrations`（一次性历史迁移，
  含 0.x/1.0.x/1.1.x 兼容路径、时间戳 UTC 迁移、configs→config_items 迁移等）。
  `1.5.0-fix1` 已停止为 `TrafficReportNotification` 建表（不删表）。
- **备份/恢复**：管理端备份（`web/backup`，`/api/admin/download/backup`）、安装向导支持上传备份
  （`/api/install/upload/*` → 重启恢复）、指标库"恢复向导"（`web/recovery`，DSN 脱敏处理）。
- **风险结论**：任何触碰 `AutoMigrate` 列表、`internal/migrations`、`pkg/metric/migrations` 的改动
  都必须经过"旧库升级 → 数据完整性核对 → 旧版本回滚"三步验证；不可逆改动必须**先停下来报告**。

### 9.2 Agent / API 兼容风险

- 1.5.0 起**仅支持 v2 协议**（v1 接口已移除：`/api/clients/report` 等）。
  v2：`GET/POST /api/clients/v2/rpc`（WebSocket 优先，POST 回退 + `agent.pull` 长轮询），
  方法集 `agent.report / basicInfo / pingResult / taskResult / exec / ping / message / event / pull / file…`
  （`protocol/v2/jsonrpc.go`）。
- 自动发现注册：`POST /api/clients/register`（`Authorization: Bearer <AutoDiscoveryKey>`，密钥 <12 位直接拒绝）。
- 终端流量走独立 WebSocket：`/api/clients/terminal?id=<session>`；文件走 `/api/clients/transfer/:id`。
- **服务端版本与 Agent 版本解耦**：Agent 独立发版（最新 1.5.10），并**自带自动更新**（从
  `komari-agent` 的 GitHub Release 拉取）。→ Fork 无法也不应干预 Agent 的升级通道；
  稳定分支的最低义务是：**v2 线协议与端点保持向后兼容**，不因服务端改动让旧 Agent 掉线。
- **外部依赖点**：前端生成的 Agent 安装命令直接引用
  `https://raw.githubusercontent.com/komari-monitor/komari-agent/refs/heads/main/install.sh|install.ps1`
  ——这是一个**会漂移的第三方引用**；若上游 Agent 仓库未来归档，安装流程将失效（Fork 需评估镜像/自托管，见 §10）。
- 上游另有 `komari-protocol` 仓库对 v1/v2 做"冻结 + 冻结测试"，可作为协议兼容的对照基准。

### 9.3 其它高风险模块

| 模块 | 位置 | 风险点 |
| --- | --- | --- |
| Web 终端 | `web/api/terminal/*`、`web/rpc/jsonrpc/admin.xtermjs.go` | WS + 2FA 流程，历史上出过 CSWSH；会话重连逻辑（1.5.0 新增断线重附着） |
| 文件管理 / 传输 | `web/filemanager/*`、`web/upload/*` | 分块上传、流式中继、路径/大小校验；备份上传在**安装模式**下无需登录（受 `requireActive` 门控） |
| 插件系统 | `internal/plugin/*`、`pkg/jsruntime/*`（goja JS 引擎） | 插件在宿主内运行 JS，有权限清单（allowRoutes/allowHooks/allowHTMLInject/allowSystemRPC）；市场下载受 SSRF 开关约束（默认关） |
| 认证与授权 | `web/api/Auth.go`、`principal.go`、`database/accounts/*` | Role Admin/Client/Guest、API Key（Bearer）、OAuth、2FA、会话管理 |
| 指标存储引擎 | `pkg/metric/*`（约 30k 行） | 最复杂模块：滚动聚合、摘要回收、方言适配、迁移与重构、远端写入超时 |
| 安全基础设施 | `web/security/*`（CORS/Origin）、SSRF 策略 | 默认值与实际部署常常不一致，需文档化 |
| 安装/恢复向导 | `web/install/*`、`web/recovery/*` | 首启无认证窗口期（仅本地/首启生效，`requireActive` 门控），恢复向导会输出 DSN（已脱敏） |

## 10. 推荐维护策略

### 10.1 分支与版本

- `upstream-baseline`：**永不修改**的官方基线（= `0ca87aa` / tag `1.5.0-fix1` 的镜像），仅用于
  对比与重放。
- `stable`：长期维护分支，所有修复提交在这里，PR 必须引用 issue 或明确的问题描述。
- 版本号：`1.5.0-stable.1`、`1.5.0-stable.2` …… 只在有真实功能变化时才考虑升 `1.6.0`。

### 10.2 允许 / 禁止进入 stable 的改动

**允许**：崩溃/数据正确性修复；安全修复；兼容性修复（保持 v2 协议、数据库 schema、配置格式、
Docker 部署方式不变）；构建/CI 修正；文档；补测试。

**禁止（需单独决策或永不）**：重构、重命名、目录调整、框架迁移、ORM/数据库替换、API 重设计、
UI 重写、依赖大版本升级、新增大型功能、删除现有功能、改变用户可见行为（除非是明确的 Bug）。

### 10.3 依赖升级策略（针对 §6 的落地方案）

- 只做**安全驱动**的升级，且**一次一个模块族**（例如先 `x/text`，再 `x/net`，再 `x/crypto`），
  每个升级单独 PR：
  `go get golang.org/x/text@v0.39.0` → 全量测试 → `govulncheck` 复核 → 发布 `1.5.0-stable.N`。
- `x/crypto` / `x/net` 的版本在 `go.mod` 里有历史注释，升级时**同步更新注释**（说明原因与 CVE 编号）。
- 不做 `gin` / `gorm` / `goja` 等大版本跳跃（除非有 CVE 且无法回避）。

### 10.4 CI 目标形态（第一阶段后的第一件工程任务）

1. `test`：`go test ./... -count=1`（Linux，Go 1.25.x 固定），PR 与 main 必跑。
2. `lint`：`go vet ./...` + `staticcheck`（或 `golangci-lint`，先只启用保守规则集）。
3. `vuln`：`govulncheck ./...`（失败阈值：出现"代码可达"漏洞时告警，不自动阻断修复 PR）。
4. `build`：Linux amd64/arm64 + Windows amd64（zig 交叉编译，版本固定 zig 0.14.1、前端 ref 固定）。
5. `docker`：release 时构建并推送 `ghcr.io/<fork-owner>/komari-stable:stable` 与
   `:<version>`；**不可变 tag 永不覆盖**。
6. （可选后期）制品校验与 SBOM。

### 10.5 发布 / 回滚规程

- 发布：`CHANGELOG.md` 必须包含 Fixed / Security / Compatibility（DB-Agent-Docker-API-Config）/
  Upgrade / Rollback 五段；release 资产与 Docker tag 对应同一 commit。
- 升级：官方二进制替换即可；服务端会在版本变化时自动备份 `./data`（§9.1）。
- 回滚：换回旧二进制 + 用 `./data/backup/upgrade-*.zip` 恢复数据目录；Docker 场景回退到旧 tag +
  保留 `data` 卷。
- Docker tag 策略：`stable` 为浮动 tag；`1.5.0-stable.N` 为不可变 tag（禁止覆盖，禁止 force push tag）。

### 10.6 上游与生态跟踪

- 跟踪上游 fork 网络与社区分支（例如 `nuomiiiii/komari` Lite 版、各主题仓库）中出现的修复，
  **择优移植**而不是 rebase 上游。
- 关注 `komari-agent` / `komari-web` / `komari-protocol` 三个仍活跃仓库：若上游对 v2 协议或前端
  API 做不兼容变更，Fork 需要**冻结**自己的前端 ref 并评估是否跟进。
- 仓库公开前执行：密钥/隐私扫描（脚本 + 历史）、README/文档脱敏、LICENSE/NOTICE/版权保留核查。

## 11. 待决问题（需要维护者拍板）

1. **仓库与命名**：GitHub 仓库名（建议 `komari-stable`）、是否公开、Docker 命名空间
   （建议 `ghcr.io/<owner>/komari-stable`）。
2. **前端策略**：固定 ref（建议固定到 `komari-web` tag `1.5.0` = `dec6495`）还是允许跟进；
   是否在 Fork 内镜像前端源码以便完全自持。
3. **`install-komari.sh` 与 Agent 安装脚本**：上游引用（`komari-monitor/*`、第三方 Lite 仓库）
   是否改为指向 Fork，或原样保留并加说明。
4. **`generate-release-notes.yml`（AI 发版说明）** 与 `development.yml`（SSH 部署作者生产环境）
   的移除方式（建议直接删除，仅保留在 git 历史里）。
5. **是否启用 `snapshot` 通道**（稳定分支建议关闭）。

---

### 附：本报告的证据来源

- 上游仓库只读克隆（`main` @ `0ca87aa`）源码与 git 历史；`.github/workflows` 全量 11 个 workflow。
- GitHub REST API：仓库元数据、releases、tags、issues（#655/#660/#666/#667/#670/#673）、
  security advisories（3 条）、组织下 12 个仓库状态。
- 官方文档：`komari-document`（install/binary、install/compile、install/update、dev/compatibility、
  dev/agent、dev/api、dev/rpc）与 komari.wiki 首页。
- 本地实测：前端构建日志、后端编译日志、`go vet`/`go test` 全量输出、`govulncheck`（含 verbose）、
  运行冒烟记录（安装向导 → 指标库初始化 → 仪表盘 → 登录）。
